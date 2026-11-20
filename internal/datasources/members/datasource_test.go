//go:build acceptance

package members_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccMembersDataSources_basic(t *testing.T) {
	acctest.RequireMemberTests(t)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				// The token belongs to a member, so neither list is empty.
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_project_members.test", "project_id"),
					resource.TestCheckResourceAttrSet("data.ory_project_members.test", "members.0.id"),
					resource.TestCheckResourceAttrSet("data.ory_project_members.test", "members.0.email"),
					resource.TestCheckResourceAttrSet("data.ory_project_members.test", "members.0.role"),
					resource.TestCheckResourceAttrSet("data.ory_project_members.test", "members.0.source"),
					resource.TestCheckResourceAttrSet("data.ory_workspace_members.test", "workspace_id"),
					resource.TestCheckResourceAttrSet("data.ory_workspace_members.test", "members.0.id"),
					resource.TestCheckResourceAttrSet("data.ory_workspace_members.test", "members.0.role"),
					resource.TestCheckNoResourceAttr("data.ory_workspace_members.test", "members.0.source"),
				),
			},
		},
	})
}
