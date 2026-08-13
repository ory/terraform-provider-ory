//go:build acceptance

package socialprovider_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

// TestAccSocialProviderResource_concurrentCreate verifies that multiple social
// providers created in a single apply (no depends_on) all persist correctly.
// This is the regression test for https://github.com/ory/terraform-provider-ory/issues/165
// where concurrent read-modify-write operations caused last-write-wins data loss.
func TestAccSocialProviderResource_concurrentCreate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create 4 providers concurrently (no depends_on)
			{
				Config: acctest.LoadTestConfig(t, "testdata/concurrent_create.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.concurrent_google", "provider_id", "test-concurrent-google"),
					resource.TestCheckResourceAttr("ory_social_provider.concurrent_github", "provider_id", "test-concurrent-github"),
					resource.TestCheckResourceAttr("ory_social_provider.concurrent_discord", "provider_id", "test-concurrent-discord"),
					resource.TestCheckResourceAttr("ory_social_provider.concurrent_slack", "provider_id", "test-concurrent-slack"),
				),
			},
			// Verify no diff — all 4 providers should already exist
			{
				Config:   acctest.LoadTestConfig(t, "testdata/concurrent_create.tf.tmpl", nil),
				PlanOnly: true,
			},
		},
	})
}

// TestAccSocialProviderResource_writeOnlySecret verifies that client_secret_wo
// is accepted by the API but never written to Terraform state, and that bumping
// client_secret_wo_version triggers an update (rotation). Write-only arguments
// require Terraform 1.11+.
func TestAccSocialProviderResource_writeOnlySecret(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with a write-only client secret (version 1).
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_secret.tf.tmpl", map[string]string{
					"Version": "1",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.wo", "provider_id", "test-wo-google"),
					resource.TestCheckResourceAttr("ory_social_provider.wo", "provider_type", "google"),
					resource.TestCheckResourceAttr("ory_social_provider.wo", "client_id", "test-wo-client-id.apps.googleusercontent.com"),
					resource.TestCheckResourceAttr("ory_social_provider.wo", "client_secret_wo_version", "1"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					// The write-only secret and the unused stateful client_secret
					// must both be absent from state.
					statecheck.ExpectKnownValue("ory_social_provider.wo", tfjsonpath.New("client_secret_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_social_provider.wo", tfjsonpath.New("client_secret"), knownvalue.Null()),
				},
			},
			// Rotate the secret by bumping the version trigger (version 2).
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_secret.tf.tmpl", map[string]string{
					"Version": "2",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.wo", "client_secret_wo_version", "2"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_social_provider.wo", tfjsonpath.New("client_secret_wo"), knownvalue.Null()),
				},
			},
			// Re-applying the same config must produce no diff (idempotency).
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_secret.tf.tmpl", map[string]string{
					"Version": "2",
				}),
				PlanOnly: true,
			},
			// Import: client_id round-trips; write-only attributes do not.
			{
				ResourceName:            "ory_social_provider.wo",
				ImportState:             true,
				ImportStateId:           "test-wo-google",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret", "client_secret_wo", "client_secret_wo_version"},
			},
		},
	})
}

// TestAccSocialProviderResource_writeOnlyClientID verifies that when client_id is
// supplied via the write-only client_id_wo, the client_id never appears in state
// and the configuration remains free of perpetual diffs.
func TestAccSocialProviderResource_writeOnlyClientID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_client_id.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.woid", "provider_id", "test-wo-id-google"),
					resource.TestCheckResourceAttr("ory_social_provider.woid", "client_id_wo_version", "1"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					// Neither the stateful client_id nor any write-only value is stored.
					statecheck.ExpectKnownValue("ory_social_provider.woid", tfjsonpath.New("client_id"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_social_provider.woid", tfjsonpath.New("client_id_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_social_provider.woid", tfjsonpath.New("client_secret_wo"), knownvalue.Null()),
				},
			},
			// No perpetual diff: client_id stays out of state across refreshes.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/write_only_client_id.tf.tmpl", nil),
				PlanOnly: true,
			},
		},
	})
}

func TestAccSocialProviderResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_type", "google"),
				),
			},
			// ImportState using provider_id
			{
				ResourceName:      "ory_social_provider.test",
				ImportState:       true,
				ImportStateId:     "test-google",
				ImportStateVerify: true,
				// client_secret is sensitive and not returned by API
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// generateTestPrivateKey creates a fresh EC P-256 private key in PEM format
// at test runtime so no key material is committed to source code.
func generateTestPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "failed to generate test private key")
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err, "failed to marshal test private key")
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestAccSocialProviderResource_baseRedirectURI(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with base_redirect_uri
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_base_redirect_uri.tf.tmpl", map[string]string{
					"BaseRedirectURI": "https://iam.example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-redir"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "base_redirect_uri", "https://iam.example.com"),
				),
			},
			// Update base_redirect_uri
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_base_redirect_uri_updated.tf.tmpl", map[string]string{
					"UpdatedBaseRedirectURI": "https://auth.example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "base_redirect_uri", "https://auth.example.com"),
				),
			},
			// Remove base_redirect_uri (unset in config)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_base_redirect_uri_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "base_redirect_uri"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-redir",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_preservesProjectConfigBaseRedirectURI verifies
// that creating and deleting a social provider does not wipe the global OIDC
// base_redirect_uri managed by ory_project_config. Both the first-provider
// create and the last-provider delete used to replace the whole OIDC method
// node, discarding sibling config keys.
func TestAccSocialProviderResource_preservesProjectConfigBaseRedirectURI(t *testing.T) {
	const baseRedirectURI = "https://iam.example.com"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Set base_redirect_uri via ory_project_config, then add a provider
			{
				Config: acctest.LoadTestConfig(t, "testdata/preserve_project_config_base_redirect_uri.tf.tmpl", map[string]string{
					"BaseRedirectURI": baseRedirectURI,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-preserve"),
					checkServerBaseRedirectURI(baseRedirectURI),
				),
			},
			// Remove the provider — base_redirect_uri must survive server-side
			{
				Config: acctest.LoadTestConfig(t, "testdata/preserve_project_config_base_redirect_uri_removed.tf.tmpl", map[string]string{
					"BaseRedirectURI": baseRedirectURI,
				}),
				Check: checkServerBaseRedirectURI(baseRedirectURI),
			},
		},
	})
}

// checkServerBaseRedirectURI asserts the OIDC base_redirect_uri stored on the
// project by reading the API directly, bypassing Terraform state.
func checkServerBaseRedirectURI(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c, err := acctest.GetOryClient()
		if err != nil {
			return fmt.Errorf("could not create client: %w", err)
		}

		var projectID string
		for _, rs := range s.RootModule().Resources {
			if pid := rs.Primary.Attributes["project_id"]; pid != "" {
				projectID = pid
				break
			}
		}
		if projectID == "" {
			return fmt.Errorf("no project_id found in state")
		}

		p, err := c.GetProject(context.Background(), projectID)
		if err != nil {
			return fmt.Errorf("could not get project: %w", err)
		}

		var current interface{} = p.Services.Identity.Config
		for _, seg := range []string{"selfservice", "methods", "oidc", "config"} {
			m, ok := current.(map[string]interface{})
			if !ok {
				return fmt.Errorf("OIDC config missing at %q", seg)
			}
			current = m[seg]
		}
		oidcConfig, ok := current.(map[string]interface{})
		if !ok {
			return fmt.Errorf("OIDC config is not an object")
		}
		got, _ := oidcConfig["base_redirect_uri"].(string)
		if got != want {
			return fmt.Errorf("base_redirect_uri = %q, want %q", got, want)
		}
		return nil
	}
}

func TestAccSocialProviderResource_autoLink(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with auto_link enabled
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_auto_link.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-autolink"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "auto_link", "true"),
				),
			},
			// Verify no perpetual diff (auto_link is write-only, state must be preserved)
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_auto_link.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Remove auto_link from config while it is true — should send false to
			// API and clear state (validates "removal disables" behavior)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_auto_link_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "auto_link"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_auto_link_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Re-enable then explicitly set to false
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_auto_link.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "auto_link", "true"),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_auto_link_disabled.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "auto_link", "false"),
				),
			},
			// ImportState — auto_link is write-only (not returned by API)
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-autolink",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret", "auto_link"},
			},
		},
	})
}

