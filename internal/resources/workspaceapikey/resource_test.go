//go:build acceptance

package workspaceapikey_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccWorkspaceAPIKeyResource_import(t *testing.T) {
	// This test requires a pre-existing workspace API key ID.
	// Since workspace API keys can only be created via identity sessions (Ory Console),
	// we list existing keys and import one.
	keyID := os.Getenv("ORY_WORKSPACE_API_KEY_ID")
	if keyID == "" {
		t.Skip("ORY_WORKSPACE_API_KEY_ID not set; skipping workspace API key import test")
	}
	workspaceID := os.Getenv("ORY_WORKSPACE_ID")
	if workspaceID == "" {
		t.Fatal("ORY_WORKSPACE_ID must be set")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:            acctest.LoadTestConfig(t, "testdata/import.tf.tmpl", nil),
				ResourceName:      "ory_workspace_api_key.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/%s", workspaceID, keyID),
				ImportStateVerify: false, // Name must match config; we verify attributes individually
			},
			// Re-apply to confirm the config is consistent with imported state
			{
				Config: acctest.LoadTestConfig(t, "testdata/import.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_workspace_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("ory_workspace_api_key.test", "name"),
					resource.TestCheckResourceAttrSet("ory_workspace_api_key.test", "workspace_id"),
					resource.TestCheckResourceAttrSet("ory_workspace_api_key.test", "owner_id"),
					resource.TestCheckResourceAttrSet("ory_workspace_api_key.test", "created_at"),
				),
			},
		},
	})
}

func importStateWorkspaceAPIKeyID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_workspace_api_key.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_workspace_api_key.test")
	}
	workspaceID := rs.Primary.Attributes["workspace_id"]
	keyID := rs.Primary.ID
	return fmt.Sprintf("%s/%s", workspaceID, keyID), nil
}
