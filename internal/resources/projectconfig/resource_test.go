//go:build acceptance

package projectconfig_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// TestAccProjectConfigResource_smtpConnectionURIWriteOnlyArgument verifies the
// native write-only argument smtp_connection_uri_wo: the value is sent to the API
// but never written to Terraform state, and bumping smtp_connection_uri_wo_version
// rotates it. This is distinct from smtp_connection_uri, which is not read back
// but is still stored in state. Write-only arguments require Terraform 1.11+.
func TestAccProjectConfigResource_smtpConnectionURIWriteOnlyArgument(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { acctest.AccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/smtp_connection_uri_wo.tf.tmpl", map[string]string{
					"SMTPURI": "smtp://tf-acc-wo:not-a-real-secret@smtp.example.com:587",
					"Version": "1",
				}),
				Check: resource.TestCheckResourceAttr("ory_project_config.test", "smtp_connection_uri_wo_version", "1"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_project_config.test", tfjsonpath.New("smtp_connection_uri_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_project_config.test", tfjsonpath.New("smtp_connection_uri"), knownvalue.Null()),
				},
			},
			// Rotate the secret via the version trigger.
			{
				Config: acctest.LoadTestConfig(t, "testdata/smtp_connection_uri_wo.tf.tmpl", map[string]string{
					"SMTPURI": "smtps://tf-acc-wo:also-not-real@smtp.example.com:465",
					"Version": "2",
				}),
				Check: resource.TestCheckResourceAttr("ory_project_config.test", "smtp_connection_uri_wo_version", "2"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_project_config.test", tfjsonpath.New("smtp_connection_uri_wo"), knownvalue.Null()),
				},
			},
			// Idempotency.
			{
				Config: acctest.LoadTestConfig(t, "testdata/smtp_connection_uri_wo.tf.tmpl", map[string]string{
					"SMTPURI": "smtps://tf-acc-wo:also-not-real@smtp.example.com:465",
					"Version": "2",
				}),
				PlanOnly: true,
			},
		},
	})
}

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
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_password_config_min_password_length", "10"),
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
					"cors_enabled", "cors_origins", "selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

// TestAccProjectConfigResource_smtpConnectionURIWriteOnly verifies that
// smtp_connection_uri can be created and updated, and that it never produces a
// perpetual diff even though the Ory API does not return it in project-config
// responses (it is a write-only secret). The terraform-plugin-testing framework
// runs a plan after each apply and fails the step on a non-empty plan, so each
// step also asserts idempotency — the read path must not clobber the configured
// value with an empty or masked value returned by the API.
func TestAccProjectConfigResource_smtpConnectionURIWriteOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with the connection URI set (STARTTLS on 587).
			{
				Config: acctest.LoadTestConfig(t, "testdata/smtp_write_only.tf.tmpl", map[string]string{
					"SMTPURI": "smtp://tf-acc-test:not-a-real-secret@smtp.example.com:587",
				}),
				Check: resource.TestCheckResourceAttr("ory_project_config.test", "smtp_connection_uri",
					"smtp://tf-acc-test:not-a-real-secret@smtp.example.com:587"),
			},
			// Update the secret (implicit TLS on 465). A successful no-diff plan after
			// this apply confirms the update was applied and not re-read as a diff.
			{
				Config: acctest.LoadTestConfig(t, "testdata/smtp_write_only.tf.tmpl", map[string]string{
					"SMTPURI": "smtps://tf-acc-test:also-not-real@smtp.example.com:465",
				}),
				Check: resource.TestCheckResourceAttr("ory_project_config.test", "smtp_connection_uri",
					"smtps://tf-acc-test:also-not-real@smtp.example.com:465"),
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
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_ttl_access_token", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_ttl_refresh_token", "720h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_ttl_auth_code", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_ttl_id_token", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_ttl_login_consent_request", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_strategies_access_token", "jwt"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_strategies_jwt_scope_claim", "list"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_strategies_scope", "wildcard"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_pkce_enforced", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_pkce_enforced_for_public_clients", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_allowed_top_level_claims.#", "2"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_mirror_top_level_claims", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_urls_login", "https://example.com/login"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_urls_consent", "https://example.com/consent"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_urls_logout", "https://example.com/logout"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_urls_error", "https://example.com/error"),
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
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_serve_cookies_same_site_mode", "Strict"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_serve_cookies_same_site_legacy_workaround", "false"),
				),
			},
			// Update to Lax with legacy workaround
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_cookies.tf.tmpl", map[string]string{
					"SameSiteMode":     "Lax",
					"LegacyWorkaround": "true",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_serve_cookies_same_site_mode", "Lax"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_serve_cookies_same_site_legacy_workaround", "true"),
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
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_urls_self_issuer", "https://auth.example.com"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"oauth2_urls_self_issuer",
					"cors_enabled", "selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_sessionEarliestPossibleExtend(t *testing.T) {
	createData := map[string]string{
		"Lifespan":               "720h0m0s",
		"EarliestPossibleExtend": "24h",
	}
	updateData := map[string]string{
		"Lifespan":               "720h0m0s",
		"EarliestPossibleExtend": "1h",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create
			{
				Config: acctest.LoadTestConfig(t, "testdata/session_extend.tf.tmpl", createData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_lifespan", "720h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_earliest_possible_extend", "24h"),
				),
			},
			// ImportState — import only sets id/project_id; Read only refreshes
			// fields that are non-null in state, so config attributes won't be
			// populated until apply.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"session_lifespan", "session_earliest_possible_extend",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/session_extend.tf.tmpl", updateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "session_earliest_possible_extend", "1h"),
				),
			},
			// Verify no perpetual diff
			{
				Config:   acctest.LoadTestConfig(t, "testdata/session_extend.tf.tmpl", updateData),
				PlanOnly: true,
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
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_totp_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_totp_config_issuer", "TerraformTest"),
				),
			},
		},
	})
}

