//go:build acceptance

package organization_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccOrganizationDataSource_basic(t *testing.T) {
	acctest.RequireB2BTests(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_organization.test", "id"),
					resource.TestCheckResourceAttrSet("data.ory_organization.test", "label"),
					resource.TestCheckResourceAttrSet("data.ory_organization.test", "created_at"),
				),
			},
		},
	})
}
