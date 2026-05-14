//go:build acceptance

package emailtemplate_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccEmailTemplateResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Subject": "Your recovery code", "BodyHTML": "<p>Your code is: {{ .RecoveryCode }}</p>"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "recovery_code_valid"),
					resource.TestCheckResourceAttr("ory_email_template.test", "subject", "Your recovery code"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_email_template.test",
				ImportState:             true,
				ImportStateId:           "recovery_code_valid",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"body_html", "body_plaintext", "subject"},
			},
			// Update subject and body
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Subject": "Recovery code for your account", "BodyHTML": "<h1>Recovery</h1><p>Code: {{ .RecoveryCode }}</p>"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "recovery_code_valid"),
					resource.TestCheckResourceAttr("ory_email_template.test", "subject", "Recovery code for your account"),
				),
			},
		},
	})
}

func TestAccEmailTemplateResource_noSubject(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create without subject (should use API default)
			{
				Config: acctest.LoadTestConfig(t, "testdata/no_subject.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "verification_code_valid"),
				),
			},
		},
	})
}

// TestAccEmailTemplateResource_drift covers the regression from issue #213:
// when an Ory project stores template content at a storage URL (subject/body
// returned as https://.../<sha512>.{txt,html}), an out-of-band change made
// via the Console used to be invisible to `terraform plan`. We now hash-check
// the URL and fall back to a fetch on mismatch, so drift must be detected
// and a subsequent apply must restore the configured value.
func TestAccEmailTemplateResource_drift(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Step 1: create the template with subject "Original subject".
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Subject":  "Original subject",
					"BodyHTML": "<p>Original body</p>",
				}),
				Check: resource.TestCheckResourceAttr("ory_email_template.test", "subject", "Original subject"),
			},
			// Step 2: rewrite the subject directly via the API. Apply with the
			// original config must detect drift (would have been a silent
			// "no changes" before the fix) and write the original subject back.
			{
				PreConfig: rewriteRecoveryCodeSubjectOutOfBand(t, "OUT-OF-BAND CONSOLE EDIT"),
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Subject":  "Original subject",
					"BodyHTML": "<p>Original body</p>",
				}),
				Check: resource.TestCheckResourceAttr("ory_email_template.test", "subject", "Original subject"),
			},
			// Step 3: re-applying the same config must be a no-op (no
			// perpetual diff from the URL/value comparison).
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Subject":  "Original subject",
					"BodyHTML": "<p>Original body</p>",
				}),
				PlanOnly: true,
			},
		},
	})
}

// rewriteRecoveryCodeSubjectOutOfBand patches the recovery_code_valid
// subject directly via the Console API to simulate someone editing the
// template in the dashboard.
func rewriteRecoveryCodeSubjectOutOfBand(t *testing.T, newSubject string) func() {
	t.Helper()
	return func() {
		c, err := acctest.GetOryClient()
		if err != nil {
			t.Fatalf("Failed to create Ory client: %v", err)
		}
		projectID := acctest.GetTestProjectID(t)
		encoded := "base64://" + base64.StdEncoding.EncodeToString([]byte(newSubject))
		patches := []ory.JsonPatch{
			{
				Op:    "replace",
				Path:  "/services/identity/config/courier/templates/recovery_code/valid/email/subject",
				Value: encoded,
			},
		}
		if _, err := c.PatchProject(context.Background(), projectID, patches); err != nil {
			t.Fatalf("Failed to rewrite subject out-of-band: %v", err)
		}
		t.Logf("Rewrote subject out-of-band to %q", newSubject)
	}
}
