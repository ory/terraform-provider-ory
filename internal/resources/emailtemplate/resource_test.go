//go:build acceptance

package emailtemplate_test

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

// Config paths ory_email_template writes for template_type "recovery_code_valid".
//
// Deleting the template does not remove the node. Ory keeps an empty skeleton,
// `{"email": {"body": {}}}`, which is also the shape every untouched template has
// on a project. So the assertion is on the content keys, not on the node.
const (
	recoveryCodeValidSubjectPath  = "/services/identity/config/courier/templates/recovery_code/valid/email/subject"
	recoveryCodeValidBodyHTMLPath = "/services/identity/config/courier/templates/recovery_code/valid/email/body/html"
)

// templateStorageHash returns the hex sha512 the API names a stored template
// file after. The API uploads the decoded `base64://` payload and reports a
// storage URL ending in "<sha512>.txt" or "<sha512>.html". The provider's own
// read path relies on the same property, see resolveStoredTemplate.
func templateStorageHash(content string) string {
	sum := sha512.Sum512([]byte(content))
	return hex.EncodeToString(sum[:])
}

// checkTemplateContentRemoved asserts the API no longer stores the content this
// test wrote. Delete tolerates a "path does not exist" error from the API by
// design, in isPathAlreadyGoneError, so a swallowed error of a different shape
// would leave the template in place while Terraform reported a clean destroy.
// Terraform state cannot see that, hence the direct read. See issue #333.
//
// The assertion is on this test's content rather than on the key being absent,
// because the acceptance tests share one Ory project with CI and with other
// checkouts. A concurrent run writing its own subject to the same template must
// not fail this test, while our own content surviving the destroy must.
func checkTemplateContentRemoved(t *testing.T, subject, bodyHTML string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		return acctest.Eventually(func() error {
			projectID := acctest.GetTestProject(t).ID
			for _, want := range []struct{ path, content string }{
				{recoveryCodeValidSubjectPath, subject},
				{recoveryCodeValidBodyHTMLPath, bodyHTML},
			} {
				value, ok, err := acctest.ProjectConfigValue(t, projectID, want.path)
				if err != nil {
					// A failed read must be retried, not read as "already cleared".
					return fmt.Errorf("could not read project %s after destroy: %w", projectID, err)
				}
				if !ok {
					continue
				}
				if stored, _ := value.(string); strings.Contains(stored, templateStorageHash(want.content)) {
					return fmt.Errorf("%s still stores the content this test wrote after destroy: %s",
						want.path, stored)
				}
			}
			return nil
		})
	}
}

func TestAccEmailTemplateResource_basic(t *testing.T) {
	const (
		createSubject  = "Your recovery code"
		createBodyHTML = "<p>Your code is: {{ .RecoveryCode }}</p>"
		updateSubject  = "Recovery code for your account"
		updateBodyHTML = "<h1>Recovery</h1><p>Code: {{ .RecoveryCode }}</p>"
	)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		// The final step's content is what destroy has to clear.
		CheckDestroy: checkTemplateContentRemoved(t, updateSubject, updateBodyHTML),
		Steps: []resource.TestStep{
			// Create
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Subject": createSubject, "BodyHTML": createBodyHTML}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "recovery_code_valid"),
					resource.TestCheckResourceAttr("ory_email_template.test", "subject", createSubject),
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
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Subject": updateSubject, "BodyHTML": updateBodyHTML}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "recovery_code_valid"),
					resource.TestCheckResourceAttr("ory_email_template.test", "subject", updateSubject),
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
		require.NoError(t, err, "Failed to create Ory client")
		projectID := acctest.GetTestProjectID(t)
		encoded := "base64://" + base64.StdEncoding.EncodeToString([]byte(newSubject))
		patches := []ory.JsonPatch{
			{
				Op:    "replace",
				Path:  "/services/identity/config/courier/templates/recovery_code/valid/email/subject",
				Value: encoded,
			},
		}
		_, err = c.PatchProject(context.Background(), projectID, patches)
		require.NoError(t, err, "Failed to rewrite subject out-of-band")
		t.Logf("Rewrote subject out-of-band to %q", newSubject)
	}
}