func TestAccProjectConfigResource_codeMFA(t *testing.T) {
	// max_submissions is here to cover the corrected patch path. The spec's
	// governs description points at selfservice.methods.code.max_submissions,
	// which the API accepts and discards; the value only sticks under
	// selfservice.methods.code.config.max_submissions. The PlanOnly steps are
	// what prove it, since a discarded value would refresh to null and show a
	// diff forever.
	createData := map[string]string{"CodeMFAEnabled": "true", "MaxSubmissions": "5"}
	updateData := map[string]string{"CodeMFAEnabled": "false", "MaxSubmissions": "3"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with code MFA enabled
			{
				Config: acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", createData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_mfa_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_config_max_submissions", "5"),
				),
			},
			// Verify no perpetual diff on create
			{
				Config:             acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", createData),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			// ImportState — config fields are ignored because import only sets
			// id/project_id; Read only refreshes fields that are non-null in
			// state, so config attributes won't be populated until apply.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_methods_code_enabled", "selfservice_methods_code_mfa_enabled",
					"selfservice_methods_code_config_max_submissions",
					"cors_enabled", "selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
			// Update to disabled
			{
				Config: acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", updateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_mfa_enabled", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_config_max_submissions", "3"),
				),
			},
			// Verify no perpetual diff
			{
				Config:             acctest.LoadTestConfig(t, "testdata/code_mfa.tf.tmpl", updateData),
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
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_oidc_enabled", "true"),
				),
			},
		},
	})
}

