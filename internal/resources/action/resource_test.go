//go:build acceptance

package action_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// TestAccActionResource_writeOnlyBasicAuth verifies that the basic-auth password
// supplied via webhook_auth_basic_auth_password_wo is sent to the API but never
// written to Terraform state, and that bumping the version trigger rotates it.
// Write-only arguments require Terraform 1.11+.
func TestAccActionResource_writeOnlyBasicAuth(t *testing.T) {
	hookPath := "/services/identity/config/selfservice/flows/registration/after/password/hooks"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, testutil.ExampleWebhookURL+"/wo-basic-auth-webhook")
		},
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_basic_auth.tf.tmpl", map[string]string{
					"WebhookURL": testutil.ExampleWebhookURL,
					"Version":    "1",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_type", "basic_auth"),
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_basic_auth_user", "webhook-user"),
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_basic_auth_password_wo_version", "1"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_action.wo", tfjsonpath.New("webhook_auth_basic_auth_password_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_action.wo", tfjsonpath.New("webhook_auth_basic_auth_password"), knownvalue.Null()),
				},
			},
			// Rotate via version bump.
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_basic_auth.tf.tmpl", map[string]string{
					"WebhookURL": testutil.ExampleWebhookURL,
					"Version":    "2",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_basic_auth_password_wo_version", "2"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_action.wo", tfjsonpath.New("webhook_auth_basic_auth_password_wo"), knownvalue.Null()),
				},
			},
			// Idempotency.
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_basic_auth.tf.tmpl", map[string]string{
					"WebhookURL": testutil.ExampleWebhookURL,
					"Version":    "2",
				}),
				PlanOnly: true,
			},
		},
	})
}

// TestAccActionResource_writeOnlyAPIKey verifies the same for the api-key value
// supplied via webhook_auth_api_key_value_wo.
func TestAccActionResource_writeOnlyAPIKey(t *testing.T) {
	hookPath := "/services/identity/config/selfservice/flows/registration/after/password/hooks"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, testutil.ExampleWebhookURL+"/wo-api-key-webhook")
		},
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_api_key.tf.tmpl", map[string]string{
					"WebhookURL": testutil.ExampleWebhookURL,
					"Version":    "1",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_type", "api_key"),
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_api_key_name", "X-API-KEY"),
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_api_key_value_wo_version", "1"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_action.wo", tfjsonpath.New("webhook_auth_api_key_value_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_action.wo", tfjsonpath.New("webhook_auth_api_key_value"), knownvalue.Null()),
				},
			},
			// Rotate via version bump.
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_api_key.tf.tmpl", map[string]string{
					"WebhookURL": testutil.ExampleWebhookURL,
					"Version":    "2",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.wo", "webhook_auth_api_key_value_wo_version", "2"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_action.wo", tfjsonpath.New("webhook_auth_api_key_value_wo"), knownvalue.Null()),
				},
			},
			// Idempotency.
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_api_key.tf.tmpl", map[string]string{
					"WebhookURL": testutil.ExampleWebhookURL,
					"Version":    "2",
				}),
				PlanOnly: true,
			},
		},
	})
}

