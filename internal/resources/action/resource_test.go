//go:build acceptance

package action_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccActionResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
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

func actionImportStateIDFunc(s *terraform.State) (string, error) {
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
}

func TestAccActionResource_withBasicAuth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with basic auth
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_basic_auth.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "registration"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "after"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_type", "basic_auth"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_basic_auth_user", "webhook-user"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_basic_auth_password", "webhook-password"),
				),
			},
			// Import
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: actionImportStateIDFunc,
				ImportStateVerify: true,
				// Sensitive fields may not be returned by the API
				ImportStateVerifyIgnore: []string{
					"webhook_auth_basic_auth_password",
				},
			},
			// Update auth credentials
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_basic_auth_updated.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_type", "basic_auth"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_basic_auth_user", "updated-user"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_basic_auth_password", "updated-password"),
				),
			},
		},
	})
}

func TestAccActionResource_withAPIKeyAuth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with API key auth
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_api_key_auth.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "registration"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "after"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_type", "api_key"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_api_key_name", "X-API-KEY"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_api_key_value", "test-api-key-value"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_api_key_in", "header"),
				),
			},
			// Import
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: actionImportStateIDFunc,
				ImportStateVerify: true,
				// Sensitive fields may not be returned by the API
				ImportStateVerifyIgnore: []string{
					"webhook_auth_api_key_value",
				},
			},
			// Update to cookie-based API key
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_api_key_auth_updated.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_type", "api_key"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_api_key_name", "X-Custom-Auth"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_api_key_value", "updated-api-key-value"),
					resource.TestCheckResourceAttr("ory_action.test", "webhook_auth_api_key_in", "cookie"),
				),
			},
		},
	})
}