// TestAccProjectConfigResource_oidcAutoLinkPolicy covers create, import, update
// and idempotency for the OIDC auto-link policy toggle.
//
// Note: the auto-link policy is a plan-gated feature. Setting it to true on a
// project whose subscription plan does not include the "Auto Link Policy"
// feature is rejected by the Ory API with a 403 feature_not_available. The
// provider surfaces that feature name, reason, and request ID via wrapAPIError
// (see internal/client) instead of a bare "403 Forbidden". This test requires a
// project on a plan that includes the feature.
func TestAccProjectConfigResource_oidcAutoLinkPolicy(t *testing.T) {
	// Enabling the auto-link policy requires the enterprise use_auto_link
	// entitlement; without it the API returns 403 feature_not_available, so this
	// test only runs when ORY_AUTO_LINK_TESTS_ENABLED=true against an entitled
	// project.
	acctest.RequireAutoLinkTests(t)

	enabled := map[string]string{"EnableAutoLink": "true"}
	disabled := map[string]string{"EnableAutoLink": "false"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with the auto-link policy enabled.
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc_auto_link_policy.tf.tmpl", enabled),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_oidc_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_oidc_enable_auto_link_policy", "true"),
				),
			},
			// ImportState — import only sets id/project_id, so configured
			// attributes (and computed defaults like cors_enabled) are not
			// repopulated until apply and must be ignored here.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_methods_oidc_enabled",
					"selfservice_methods_oidc_enable_auto_link_policy",
					"cors_enabled",
				},
			},
			// Update — toggle the auto-link policy back off.
			{
				Config: acctest.LoadTestConfig(t, "testdata/oidc_auto_link_policy.tf.tmpl", disabled),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_oidc_enable_auto_link_policy", "false"),
				),
			},
			// Verify no perpetual diff.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/oidc_auto_link_policy.tf.tmpl", disabled),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProjectConfigResource_accountExperience(t *testing.T) {
	// Two distinct 1x1 PNGs (red and blue pixels) so the update step changes
	// the image content hash.
	logoRed := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/iZk9HQAAAABJRU5ErkJggg=="
	logoBlue := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNgYPj/HwADAgH/5ncLrgAAAABJRU5ErkJggg=="

	createData := map[string]string{
		"LogoLight":    logoRed,
		"LogoDark":     logoBlue,
		"FaviconLight": logoRed,
		"FaviconDark":  logoBlue,
		"BrandColor":   "#0066ff",
	}
	updateData := map[string]string{
		"LogoLight":    logoBlue,
		"LogoDark":     logoRed,
		"FaviconLight": logoRed,
		"FaviconDark":  logoBlue,
		"BrandColor":   "#22cc88",
	}

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: Create with branding configured. Regression for issue
			// #250: the logo previously failed to apply (silently ignored
			// config key) and theme variables returned HTTP 500 (string sent
			// where the API expects a map of color tokens).
			{
				Config: acctest.LoadTestConfig(t, "testdata/account_experience.tf.tmpl", createData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_default_locale", "en"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_logo_light", logoRed),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_logo_dark", logoBlue),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_favicon_light", logoRed),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_favicon_dark", logoBlue),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_theme_variables_light.ax_background_default", "#fafafa"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_theme_variables_light.brand_500", "#0066ff"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_theme_variables_dark.ax_background_default", "#0a0a0a"),
				),
			},
			// Step 2: No perpetual diff — the API stores the image at a
			// content-addressed storage URL; the provider must match it
			// against the configured data URI by content hash.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/account_experience.tf.tmpl", createData),
				PlanOnly: true,
			},
			// Step 3: Update the logo image and a theme color.
			{
				Config: acctest.LoadTestConfig(t, "testdata/account_experience.tf.tmpl", updateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_logo_light", logoBlue),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_logo_dark", logoRed),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_favicon_light", logoRed),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_favicon_dark", logoBlue),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_theme_variables_light.brand_500", "#22cc88"),
				),
			},
			// Step 4: No perpetual diff after the update.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/account_experience.tf.tmpl", updateData),
				PlanOnly: true,
			},
			// Step 5: ImportState — import only sets id/project_id, so config
			// fields stay null until the next apply and are ignored.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"account_experience_default_locale",
					"account_experience_logo_light",
					"account_experience_logo_dark",
					"account_experience_favicon_light",
					"account_experience_favicon_dark",
					"account_experience_theme_variables_light",
					"account_experience_theme_variables_dark",
					"cors_enabled",
					"smtp_connection_uri",
				},
			},
			// Step 6: Clear all branding again (also restores the shared
			// test project to its pre-test state).
			{
				Config: acctest.LoadTestConfig(t, "testdata/account_experience_cleared.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_logo_light", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_logo_dark", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_favicon_light", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_favicon_dark", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_theme_variables_light.%", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_theme_variables_dark.%", "0"),
				),
			},
			// Step 7: Cleared values round-trip without drift.
			{
				Config:   acctest.LoadTestConfig(t, "testdata/account_experience_cleared.tf.tmpl", nil),
				PlanOnly: true,
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
					"selfservice_methods_password_config_min_password_length",
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
		require.NoError(t, err, "Failed to create Ory client")
		projectID := acctest.GetTestProjectID(t)
		patches := []ory.JsonPatch{
			{
				Op:    "replace",
				Path:  "/services/identity/config/session/whoami/tokenizer/templates",
				Value: map[string]interface{}{},
			},
		}
		_, err = c.PatchProject(context.Background(), projectID, patches)
		require.NoError(t, err, "Failed to clear tokenizer templates out-of-band")
		t.Log("Cleared tokenizer templates out-of-band to simulate drift")
	}
}