func TestAccSocialProviderResource_labelAndAccountLinkingMode(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with label and account_linking_mode
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_label_and_linking.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-label-linking"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "label", "Sign in with Google"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "account_linking_mode", "automatic"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_label_and_linking.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update both fields
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_label_and_linking_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "label", "Google SSO"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "account_linking_mode", "confirm_with_existing_credential"),
				),
			},
			// Remove both fields
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_label_and_linking_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "label"),
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "account_linking_mode"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_label_and_linking_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-label-linking",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_additionalIDTokenAudiences verifies that the
// additional_id_token_audiences attribute is set, read, updated, and removed
// correctly. Regression test for https://github.com/ory/terraform-provider-ory/issues/211.
func TestAccSocialProviderResource_additionalIDTokenAudiences(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with two audiences
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_audiences.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-audiences"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "additional_id_token_audiences.#", "2"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "additional_id_token_audiences.0", "https://other.example.com"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "additional_id_token_audiences.1", "another-audience"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_audiences.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update audiences (shrink to one entry, change value)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_audiences_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "additional_id_token_audiences.#", "1"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "additional_id_token_audiences.0", "https://updated.example.com"),
				),
			},
			// Remove audiences from config — should clear them server-side
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_audiences_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "additional_id_token_audiences"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_audiences_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-audiences",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_pkce verifies that the pkce attribute is set,
// read, updated, and removed correctly. PKCE accepts "auto", "force", or
// "never"; this test exercises the round-trip for "force" → "auto" → "never"
// → unset, including the explicit "auto" value (which the API also uses as
// its default when the attribute is absent).
func TestAccSocialProviderResource_pkce(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with pkce = "force"
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_pkce.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-pkce"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "pkce", "force"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_pkce.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update pkce to "auto" — explicit default; catches drift if the API
			// normalizes "auto" to absence.
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_pkce_auto.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "pkce", "auto"),
				),
			},
			// Verify no perpetual diff on "auto"
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_pkce_auto.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update pkce to "never"
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_pkce_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "pkce", "never"),
				),
			},
			// Remove pkce from config — should clear it server-side
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_pkce_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "pkce"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_pkce_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-pkce",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_fedcmConfigURL exercises fedcm_config_url across
// create, update (changed URL), removal, and import. The value is an opaque URL
// stored on the provider config; the API stores and returns it verbatim, so the
// test asserts on string equality and on clearing when the attribute is removed.
func TestAccSocialProviderResource_fedcmConfigURL(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with fedcm_config_url set
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_fedcm.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-fedcm"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "fedcm_config_url", "https://accounts.google.com/gsi/fedcm.json"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_fedcm.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update fedcm_config_url to a different URL
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_fedcm_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "fedcm_config_url", "https://accounts.example.com/fedcm/config.json"),
				),
			},
			// Verify no perpetual diff after update
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_fedcm_updated.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Remove fedcm_config_url from config — should clear it server-side
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_fedcm_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "fedcm_config_url"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_fedcm_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-fedcm",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_netIDTokenOriginHeader exercises
// net_id_token_origin_header across create, update (changed origin), removal,
// and import. Create and Update replace the whole provider object, so an
// attribute missing from the schema is blanked server-side on every apply. The
// PlanOnly step after each apply is the regression guard for that: it fails if
// the API drops the value.
// Regression test for https://github.com/ory/terraform-provider-ory/issues/329.
func TestAccSocialProviderResource_netIDTokenOriginHeader(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with net_id_token_origin_header set
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_net_id_origin_header.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-netid-origin"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_type", "netid"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "fedcm_config_url", "https://broker.netid.de/fedcm.json"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "net_id_token_origin_header", "https://www.example.com"),
				),
			},
			// Verify no perpetual diff — the API must echo the value back unchanged
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_net_id_origin_header.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update the origin header
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_net_id_origin_header_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "net_id_token_origin_header", "https://news.example.com"),
				),
			},
			// Verify no perpetual diff after update
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_net_id_origin_header_updated.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Remove net_id_token_origin_header from config — should clear it server-side
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_net_id_origin_header_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "net_id_token_origin_header"),
					// The sibling FedCM attribute must survive the removal.
					resource.TestCheckResourceAttr("ory_social_provider.test", "fedcm_config_url", "https://broker.netid.de/fedcm.json"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_net_id_origin_header_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-netid-origin",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_aal2Values exercises aal2_acr_values and
// aal2_amr_values across create, update (changed list contents), removal, and
// import. Values are opaque strings stored on the provider config — the API
// does not dereference them — so the test asserts only on string equality
// using synthetic URN-style identifiers that have no external dependency.
func TestAccSocialProviderResource_aal2Values(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with aal2_acr_values and aal2_amr_values set
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_aal2.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-aal2"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_acr_values.#", "2"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_acr_values.0", "urn:test:acr:silver"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_acr_values.1", "urn:test:acr:mfa"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.#", "3"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.0", "mfa"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.1", "otp"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.2", "hwk"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_aal2.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update both lists
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_aal2_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_acr_values.#", "1"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_acr_values.0", "urn:test:acr:gold"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.#", "2"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.0", "mfa"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "aal2_amr_values.1", "fpt"),
				),
			},
			// Remove both attributes from config — API should clear them and state should follow
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_aal2_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "aal2_acr_values"),
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "aal2_amr_values"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_aal2_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-aal2",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_updateIdentityOnLogin exercises the
// update_identity_on_login attribute across create ("automatic"), update to the
// explicit default ("never"), removal, and import. The Ory API accepts only the
// enum values "never" and "automatic"; when the attribute is omitted the API
// omits it on read, which the provider collapses to null.
// Regression test for https://github.com/ory/terraform-provider-ory/issues/278.
func TestAccSocialProviderResource_updateIdentityOnLogin(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with update_identity_on_login = "automatic"
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_update_identity_on_login.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-uiol"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "update_identity_on_login", "automatic"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_update_identity_on_login.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Update to "never" — the explicit default; catches drift if the API
			// normalizes "never" to absence.
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_update_identity_on_login_never.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test", "update_identity_on_login", "never"),
				),
			},
			// Verify no perpetual diff on "never"
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_update_identity_on_login_never.tf.tmpl", nil),
				PlanOnly: true,
			},
			// Remove update_identity_on_login from config — should clear it server-side
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_update_identity_on_login_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ory_social_provider.test", "update_identity_on_login"),
				),
			},
			// Verify no diff after removal
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_update_identity_on_login_removed.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-uiol",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

