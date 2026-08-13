//go:build acceptance

package workspace_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
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

// checkWorkspaceSurvivesDestroy asserts the workspace is still there after
// destroy. ory_workspace has an intentional no-op Delete, because Ory has no
// workspace delete endpoint, and the resource warns as much. This is the assertion
// that matters most of the set: the resource is normally adopted by import, so a
// Delete that ever started calling an API would destroy a workspace the user only
// meant to stop managing. See issue #333.
func checkWorkspaceSurvivesDestroy(t *testing.T, workspaceID string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		return acctest.Eventually(func() error {
			exists, err := acctest.WorkspaceExists(t, workspaceID)
			if err != nil {
				return fmt.Errorf("could not read workspace %s after destroy: %w", workspaceID, err)
			}
			if !exists {
				return fmt.Errorf("workspace %s is gone after destroy, it must be left alone", workspaceID)
			}
			return nil
		})
	}
}

func TestAccWorkspaceResource_import(t *testing.T) {
	workspaceID := os.Getenv("ORY_WORKSPACE_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckImport(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkWorkspaceSurvivesDestroy(t, workspaceID),
		Steps: []resource.TestStep{
			{
				Config:        acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Name": "placeholder"}),
				ImportState:   true,
				ImportStateId: workspaceID,
				ResourceName:  "ory_workspace.test",
				// Persist the imported state so the test case has something to tear
				// down. Without this the step leaves no state, no destroy runs, and
				// CheckDestroy never fires, which is what makes the assertion below
				// worth anything.
				ImportStatePersist: true,
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
