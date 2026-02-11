//go:build acceptance

package project_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccProjectDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_project.test", "id"),
					resource.TestCheckResourceAttrSet("data.ory_project.test", "name"),
					resource.TestCheckResourceAttrSet("data.ory_project.test", "slug"),
					resource.TestCheckResourceAttrSet("data.ory_project.test", "state"),
					resource.TestCheckResourceAttrSet("data.ory_project.test", "environment"),
					resource.TestCheckResourceAttrSet("data.ory_project.test", "home_region"),
				),
			},
		},
	})
}

func testAccProjectDataSourceConfig() string {
	return `
provider "ory" {}

data "ory_project" "test" {}
`
}