// TestAccSocialProviderResource_mapperURLNoDrift verifies that a configured
// base64:// mapper_url does not produce a perpetual diff even though Ory rewrites
// it into an opaque GCS URL server-side. The provider preserves the configured
// value in state instead of reading back the transformed value.
// Regression test for https://github.com/ory/terraform-provider-ory/issues/278.
func TestAccSocialProviderResource_mapperURLNoDrift(t *testing.T) {
	const mapperURL = "base64://bG9jYWwgY2xhaW1zID0gc3RkLmV4dFZhcignY2xhaW1zJyk7CnsKICBpZGVudGl0eTogewogICAgdHJhaXRzOiB7CiAgICAgIGVtYWlsOiBjbGFpbXMuZW1haWwsCiAgICB9LAogIH0sCn0="

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with a base64 mapper_url
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_mapper_url.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google-mapper"),
					// State must hold the configured value, not the API's GCS rewrite.
					resource.TestCheckResourceAttr("ory_social_provider.test", "mapper_url", mapperURL),
				),
			},
			// The critical assertion: re-planning the same config must show no diff,
			// even though the API stores a transformed mapper_url.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_mapper_url.tf.tmpl", nil),
				PlanOnly: true,
			},
			// A second refresh-and-plan cycle confirms the value stays stable.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_mapper_url.tf.tmpl", nil),
				PlanOnly: true,
			},
			// ImportState — mapper_url is not read back from the API (it cannot be
			// reversed from the GCS URL), so it is not populated on import.
			{
				ResourceName:            "ory_social_provider.test",
				ImportState:             true,
				ImportStateId:           "test-google-mapper",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret", "mapper_url"},
			},
		},
	})
}

func TestAccSocialProviderResource_apple(t *testing.T) {
	tmplData := struct{ PrivateKey string }{PrivateKey: generateTestPrivateKey(t)}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create Apple provider with Apple-specific fields
			{
				Config: acctest.LoadTestConfig(t, "testdata/apple_basic.tf.tmpl", tmplData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test_apple", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "provider_id", "test-apple"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "provider_type", "apple"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "client_id", "com.example.auth.service"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_team_id", "TESTTEAMID"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_private_key_id", "TESTKEYID1"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_social_provider.test_apple",
				ImportState:       true,
				ImportStateId:     "test-apple",
				ImportStateVerify: true,
				// Sensitive fields not returned by API
				ImportStateVerifyIgnore: []string{"client_secret", "apple_private_key"},
			},
			// Update Apple provider fields
			{
				Config: acctest.LoadTestConfig(t, "testdata/apple_updated.tf.tmpl", tmplData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "client_id", "com.example.auth.service.updated"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_team_id", "UPDATEDTID"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_private_key_id", "UPDATEDKID"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "scope.#", "3"),
				),
			},
		},
	})
}