// Note on drift detection for values removed outside Terraform.
//
// There is deliberately no acceptance test that clears a project_config value
// out-of-band and asserts the next refresh notices. It cannot pass. The client
// caches the PatchProject response with no expiry, Read prefers that cache over
// a live GetProject, and Provider.Configure reuses the same client for the whole
// provider server lifecycle to work around API eventual consistency. Because the
// test framework builds the provider once per test case, every refresh after the
// first apply sees the cached pre-drift project, so a PreConfig hook that
// patches through a second client is invisible.
//
// Real terraform runs are unaffected: each CLI command is a fresh process with
// an empty cache. Coverage lives in read_simple_fields_test.go, which drives
// readSimpleFields against API responses that omit the key.

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
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_login_style", "identifier_first"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_flows_login_style", "selfservice_methods_password_enabled",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
			// Update to unified
			{
				Config: acctest.LoadTestConfig(t, "testdata/login_style.tf.tmpl", map[string]string{"LoginStyle": "unified"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_login_style", "unified"),
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
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_profile_enabled", "true"),
					// Code config
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_config_lifespan", "15m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_config_missing_credential_fallback_enabled", "true"),
					// Settings flow
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_lifespan", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_privileged_session_max_age", "15m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_required_aal", "aal1"),
					// Verification flow
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_verification_use", "code"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_verification_lifespan", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_verification_notify_unknown_recipients", "false"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_methods_profile_enabled",
					"selfservice_methods_code_enabled", "selfservice_methods_code_config_lifespan", "selfservice_methods_code_config_missing_credential_fallback_enabled",
					"selfservice_flows_settings_lifespan", "selfservice_flows_settings_privileged_session_max_age", "selfservice_flows_settings_required_aal",
					"selfservice_flows_verification_enabled", "selfservice_flows_verification_use", "selfservice_flows_verification_lifespan", "selfservice_flows_verification_notify_unknown_recipients",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/settings_verification.tf.tmpl", updateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Code config updated
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_config_lifespan", "20m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_code_config_missing_credential_fallback_enabled", "false"),
					// Settings flow updated
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_lifespan", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_privileged_session_max_age", "30m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_required_aal", "highest_available"),
					// Verification flow updated
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_verification_use", "link"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_verification_lifespan", "1h0m0s"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_verification_notify_unknown_recipients", "true"),
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

func TestAccProjectConfigResource_oauth2TokenHookAuth(t *testing.T) {
	// Always wipe `oauth2.token_hook` after the test, even on intermediate
	// step failure. Background: the API normalizes a `replace token_hook =
	// "<url>"` patch into `{"url": "<url>"}`, but its own schema's `oneOf`
	// rejects that shape on the *next* PATCH (must be either a string URL
	// or a full `{url, auth}` object). Without this cleanup, a failed run
	// can leave the shared CI project in a state that breaks every
	// subsequent PATCH from every PR and trips the EU-W3 5xx alarm.
	t.Cleanup(func() { clearProjectTokenHook(t) })

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_token_hook_auth.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook", "https://example.com/token-hook"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.type", "api_key"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.name", "X-Api-Key"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.value", "test-token-hook-api-key"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.in", "header"),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_token_hook_auth_updated.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook", "https://example.com/token-hook-v2"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.name", "ory-token-hook-auth"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.value", "updated-cookie-value"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook_auth.in", "cookie"),
				),
			},
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"oauth2_token_hook",
					"oauth2_token_hook_auth",
					"smtp_connection_uri",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
				},
			},
			// Drop auth but keep URL: the provider must collapse the URL
			// patch with a full `token_hook` replace so the API doesn't reject
			// the remove (the schema requires either a URL string or a
			// `{url, auth}` object — never `{url}` alone).
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_token_hook_no_auth.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_hook", "https://example.com/token-hook-no-auth"),
					resource.TestCheckNoResourceAttr("ory_project_config.test", "oauth2_token_hook_auth"),
				),
			},
		},
	})
}

