//go:build acceptance

package oauth2client_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccOAuth2ClientResource_basic(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Name": "Test API Client",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test API Client"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read"),
					// client_secret is only returned on create
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "client_secret"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_oauth2_client.test",
				ImportState:       true,
				ImportStateVerify: true,
				// client_secret is only returned on create, not on read
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", map[string]string{
					"Name": "Test API Client Updated",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test API Client Updated"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read api:write"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withAudience(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_audience.tf.tmpl", map[string]string{
					"Name":   "Test Client with Audience",
					"APIURL": testutil.ExampleAPIURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with Audience"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "audience.#", "2"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withRedirectURIs(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_redirect_uris.tf.tmpl", map[string]string{
					"Name":   "Test Client with Redirects",
					"AppURL": testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with Redirects"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "redirect_uris.#", "2"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withNewFields(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_new_fields.tf.tmpl", map[string]string{
					"Name":   "Test Client Extended",
					"AppURL": testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Extended"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "allowed_cors_origins.#", "2"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_uri", testutil.ExampleAppURL),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "logo_uri", testutil.ExampleAppURL+"/logo.png"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "policy_uri", testutil.ExampleAppURL+"/privacy"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "tos_uri", testutil.ExampleAppURL+"/tos"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

func TestAccOAuth2ClientResource_withConsentAndSubjectType(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with skip_consent, skip_logout_consent, subject_type, contacts
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_consent.tf.tmpl", map[string]string{
					"Name":        "Test Client Consent",
					"AppURL":      testutil.ExampleAppURL,
					"EmailDomain": testutil.ExampleEmailDomain,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Consent"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "skip_consent", "true"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "skip_logout_consent", "true"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "subject_type", "public"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "contacts.#", "2"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

func TestAccOAuth2ClientResource_withTokenLifespans(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_lifespans.tf.tmpl", map[string]string{
					"Name":   "Test Client Lifespans",
					"AppURL": testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Lifespans"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "authorization_code_grant_access_token_lifespan", "1h"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "authorization_code_grant_refresh_token_lifespan", "720h"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_credentials_grant_access_token_lifespan", "30m"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "backchannel_logout_session_required", "true"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "frontchannel_logout_session_required", "true"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}
