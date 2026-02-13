//go:build acceptance

package oauth2client_test

import (
	"fmt"
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
				Config: testAccOAuth2ClientResourceConfig("Test API Client"),
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
				Config: testAccOAuth2ClientResourceConfigUpdated("Test API Client Updated"),
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
				Config: testAccOAuth2ClientResourceConfigWithAudience("Test Client with Audience"),
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
				Config: testAccOAuth2ClientResourceConfigWithRedirectURIs("Test Client with Redirects"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with Redirects"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "redirect_uris.#", "2"),
				),
			},
		},
	})
}

func testAccOAuth2ClientResourceConfig(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["client_credentials"]
  response_types = ["token"]
  scope          = "api:read"
}
`, name)
}

func testAccOAuth2ClientResourceConfigUpdated(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["client_credentials"]
  response_types = ["token"]
  scope          = "api:read api:write"
}
`, name)
}

func testAccOAuth2ClientResourceConfigWithAudience(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["client_credentials"]
  response_types = ["token"]
  scope          = "api:read"
  audience       = ["%[2]s", "%[2]s/v2"]
}
`, name, testutil.ExampleAPIURL)
}

func testAccOAuth2ClientResourceConfigWithRedirectURIs(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid profile email"
  redirect_uris  = ["%[2]s/callback", "http://localhost:3000/callback"]
}
`, name, testutil.ExampleAppURL)
}

func TestAccOAuth2ClientResource_withNewFields(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccOAuth2ClientResourceConfigWithNewFields("Test Client Extended"),
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

func testAccOAuth2ClientResourceConfigWithNewFields(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid profile email"
  redirect_uris  = ["%[2]s/callback"]

  allowed_cors_origins = ["%[2]s", "http://localhost:3000"]
  client_uri           = "%[2]s"
  logo_uri             = "%[2]s/logo.png"
  policy_uri           = "%[2]s/privacy"
  tos_uri              = "%[2]s/tos"
}
`, name, testutil.ExampleAppURL)
}

func TestAccOAuth2ClientResource_withConsentAndSubjectType(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with skip_consent, skip_logout_consent, subject_type, contacts
			{
				Config: testAccOAuth2ClientResourceConfigWithConsent("Test Client Consent"),
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
				Config: testAccOAuth2ClientResourceConfigWithLifespans("Test Client Lifespans"),
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

func testAccOAuth2ClientResourceConfigWithLifespans(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["authorization_code", "client_credentials", "refresh_token"]
  response_types = ["code"]
  scope          = "openid profile email"
  redirect_uris  = ["%[2]s/callback"]

  authorization_code_grant_access_token_lifespan  = "1h"
  authorization_code_grant_refresh_token_lifespan = "720h"
  client_credentials_grant_access_token_lifespan  = "30m"

  backchannel_logout_session_required  = true
  frontchannel_logout_session_required = true
}
`, name, testutil.ExampleAppURL)
}

func testAccOAuth2ClientResourceConfigWithConsent(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_oauth2_client" "test" {
  client_name = %[1]q

  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid profile email"
  redirect_uris  = ["%[2]s/callback"]

  skip_consent        = true
  skip_logout_consent = true
  subject_type        = "public"
  contacts            = ["admin@%[3]s", "dev@%[3]s"]
}
`, name, testutil.ExampleAppURL, testutil.ExampleEmailDomain)
}
