//go:build acceptance

package samlprovider_test

import (
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

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

// buildTestMetadataXML generates a minimal SAML 2.0 IDP metadata XML document
// containing a fresh self-signed signing certificate, encoded as base64://.
// Ory validates the metadata structure on PATCH, so a complete KeyDescriptor
// is required even for tests.
func buildTestMetadataXML(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test signing key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idp.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create test certificate: %v", err)
	}
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
