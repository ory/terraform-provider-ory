//go:build acceptance

package projectconfig_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccProjectConfigResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "cors_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "password_min_length", "10"),
				),
			},
			// ImportState - after import, Read only refreshes fields that are
			// non-null in state. Since import only sets id/project_id, config
			// fields won't be populated until the user runs terraform apply.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"cors_enabled", "cors_origins", "password_min_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_hydraConfig(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/hydra_config.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_access_token_lifespan", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_refresh_token_lifespan", "720h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_auth_code_lifespan", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_id_token_lifespan", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_login_consent_request_lifespan", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_access_token_strategy", "jwt"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_jwt_scope_claim", "list"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_scope_strategy", "wildcard"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_pkce_enforced", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_pkce_enforced_for_public_clients", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_session_encrypt_at_rest", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_allowed_top_level_claims.#", "2"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_mirror_top_level_claims", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_login_url", "https://example.com/login"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_consent_url", "https://example.com/consent"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_logout_url", "https://example.com/logout"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_error_url", "https://example.com/error"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_oauth2Cookies(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with Strict mode
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_cookies.tf.tmpl", map[string]string{
					"SameSiteMode":     "Strict",
					"LegacyWorkaround": "false",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_cookies_same_site_mode", "Strict"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_cookies_same_site_legacy_workaround", "false"),
				),
			},
			// Update to Lax with legacy workaround
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_cookies.tf.tmpl", map[string]string{
					"SameSiteMode":     "Lax",
					"LegacyWorkaround": "true",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_cookies_same_site_mode", "Lax"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_cookies_same_site_legacy_workaround", "true"),
				),
			},
			// Verify no perpetual diff
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_cookies.tf.tmpl", map[string]string{
					"SameSiteMode":     "Lax",
					"LegacyWorkaround": "true",
				}),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProjectConfigResource_oauth2IssuerURL(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_issuer.tf.tmpl", map[string]string{
					"IssuerURL": "https://auth.example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_issuer_url", "https://auth.example.com"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"oauth2_issuer_url",
					"cors_enabled", "password_min_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_mfaPolicy(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/mfa.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_totp", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "totp_issuer", "TerraformTest"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_codeMFA(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with code MFA enabled
			{
				Config: acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", map[string]string{"CodeMFAEnabled": "true"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_code", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "code_mfa_enabled", "true"),
				),
			},
			// ImportState — config fields are ignored because import only sets
			// id/project_id; Read only refreshes fields that are non-null in
			// state, so config attributes won't be populated until apply.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"enable_code", "code_mfa_enabled",
					"cors_enabled", "password_min_length",
					"smtp_connection_uri",
				},
			},
			// Update to disabled
			{
				Config: acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", map[string]string{"CodeMFAEnabled": "false"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_code", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "code_mfa_enabled", "false"),
				),
			},
			// Verify no perpetual diff
			{
				Config:             acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", map[string]string{"CodeMFAEnabled": "false"}),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccProjectConfigResource_oidc(t *testing.T) {
	acctest.RequireSocialProviderTests(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_oidc", "true"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_oidcAutoLinkPolicy(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc_auto_link_policy.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_oidc", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_oidc_auto_link_policy", "true"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_oidcBaseRedirectURI(t *testing.T) {
	acctest.RequireSocialProviderTests(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc_base_redirect_uri.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_oidc", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oidc_base_redirect_uri", "https://iam.example.com"),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc_base_redirect_uri_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "oidc_base_redirect_uri", "https://auth.example.com"),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc_base_redirect_uri_removed.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "oidc_base_redirect_uri", ""),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_accountExperience(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/account_experience.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_name", "TF Test App"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_default_locale", "en"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_adminCORS(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/admin_cors.tf.tmpl", map[string]string{"AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "cors_admin_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "cors_admin_origins.#", "1"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_tokenizerTemplates(t *testing.T) {
	templateData := map[string]string{"TTL": "1h"}

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: Create templates
			{
				Config: acctest.LoadTestConfig(t, "testdata/tokenizer_templates.tf.tmpl", templateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.my_jwt.ttl", "1h"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.ttl", "5m"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.subject_source", "external_id"),
				),
			},
			// Step 2: Verify no perpetual diff after create
			{
				Config:   acctest.LoadTestConfig(t, "testdata/tokenizer_templates.tf.tmpl", templateData),
				PlanOnly: true,
			},
			// Step 3: Update TTL
			{
				Config: acctest.LoadTestConfig(t, "testdata/tokenizer_templates.tf.tmpl", map[string]string{"TTL": "2h"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.my_jwt.ttl", "2h"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.ttl", "5m"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.subject_source", "external_id"),
				),
			},
			// Step 4: Verify no perpetual diff after update
			{
				Config:   acctest.LoadTestConfig(t, "testdata/tokenizer_templates.tf.tmpl", map[string]string{"TTL": "2h"}),
				PlanOnly: true,
			},
			// Step 5: Remove templates out-of-band via API, then re-apply
			// to verify drift detection correctly triggers re-creation.
			{
				PreConfig: clearTokenizerTemplatesOutOfBand(t),
				Config:    acctest.LoadTestConfig(t, "testdata/tokenizer_templates.tf.tmpl", map[string]string{"TTL": "2h"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.my_jwt.ttl", "2h"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.ttl", "5m"),
				),
			},
			// Step 6: ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"session_tokenizer_templates",
					"smtp_connection_uri",
					"cors_enabled",
					"password_min_length",
				},
			},
		},
	})
}

