//go:build acceptance

package socialprovider_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
