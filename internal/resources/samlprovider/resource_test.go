//go:build acceptance

package samlprovider_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

// buildTestMetadataXML generates a minimal SAML 2.0 IDP metadata XML document
// containing a fresh self-signed signing certificate, encoded as base64://.
// Ory validates the metadata structure on PATCH, so a complete KeyDescriptor
// is required even for tests.
func buildTestMetadataXML(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate test signing key")
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idp.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err, "failed to create test certificate")
	certB64 := base64.StdEncoding.EncodeToString(certDER)

	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" WantAuthnRequestsSigned="false">
    <KeyDescriptor use="signing">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data>
          <X509Certificate>%s</X509Certificate>
        </X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</NameIDFormat>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://idp.example.com/sso"/>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, certB64)

	// base64-encode the entire XML so the API accepts it as a `base64://` value.
	return "base64://" + base64.StdEncoding.EncodeToString([]byte(metadata))
}

func TestAccSAMLProviderResource_basic(t *testing.T) {
	metadata := buildTestMetadataXML(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"MetadataXML": metadata,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_saml_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_saml_provider.test", "provider_id", "test-saml"),
					resource.TestCheckResourceAttr("ory_saml_provider.test", "label", "Test SAML"),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", map[string]string{
					"MetadataXML": metadata,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_saml_provider.test", "label", "Test SAML Updated"),
					resource.TestCheckResourceAttr("ory_saml_provider.test", "audience_override_base_url", "https://audience.example.com"),
					resource.TestCheckResourceAttr("ory_saml_provider.test", "proxy_saml_audience_override", "https://audience-override.example.com"),
				),
			},
			{
				ResourceName:      "ory_saml_provider.test",
				ImportState:       true,
				ImportStateId:     "test-saml",
				ImportStateVerify: true,
				// raw_idp_metadata_xml is transformed to a GCS URL on read. When the user
				// configured a base64:// value, we preserve it in state on refresh, but
				// import starts from a blank state — the API's transformed URL is what
				// import sees, and that's a legitimate value (also accepted by PATCH).
				ImportStateVerifyIgnore: []string{"raw_idp_metadata_xml"},
			},
		},
	})
}

// readSAMLMethodNode returns the selfservice.methods.saml node from the test
// project, read straight from the API rather than from Terraform state.
func readSAMLMethodNode(t *testing.T) map[string]interface{} {
	t.Helper()

	c, err := acctest.GetOryClient()
	require.NoError(t, err, "could not create client")

	project, err := c.GetProject(context.Background(), acctest.GetTestProject(t).ID)
	require.NoError(t, err, "could not get project")
	require.NotNil(t, project.Services.Identity, "project has no identity service")

	selfservice, _ := project.Services.Identity.Config["selfservice"].(map[string]interface{})
	methods, _ := selfservice["methods"].(map[string]interface{})
	node, _ := methods["saml"].(map[string]interface{})
	return node
}

// samlProviderIDs returns the provider ids stored in the SAML method node.
func samlProviderIDs(node map[string]interface{}) []string {
	config, _ := node["config"].(map[string]interface{})
	providers, _ := config["providers"].([]interface{})
	ids := make([]string, 0, len(providers))
	for _, p := range providers {
		pm, _ := p.(map[string]interface{})
		if id, ok := pm["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// TestAccSAMLProviderResource_deleteKeepsSiblingProviders is the regression test
// for issue #332. Both the first-provider create and the last-provider delete
// used to write the whole selfservice.methods.saml node, which replaces the
// object and discards every key not named in the literal (RFC 6902). The same
// defect on the OIDC node discarded config.base_redirect_uri, see
// TestAccSocialProviderResource_preservesProjectConfigBaseRedirectURI.
//
// Every assertion reads the API directly. Terraform state cannot see a node
// overwrite, because state only records what the provider believes it wrote.
func TestAccSAMLProviderResource_deleteKeepsSiblingProviders(t *testing.T) {
	metadata := buildTestMetadataXML(t)
	configData := map[string]string{"MetadataXML": metadata}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		// After the last provider goes, the method must be left disabled with an
		// empty providers array, and the node must still carry both keys.
		CheckDestroy: func(*terraform.State) error {
			node := readSAMLMethodNode(t)
			if node == nil {
				return fmt.Errorf("the saml method node was removed entirely, it should be left disabled")
			}
			if _, ok := node["enabled"]; !ok {
				return fmt.Errorf("the saml method node lost its enabled key: %v", node)
			}
			if enabled, _ := node["enabled"].(bool); enabled {
				return fmt.Errorf("the saml method is still enabled after the last provider was deleted")
			}
			if _, ok := node["config"]; !ok {
				return fmt.Errorf("the saml method node lost its config key: %v", node)
			}
			if ids := samlProviderIDs(node); len(ids) != 0 {
				return fmt.Errorf("providers remain after destroy: %v", ids)
			}
			return nil
		},
		Steps: []resource.TestStep{
			// Two providers. The first create enables the method.
			{
				Config: acctest.LoadTestConfig(t, "testdata/two_providers.tf.tmpl", configData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_saml_provider.keep", "provider_id", "test-saml-keep"),
					resource.TestCheckResourceAttr("ory_saml_provider.remove", "provider_id", "test-saml-remove"),
					func(*terraform.State) error {
						node := readSAMLMethodNode(t)
						if enabled, _ := node["enabled"].(bool); !enabled {
							return fmt.Errorf("adding the first provider did not enable the saml method: %v", node)
						}
						ids := samlProviderIDs(node)
						if len(ids) != 2 {
							return fmt.Errorf("expected 2 providers server-side, got %v", ids)
						}
						return nil
					},
				),
			},
			// Remove one of the two. The survivor must still be there.
			{
				Config: acctest.LoadTestConfig(t, "testdata/one_provider.tf.tmpl", configData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_saml_provider.keep", "provider_id", "test-saml-keep"),
					func(*terraform.State) error {
						ids := samlProviderIDs(readSAMLMethodNode(t))
						if len(ids) != 1 || ids[0] != "test-saml-keep" {
							return fmt.Errorf("expected only test-saml-keep server-side, got %v", ids)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccSAMLProviderResource_concurrentCreate verifies that multiple SAML
// providers created in a single apply (no depends_on) all persist correctly.
// This is the same read-modify-write race guarded against in social_provider
// (see https://github.com/ory/terraform-provider-ory/issues/165).
func TestAccSAMLProviderResource_concurrentCreate(t *testing.T) {
	metadata := buildTestMetadataXML(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/concurrent_create.tf.tmpl", map[string]string{
					"MetadataXML": metadata,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_saml_provider.concurrent_one", "provider_id", "test-concurrent-saml-one"),
					resource.TestCheckResourceAttr("ory_saml_provider.concurrent_two", "provider_id", "test-concurrent-saml-two"),
					resource.TestCheckResourceAttr("ory_saml_provider.concurrent_three", "provider_id", "test-concurrent-saml-three"),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/concurrent_create.tf.tmpl", map[string]string{
					"MetadataXML": metadata,
				}),
				PlanOnly: true,
			},
		},
	})
}
