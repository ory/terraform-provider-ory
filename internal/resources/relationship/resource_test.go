//go:build acceptance

package relationship_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
)

func TestAccRelationshipResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireKetoTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccRelationshipResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_relationship.test", "id"),
					resource.TestCheckResourceAttr("ory_relationship.test", "namespace", "documents"),
					resource.TestCheckResourceAttr("ory_relationship.test", "object", "doc-123"),
					resource.TestCheckResourceAttr("ory_relationship.test", "relation", "viewer"),
					resource.TestCheckResourceAttr("ory_relationship.test", "subject_id", "user-456"),
				),
			},
		},
	})
}

func testAccRelationshipResourceConfig() string {
	return `
provider "ory" {}

resource "ory_relationship" "test" {
  namespace  = "documents"
  object     = "doc-123"
  relation   = "viewer"
  subject_id = "user-456"
}
`
}