// clearProjectTokenHook removes the `oauth2.token_hook` field from the shared
// test project. The "no-auth" tail of the test leaves the API in a
// schema-invalid `{"url": ...}` shape that the API itself rejects on the next
// PATCH; we explicitly tear it down so the shared project doesn't break other
// tests. Errors are downgraded to log lines because a missing field is the
// expected post-cleanup state.
func clearProjectTokenHook(t *testing.T) {
	t.Helper()

	c, err := acctest.GetOryClient()
	if err != nil {
		t.Logf("Warning: could not create client to clear token_hook: %v", err)
		return
	}

	projectID := acctest.GetTestProjectID(t)
	patches := []ory.JsonPatch{
		{
			Op:   "remove",
			Path: "/services/oauth2/config/oauth2/token_hook",
		},
	}
	if _, err := c.PatchProject(context.Background(), projectID, patches); err != nil {
		t.Logf("Warning: failed to clear oauth2.token_hook (may already be absent): %v", err)
		return
	}
	t.Log("Cleared oauth2.token_hook on test project")
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
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.url", "https://example.com/mail-api/send"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.method", "POST"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.auth.type", "basic_auth"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config.auth.user", "mailuser"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.#", "1"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.0.id", "sms"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_channels.0.request_config.url", "https://example.com/sms-api/send"),
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
					"selfservice_methods_password_config_min_password_length",
				},
			},
		},
	})
}

// courierBodyTemplate and courierBodyTemplateUpdated are `base64://` Jsonnet
// payloads for the flat courier_http_request_config_body attribute. The Ory API
// stores the decoded payload and reports a storage URL instead, so each step's
// implicit post-apply plan check is what proves the read path resolves that URL
// back to the configured value. See issue #315.
const (
	courierBodyTemplate        = "base64://ewogICJyZWNpcGllbnQiOiB7eyAucmVjaXBpZW50IH19LAogICJib2R5Ijoge3sgLmJvZHkgfX0KfQ=="
	courierBodyTemplateUpdated = "base64://ewogICJyZWNpcGllbnQiOiB7eyAucmVjaXBpZW50IH19LAogICJzdWJqZWN0Ijoge3sgLnN1YmplY3QgfX0sCiAgImJvZHkiOiB7eyAuYm9keSB9fQp9"
)

