//go:build acceptance
package oauth2client_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
	"github.com/ory/terraform-provider-orynetwork/internal/testutil"
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
