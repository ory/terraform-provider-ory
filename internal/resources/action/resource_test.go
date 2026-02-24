//go:build acceptance

package action_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// cleanupDanglingWebhook removes a webhook left behind by a previous failed test run.
// This makes the test idempotent when sharing a single smoke-test project.
func cleanupDanglingWebhook(t *testing.T, hookPath, webhookURL string) {
	t.Helper()

	c, err := acctest.GetOryClient()
	if err != nil {
		t.Logf("Warning: could not create client for cleanup: %v", err)
		return
	}

	project := acctest.GetTestProject(t)
	ctx := context.Background()

	p, err := c.GetProject(ctx, project.ID)
	if err != nil {
		t.Logf("Warning: could not get project for cleanup: %v", err)
		return
	}

	// Navigate to the hooks via the identity config
	configMap := p.Services.Identity.Config
	if configMap == nil {
		return
	}

	// Walk the hookPath segments (e.g. "registration", "after", "password")
	// to find the hooks array.
	segments := []string{"selfservice", "flows", "registration", "after", "password"}
	var current interface{} = configMap
	for _, seg := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			return
		}
		current = m[seg]
	}

	hooksSlice, ok := current.(map[string]interface{})["hooks"].([]interface{})
	if !ok || len(hooksSlice) == 0 {
		return
	}

	// Check if our test webhook is in there
	found := false
	for _, h := range hooksSlice {
		hm, _ := h.(map[string]interface{})
		if hm["hook"] == "web_hook" {
			cfg, _ := hm["config"].(map[string]interface{})
			if url, _ := cfg["url"].(string); url == webhookURL {
				found = true
				break
			}
		}
	}

	if !found {
		return
	}

	t.Logf("Cleaning up dangling webhook at %s: %s", hookPath, webhookURL)
	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  hookPath,
		Value: []interface{}{},
	}}
	_, err = c.PatchProject(ctx, project.ID, patches)
	if err != nil {
		t.Logf("Warning: failed to clean up dangling webhook: %v", err)
	}
}

func TestAccActionResource_basic(t *testing.T) {
	webhookURL := testutil.ExampleWebhookURL + "/user-registered"
	hookPath := "/services/identity/config/selfservice/flows/registration/after/password/hooks"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, webhookURL)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
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
