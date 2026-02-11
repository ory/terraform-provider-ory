//go:build acceptance

package socialprovider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

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
				Config: testAccSocialProviderResourceConfig(),
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

func testAccSocialProviderResourceConfig() string {
	return `
provider "ory" {}

resource "ory_social_provider" "test" {
  provider_id   = "test-google"
  provider_type = "google"
  client_id     = "test-client-id.apps.googleusercontent.com"
  client_secret = "test-client-secret"
  scope         = ["email", "profile"]
}
`
}
