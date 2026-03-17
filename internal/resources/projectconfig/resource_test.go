//go:build acceptance

package projectconfig_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

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
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/tokenizer_templates.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.my_jwt.ttl", "1h"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.ttl", "5m"),
					resource.TestCheckResourceAttr("ory_project_config.test", "session_tokenizer_templates.short_token.subject_source", "external_id"),
				),
			},
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