// TestAccProjectConfigResource_courierHTTPBody covers the flat courier body
// attribute end to end: create, refresh, update, and import. Before the storage
// URL fix, the refresh after step 1 wrote
// `https://storage.googleapis.com/.../<sha512>.jsonnet` into state and the step
// failed with a non-empty plan.
func TestAccProjectConfigResource_courierHTTPBody(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/courier_http_body.tf.tmpl", map[string]string{
					"Body": courierBodyTemplate,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_delivery_strategy", "http"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config_url", "https://example.com/mail-api/send"),
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config_body", courierBodyTemplate),
				),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/courier_http_body.tf.tmpl", map[string]string{
					"Body": courierBodyTemplateUpdated,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "courier_http_request_config_body", courierBodyTemplateUpdated),
				),
			},
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Import starts from empty state, so every optional attribute
				// comes back null. That is the resource's existing import
				// behavior and is not what this test covers.
				ImportStateVerifyIgnore: []string{
					"courier_delivery_strategy",
					"courier_http_request_config_url",
					"courier_http_request_config_body",
					"smtp_connection_uri",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_featureFlags(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/feature_flags.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "feature_flags_cacheable_sessions", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "feature_flags_use_continue_with_transitions", "true"),
				),
			},
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"feature_flags_cacheable_sessions",
					"feature_flags_cacheable_sessions_max_age",
					"feature_flags_use_continue_with_transitions",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_recoveryFlow(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/recovery_flow.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_recovery_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_recovery_use", "code"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_recovery_notify_unknown_recipients", "true"),
				),
			},
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_flows_recovery_enabled",
					"selfservice_flows_recovery_use",
					"selfservice_flows_recovery_lifespan",
					"selfservice_flows_recovery_notify_unknown_recipients",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_sessionHookOnPasswordRegistration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Enable the session hook on the password registration flow.
			{
				Config: acctest.LoadTestConfig(t, "testdata/session_hook.tf.tmpl", map[string]string{
					"Session": "true",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_registration_after_password_hook_session", "true"),
				),
			},
			// Disable it.
			{
				Config: acctest.LoadTestConfig(t, "testdata/session_hook.tf.tmpl", map[string]string{
					"Session": "false",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_registration_after_password_hook_session", "false"),
				),
			},
			// Re-apply the same config to confirm no perpetual diff.
			{
				Config: acctest.LoadTestConfig(t, "testdata/session_hook.tf.tmpl", map[string]string{
					"Session": "false",
				}),
				PlanOnly: true,
			},
			// Import then verify state.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_flows_registration_after_password_hook_session",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_showVerificationUIHooks(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Enable all three show_verification_ui hooks.
			{
				Config: acctest.LoadTestConfig(t, "testdata/show_verification_ui_hooks.tf.tmpl", map[string]string{
					"Password": "true",
					"OIDC":     "true",
					"Profile":  "true",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_registration_after_password_hook_show_verification_ui", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_registration_after_oidc_hook_show_verification_ui", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_show_verification_ui", "true"),
				),
			},
			// Disable the password and profile hooks, keep oidc enabled.
			{
				Config: acctest.LoadTestConfig(t, "testdata/show_verification_ui_hooks.tf.tmpl", map[string]string{
					"Password": "false",
					"OIDC":     "true",
					"Profile":  "false",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_registration_after_password_hook_show_verification_ui", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_registration_after_oidc_hook_show_verification_ui", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_show_verification_ui", "false"),
				),
			},
			// Re-apply the same config to confirm no perpetual diff.
			{
				Config: acctest.LoadTestConfig(t, "testdata/show_verification_ui_hooks.tf.tmpl", map[string]string{
					"Password": "false",
					"OIDC":     "true",
					"Profile":  "false",
				}),
				PlanOnly: true,
			},
			// Import then verify state.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_flows_registration_after_password_hook_show_verification_ui",
					"selfservice_flows_registration_after_oidc_hook_show_verification_ui",
					"selfservice_flows_settings_after_profile_hook_show_verification_ui",
					"selfservice_flows_verification_enabled",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_emailVerificationHooks(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Enable all three Email verification hooks.
			{
				Config: acctest.LoadTestConfig(t, "testdata/email_verification_hooks.tf.tmpl", map[string]string{
					"RequireVerifiedAddress":     "true",
					"RequireVerifiedAddressOIDC": "true",
					"VerifyNewAddress":           "true",
					"NotifyPreviousAddresses":    "true",
					"Recipients":                 "all_verified",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_login_after_password_hook_require_verified_address", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_login_after_oidc_hook_require_verified_address", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_verify_new_address", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_notify_previous_addresses", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients", "all_verified"),
				),
			},
			// Re-apply the same config to confirm no perpetual diff.
			{
				Config: acctest.LoadTestConfig(t, "testdata/email_verification_hooks.tf.tmpl", map[string]string{
					"RequireVerifiedAddress":     "true",
					"RequireVerifiedAddressOIDC": "true",
					"VerifyNewAddress":           "true",
					"NotifyPreviousAddresses":    "true",
					"Recipients":                 "all_verified",
				}),
				PlanOnly: true,
			},
			// Change only the recipient scope: the hook is updated in place.
			{
				Config: acctest.LoadTestConfig(t, "testdata/email_verification_hooks.tf.tmpl", map[string]string{
					"RequireVerifiedAddress":     "true",
					"RequireVerifiedAddressOIDC": "true",
					"VerifyNewAddress":           "true",
					"NotifyPreviousAddresses":    "true",
					"Recipients":                 "all",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients", "all"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_notify_previous_addresses", "true"),
				),
			},
			// Disable the login and notify hooks, keep verify_new_address on.
			// Both hooks share the profile hooks array, so this also covers a
			// removal that must not disturb its neighbour.
			{
				Config: acctest.LoadTestConfig(t, "testdata/email_verification_hooks.tf.tmpl", map[string]string{
					"RequireVerifiedAddress":     "false",
					"RequireVerifiedAddressOIDC": "false",
					"VerifyNewAddress":           "true",
					"NotifyPreviousAddresses":    "false",
					"Recipients":                 "all",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_login_after_password_hook_require_verified_address", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_login_after_oidc_hook_require_verified_address", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_verify_new_address", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_flows_settings_after_profile_hook_notify_previous_addresses", "false"),
				),
			},
			// Import then verify state.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"selfservice_flows_login_after_password_hook_require_verified_address",
					"selfservice_flows_login_after_oidc_hook_require_verified_address",
					"selfservice_flows_settings_after_profile_hook_verify_new_address",
					"selfservice_flows_settings_after_profile_hook_notify_previous_addresses",
					"selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients",
					"selfservice_flows_verification_enabled",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_oauth2Advanced(t *testing.T) {
	// The pairwise salt is here to cover its read path. The API accepts the write
	// at oidc.subject_identifiers.pairwise_salt and reports the value back under
	// the nested oidc.subject_identifiers.pairwise.salt, so reading the write
	// path would null a value the API did return. The PlanOnly step is what
	// catches that.
	const salt = "tf-acc-pairwise-salt"
	templateData := map[string]string{"PairwiseSalt": salt}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/oauth2_advanced.tf.tmpl", templateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_grant_jwt_jti_optional", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_grant_jwt_iat_optional", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_exclude_not_before_claim", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oidc_subject_identifiers_pairwise_salt", salt),
				),
			},
			// Verify no perpetual diff, which is the read-path assertion
			{
				Config:             acctest.LoadTestConfig(t, "testdata/oauth2_advanced.tf.tmpl", templateData),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"oauth2_grant_jwt_max_ttl",
					"oauth2_grant_jwt_jti_optional",
					"oauth2_grant_jwt_iat_optional",
					"oauth2_exclude_not_before_claim",
					"oauth2_client_credentials_default_grant_allowed_scope",
					"oidc_subject_identifiers_supported_types",
					"oidc_subject_identifiers_pairwise_salt",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func TestAccProjectConfigResource_emptyValuesAndEnumFlags(t *testing.T) {
	// Issue #321: the API accepts empty lists/maps/strings and integer zero
	// with HTTP 200 and prunes them from the stored config, so the keys never
	// appear in any read. Nulling state on the missing keys produced a diff on
	// every plan that no apply could settle. The PlanOnly steps are the
	// regression proof.
	//
	// The two bools cover the other perpetual-diff classes:
	// feature_flags_password_profile_registration_node_group is stored as the
	// string enum "password"/"default" (a raw bool write silently lands as
	// "default"), and enable_ax_v2 is reported under the config key "enabled".
	createData := map[string]string{"PasswordProfileNodeGroup": "false", "EnableAXV2": "true"}
	// End at the server defaults (flag true = "password", AX v2 off) so the
	// shared acceptance project is left in its default state.
	updateData := map[string]string{"PasswordProfileNodeGroup": "true", "EnableAXV2": "false"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create — fails on unfixed code with "the refresh plan was not empty".
			{
				Config: acctest.LoadTestConfig(t, "testdata/empty_values.tf.tmpl", createData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_allowed_top_level_claims.#", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_webfinger_oidc_discovery_supported_claims.#", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_webfinger_oidc_discovery_supported_scope.#", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oidc_dynamic_client_registration_default_scope.#", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_captcha_config_allowed_domains.#", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_provider_headers.%", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "oauth2_token_prefix", ""),
					resource.TestCheckResourceAttr("ory_project_config.test", "selfservice_methods_password_config_max_breaches", "0"),
					resource.TestCheckResourceAttr("ory_project_config.test", "feature_flags_password_profile_registration_node_group", "false"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_ax_v2", "true"),
				),
			},
			// Verify no perpetual diff after create
			{
				Config:             acctest.LoadTestConfig(t, "testdata/empty_values.tf.tmpl", createData),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			// Update flips both bools; the empty values stay put.
			{
				Config: acctest.LoadTestConfig(t, "testdata/empty_values.tf.tmpl", updateData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project_config.test", "feature_flags_password_profile_registration_node_group", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_ax_v2", "false"),
				),
			},
			// Verify no perpetual diff after update
			{
				Config:             acctest.LoadTestConfig(t, "testdata/empty_values.tf.tmpl", updateData),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			// ImportState — import only sets id/project_id; Read only refreshes
			// fields that are non-null in state, so config attributes won't be
			// populated until apply.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"oauth2_allowed_top_level_claims",
					"oauth2_webfinger_oidc_discovery_supported_claims",
					"oauth2_webfinger_oidc_discovery_supported_scope",
					"oidc_dynamic_client_registration_default_scope",
					"selfservice_methods_captcha_config_allowed_domains",
					"oauth2_provider_headers",
					"oauth2_token_prefix",
					"selfservice_methods_password_config_max_breaches",
					"feature_flags_password_profile_registration_node_group",
					"enable_ax_v2",
					"cors_enabled",
					"selfservice_methods_password_config_min_password_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}
