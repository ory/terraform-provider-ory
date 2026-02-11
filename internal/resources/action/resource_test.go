//go:build acceptance

package action_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccActionResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccActionResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "registration"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "after"),
					resource.TestCheckResourceAttr("ory_action.test", "auth_method", "password"),
					resource.TestCheckResourceAttr("ory_action.test", "method", "POST"),
				),
			},
			// Import using the new 6-part format: project_id:flow:timing:auth_method:method:url
			{
				ResourceName: "ory_action.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["ory_action.test"]
					if !ok {
						return "", fmt.Errorf("resource not found: ory_action.test")
					}
					projectID := rs.Primary.Attributes["project_id"]
					flow := rs.Primary.Attributes["flow"]
					timing := rs.Primary.Attributes["timing"]
					authMethod := rs.Primary.Attributes["auth_method"]
					method := rs.Primary.Attributes["method"]
					url := rs.Primary.Attributes["url"]
					return fmt.Sprintf("%s:%s:%s:%s:%s:%s", projectID, flow, timing, authMethod, method, url), nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func testAccActionResourceConfig() string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_action" "test" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "%s/user-registered"
  method      = "POST"
}
`, testutil.ExampleWebhookURL)
}
