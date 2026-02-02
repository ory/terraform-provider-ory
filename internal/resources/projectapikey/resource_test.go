//go:build acceptance

package projectapikey_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
)

func TestAccProjectAPIKeyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectAPIKeyResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_api_key.test", "id"),
					resource.TestCheckResourceAttr("ory_project_api_key.test", "name", "tf-test-key"),
					resource.TestCheckResourceAttrSet("ory_project_api_key.test", "value"),
					resource.TestCheckResourceAttrSet("ory_project_api_key.test", "owner_id"),
				),
			},
		},
	})
}

func testAccProjectAPIKeyResourceConfig() string {
	return `
provider "ory" {}

resource "ory_project_api_key" "test" {
  name = "tf-test-key"
}
`
}
