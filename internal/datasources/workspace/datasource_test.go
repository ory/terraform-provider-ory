//go:build acceptance

package workspace_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccWorkspaceDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_workspace.test", "id"),
					resource.TestCheckResourceAttrSet("data.ory_workspace.test", "name"),
					resource.TestCheckResourceAttrSet("data.ory_workspace.test", "created_at"),
					resource.TestCheckResourceAttrSet("data.ory_workspace.test", "updated_at"),
				),
			},
		},
	})
}