// cleanupDanglingWebhook removes a specific webhook left behind by a previous
// failed test run. Only the matching webhook is removed; other hooks at the
// same path are preserved. hookPath must be a full JSON Patch path ending in
// "/hooks" (e.g. "/services/identity/config/selfservice/flows/registration/after/password/hooks").
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

	configMap := p.Services.Identity.Config
	if configMap == nil {
		return
	}

	// Derive navigation segments from hookPath.
	// e.g. "/services/identity/config/selfservice/flows/registration/after/password/hooks"
	// → strip prefix "/services/identity/config/" and suffix "/hooks"
	// → segments: ["selfservice", "flows", "registration", "after", "password"]
	trimmed := strings.TrimPrefix(hookPath, "/services/identity/config/")
	trimmed = strings.TrimSuffix(trimmed, "/hooks")
	segments := strings.Split(trimmed, "/")

	var current interface{} = configMap
	for _, seg := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			return
		}
		current = m[seg]
		if current == nil {
			return
		}
	}

	hooksSlice, ok := current.(map[string]interface{})["hooks"].([]interface{})
	if !ok || len(hooksSlice) == 0 {
		return
	}

	// Build a new hooks list without the dangling test webhook.
	filtered := make([]interface{}, 0, len(hooksSlice))
	found := false
	for _, h := range hooksSlice {
		hm, _ := h.(map[string]interface{})
		if hm["hook"] == "web_hook" {
			cfg, _ := hm["config"].(map[string]interface{})
			if url, _ := cfg["url"].(string); url == webhookURL {
				found = true
				continue // skip the dangling webhook
			}
		}
		filtered = append(filtered, h)
	}

	if !found {
		return
	}

	t.Logf("Cleaning up dangling webhook at %s: %s", hookPath, webhookURL)
	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  hookPath,
		Value: filtered,
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
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: actionImportStateIDFunc,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccActionResource_samlLogin is the regression test for issue #305: the
// SAML authentication method is offered in the Ory Console UI for after-login
// hooks but the provider's auth_method validator rejected "saml". This exercises
// create, read, import, and delete of a webhook bound to the SAML method on the
// login flow (stored at .../login/after/saml/hooks, alongside any organization
// hook already configured for SAML SSO).
func TestAccActionResource_samlLogin(t *testing.T) {
	webhookURL := testutil.ExampleWebhookURL + "/saml-after-login"
	hookPath := "/services/identity/config/selfservice/flows/login/after/saml/hooks"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, webhookURL)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/saml_login.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "login"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "after"),
					resource.TestCheckResourceAttr("ory_action.test", "auth_method", "saml"),
					resource.TestCheckResourceAttr("ory_action.test", "method", "POST"),
				),
			},
			// Import using the 6-part format: project_id:flow:timing:auth_method:method:url
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: actionImportStateIDFunc,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccActionResource_beforeTiming is the regression test for issue #280:
// before-timing actions could not be imported because the url's own colons
// (https://...) broke the segment-counting import ID parser. Both documented
// before formats are exercised: with an explicit method and the legacy form
// without one. The webhook url also embeds a nested scheme in a query param
// (?redirect=https://...) to guard against the method detector mistaking it for
// an invalid HTTP method when the method segment is omitted.
func TestAccActionResource_beforeTiming(t *testing.T) {
	webhookURL := testutil.ExampleWebhookURL + "/before-login?redirect=https://callback.example.com"
	hookPath := "/services/identity/config/selfservice/flows/login/before/hooks"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, webhookURL)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/before_login.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "login"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "before"),
					resource.TestCheckResourceAttr("ory_action.test", "method", "POST"),
				),
			},
			// Import using the documented format: project_id:flow:before:method:url
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: beforeActionImportStateIDFunc(true),
				ImportStateVerify: true,
			},
			// Import using the legacy format without method: project_id:flow:before:url
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: beforeActionImportStateIDFunc(false),
				ImportStateVerify: true,
			},
		},
	})
}

// beforeActionImportStateIDFunc builds a before-timing import ID from state,
// with or without the optional method segment.
func beforeActionImportStateIDFunc(includeMethod bool) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources["ory_action.test"]
		if !ok {
			return "", fmt.Errorf("resource not found: ory_action.test")
		}
		projectID := rs.Primary.Attributes["project_id"]
		flow := rs.Primary.Attributes["flow"]
		url := rs.Primary.Attributes["url"]
		if includeMethod {
			return fmt.Sprintf("%s:%s:before:%s:%s", projectID, flow, rs.Primary.Attributes["method"], url), nil
		}
		return fmt.Sprintf("%s:%s:before:%s", projectID, flow, url), nil
	}
}

// TestAccActionResource_verificationFlow is the regression test for issue #241:
// the verification flow's "after" hooks are stored in a flat array at
// .../verification/after/hooks (no auth_method level). Before the fix, the
// provider PATCHed to .../verification/after/password/hooks, which the API
// accepted with 200 but silently dropped — failing with "Hook not found in
// PatchProject response". This exercises create, read, update, import, delete.
func TestAccActionResource_verificationFlow(t *testing.T) {
	hookPath := "/services/identity/config/selfservice/flows/verification/after/hooks"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, testutil.ExampleWebhookURL+"/verification-after")
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/verification.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "verification"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "after"),
					resource.TestCheckResourceAttr("ory_action.test", "method", "POST"),
					resource.TestCheckResourceAttr("ory_action.test", "response_ignore", "false"),
				),
			},
			// Import (auth_method defaults to "password" in the composite ID but is ignored for this flow)
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: actionImportStateIDFunc,
				ImportStateVerify: true,
			},
			// Update an in-place attribute
			{
				Config: acctest.LoadTestConfig(t, "testdata/verification_updated.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_action.test", "response_ignore", "true"),
				),
			},
		},
	})
}

