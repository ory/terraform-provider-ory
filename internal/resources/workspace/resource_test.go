//go:build acceptance
package workspace_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
)

func testAccPreCheckImport(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	if os.Getenv("ORY_WORKSPACE_API_KEY") == "" {
		t.Skip("ORY_WORKSPACE_API_KEY must be set for workspace acceptance tests")
	}
	if os.Getenv("ORY_WORKSPACE_ID") == "" {
		t.Skip("ORY_WORKSPACE_ID must be set for workspace import tests")
	}
}

func TestAccWorkspaceResource_import(t *testing.T) {
	workspaceID := os.Getenv("ORY_WORKSPACE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckImport(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:        testAccWorkspaceResourceConfig("placeholder"),
				ImportState:   true,
				ImportStateId: workspaceID,
				ResourceName:  "ory_workspace.test",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 state, got %d", len(states))
					}
					state := states[0]
					if state.ID == "" {
						return fmt.Errorf("expected non-empty ID")
					}
					if state.Attributes["name"] == "" {
						return fmt.Errorf("expected non-empty name")
					}
					return nil
				},
			},
		},
	})
}

func testAccWorkspaceResourceConfig(name string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_workspace" "test" {
  name = %[1]q
}
`, name)
}