// clearTokenizerTemplatesOutOfBand returns a PreConfig function that removes
// tokenizer templates directly via the API to simulate external drift.
func clearTokenizerTemplatesOutOfBand(t *testing.T) func() {
	t.Helper()
	return func() {
		c, err := acctest.GetOryClient()
		if err != nil {
			t.Fatalf("Failed to create Ory client: %v", err)
		}
		projectID := acctest.GetTestProjectID(t)
		patches := []ory.JsonPatch{
			{
				Op:    "replace",
				Path:  "/services/identity/config/session/whoami/tokenizer/templates",
				Value: map[string]interface{}{},
			},
		}
		if _, err := c.PatchProject(context.Background(), projectID, patches); err != nil {
			t.Fatalf("Failed to clear tokenizer templates out-of-band: %v", err)
		}
		t.Log("Cleared tokenizer templates out-of-band to simulate drift")
	}
}

func TestAccProjectConfigResource_emptyReturnURLs(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: Set return URLs to empty values (clears any server defaults)
			{
				Config: acctest.LoadTestConfig(t, "testdata/empty_return_urls.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "default_return_url", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "allowed_return_urls.#", "0"),
				),
			},
			// Step 2: Re-apply same config to verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/empty_return_urls.tf.tmpl", nil),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProjectConfigResource_returnURLsWithServerDefaults(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: Set specific return URLs (API will append server defaults)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_return_urls.tf.tmpl", map[string]string{"AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "default_return_url", testutil.ExampleAppURL),
					resource.TestCheckResourceAttr("ory_project_config.test", "allowed_return_urls.#", "1"),
					resource.TestCheckResourceAttr("ory_project_config.test", "allowed_return_urls.0", testutil.ExampleAppURL),
				),
			},
			// Step 2: Verify no perpetual diff despite API appending server defaults
			{
				Config:   acctest.LoadTestConfig(t, "testdata/with_return_urls.tf.tmpl", map[string]string{"AppURL": testutil.ExampleAppURL}),
				PlanOnly: true,
			},
			// Step 3: Clear to empty values
			{
				Config: acctest.LoadTestConfig(t, "testdata/empty_return_urls.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "default_return_url", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "allowed_return_urls.#", "0"),
				),
			},
			// Step 4: Verify no perpetual diff after clearing
			{
				Config:   acctest.LoadTestConfig(t, "testdata/empty_return_urls.tf.tmpl", nil),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProjectConfigResource_loginStyle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with identifier_first
			{
				Config: acctest.LoadTestConfig(t, "testdata/login_style.tf.tmpl", map[string]string{"LoginStyle": "identifier_first"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "login_style", "identifier_first"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"login_style", "enable_password",
					"cors_enabled", "password_min_length",
					"smtp_connection_uri",
				},
			},
			// Update to unified
			{
				Config: acctest.LoadTestConfig(t, "testdata/login_style.tf.tmpl", map[string]string{"LoginStyle": "unified"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "login_style", "unified"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/login_style.tf.tmpl", map[string]string{"LoginStyle": "unified"}),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProjectConfigResource_settingsAndVerification(t *testing.T) {
	createData := map[string]string{
		"EnableProfile":                        "true",
		"CodeLifespan":                         "15m0s",
		"CodeMissingCredentialFallbackEnabled": "true",
		"SettingsLifespan":                     "30m0s",
		"SettingsPrivilegedSessionMaxAge":      "15m0s",
		"RequiredAAL":                          "aal1",
		"VerificationUse":                      "code",
		"VerificationLifespan":                 "30m0s",
		"VerificationNotifyUnknownRecipients":  "false",
	}
	updateData := map[string]string{
		"EnableProfile":                        "true",
		"CodeLifespan":                         "20m0s",
		"CodeMissingCredentialFallbackEnabled": "false",
		"SettingsLifespan":                     "1h0m0s",
		"SettingsPrivilegedSessionMaxAge":      "30m0s",
		"RequiredAAL":                          "highest_available",
		"VerificationUse":                      "link",
		"VerificationLifespan":                 "1h0m0s",
		"VerificationNotifyUnknownRecipients":  "true",
	}

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create
			{
				Config: acctest.LoadTestConfig(t, "testdata/settings_verification.tf.tmpl", createData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					// Profile method
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_profile", "true"),
					// Code config
					resource.TestCheckResourceAttr("ory_project_config.test", "code_lifespan", "15m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "code_missing_credential_fallback_enabled", "true"),
					// Settings flow
					resource.TestCheckResourceAttr("ory_project_config.test", "settings_lifespan", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "settings_privileged_session_max_age", "15m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "required_aal", "aal1"),
					// Verification flow
					resource.TestCheckResourceAttr("ory_project_config.test", "verification_use", "code"),
					resource.TestCheckResourceAttr("ory_project_config.test", "verification_lifespan", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "verification_notify_unknown_recipients", "false"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"enable_profile",
					"enable_code", "code_lifespan", "code_missing_credential_fallback_enabled",
					"settings_lifespan", "settings_privileged_session_max_age", "required_aal",
					"enable_verification", "verification_use", "verification_lifespan", "verification_notify_unknown_recipients",
					"cors_enabled", "password_min_length", "smtp_connection_uri",
				},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/settings_verification.tf.tmpl", updateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Code config updated
					resource.TestCheckResourceAttr("ory_project_config.test", "code_lifespan", "20m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "code_missing_credential_fallback_enabled", "false"),
					// Settings flow updated
					resource.TestCheckResourceAttr("ory_project_config.test", "settings_lifespan", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "settings_privileged_session_max_age", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "required_aal", "highest_available"),
					// Verification flow updated
					resource.TestCheckResourceAttr("ory_project_config.test", "verification_use", "link"),
					resource.TestCheckResourceAttr("ory_project_config.test", "verification_lifespan", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "verification_notify_unknown_recipients", "true"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/settings_verification.tf.tmpl", updateData),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProjectConfigResource_courierHTTP(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/courier_http.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_delivery_strategy", "http"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.url", "https://mail-api.example.com/send"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.method", "POST"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.auth.type", "basic_auth"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.auth.user", "mailuser"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.#", "1"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.0.id", "sms"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.0.request_config.url", "https://sms-api.example.com/send"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.0.request_config.auth.type", "api_key"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.0.request_config.auth.name", "Authorization"),
				),
			},
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"courier_delivery_strategy",
					"courier_http_request_config",
					"courier_channels",
					"smtp_connection_uri",
					"cors_enabled",
					"password_min_length",
				},
			},
		},
	})
}