// TestAccActionResource_recoveryFlow verifies the recovery flow (also flat,
// no auth_method level) creates, reads, and imports correctly. See issue #241.
func TestAccActionResource_recoveryFlow(t *testing.T) {
	hookPath := "/services/identity/config/selfservice/flows/recovery/after/hooks"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, testutil.ExampleWebhookURL+"/recovery-after")
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/recovery.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_action.test", "id"),
					resource.TestCheckResourceAttr("ory_action.test", "flow", "recovery"),
					resource.TestCheckResourceAttr("ory_action.test", "timing", "after"),
					resource.TestCheckResourceAttr("ory_action.test", "method", "POST"),
				),
			},
			// Import
			{
				ResourceName:      "ory_action.test",
				ImportState:       true,
				ImportStateIdFunc: actionImportStateIDFunc,
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
	hookPath := "/services/identity/config/selfservice/flows/registration/after/password/hooks"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, testutil.ExampleWebhookURL+"/basic-auth-webhook")
		},
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

// TestAccActionResource_parallel verifies that creating and destroying multiple
// actions in the same flow/timing/auth_method does not suffer from
// read-modify-write races. Without per-project serialization, Terraform's
// default parallelism causes concurrent goroutines to read the same hooks
// array, append their own hook, and replace the array — so the last writer
// wins and the other hooks are silently dropped. See issue #189.
func TestAccActionResource_parallel(t *testing.T) {
	hookPath := "/services/identity/config/selfservice/flows/registration/after/password/hooks"
	urls := []string{
		testutil.ExampleWebhookURL + "/parallel-a",
		testutil.ExampleWebhookURL + "/parallel-b",
		testutil.ExampleWebhookURL + "/parallel-c",
		testutil.ExampleWebhookURL + "/parallel-d",
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			for _, u := range urls {
				cleanupDanglingWebhook(t, hookPath, u)
			}
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/parallel.tf.tmpl", map[string]string{"WebhookURL": testutil.ExampleWebhookURL}),
				Check:  allParallelHooksExist(hookPath, urls),
			},
		},
	})
}

// allParallelHooksExist asserts that every URL in urls is present in the hooks
// array at hookPath on the test project. This is the invariant issue #189
// breaks: all resources report success, but only some of them actually landed.
func allParallelHooksExist(hookPath string, urls []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c, err := acctest.GetOryClient()
		if err != nil {
			return fmt.Errorf("could not create client: %w", err)
		}

		var projectID string
		for _, rs := range s.RootModule().Resources {
			if pid := rs.Primary.Attributes["project_id"]; pid != "" {
				projectID = pid
				break
			}
		}
		if projectID == "" {
			return fmt.Errorf("no project_id found in state")
		}

		ctx := context.Background()
		p, err := c.GetProject(ctx, projectID)
		if err != nil {
			return fmt.Errorf("could not get project: %w", err)
		}

		trimmed := strings.TrimPrefix(hookPath, "/services/identity/config/")
		trimmed = strings.TrimSuffix(trimmed, "/hooks")
		segments := strings.Split(trimmed, "/")

		var current interface{} = p.Services.Identity.Config
		for _, seg := range segments {
			m, ok := current.(map[string]interface{})
			if !ok {
				return fmt.Errorf("unexpected type at segment %q", seg)
			}
			current = m[seg]
			if current == nil {
				return fmt.Errorf("nothing found at segment %q", seg)
			}
		}
		hooks, _ := current.(map[string]interface{})["hooks"].([]interface{})

		found := make(map[string]bool, len(urls))
		for _, h := range hooks {
			hm, _ := h.(map[string]interface{})
			if hm["hook"] != "web_hook" {
				continue
			}
			cfg, _ := hm["config"].(map[string]interface{})
			if url, _ := cfg["url"].(string); url != "" {
				found[url] = true
			}
		}

		for _, u := range urls {
			if !found[u] {
				return fmt.Errorf("expected webhook %q to exist at %s but it was not found (race condition clobbered it)", u, hookPath)
			}
		}
		return nil
	}
}

func TestAccActionResource_withAPIKeyAuth(t *testing.T) {
	hookPath := "/services/identity/config/selfservice/flows/registration/after/password/hooks"
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			cleanupDanglingWebhook(t, hookPath, testutil.ExampleWebhookURL+"/api-key-webhook")
		},
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
