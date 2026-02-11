//go:build acceptance

package emailtemplate_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccEmailTemplateResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccEmailTemplateResourceConfig("Your recovery code", "<p>Your code is: {{ .RecoveryCode }}</p>"),
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
				Config: testAccEmailTemplateResourceConfig("Recovery code for your account", "<h1>Recovery</h1><p>Code: {{ .RecoveryCode }}</p>"),
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
				Config: testAccEmailTemplateResourceConfigNoSubject(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "verification_code_valid"),
				),
			},
		},
	})
}

func testAccEmailTemplateResourceConfig(subject, bodyHTML string) string {
	return `
provider "ory" {}

resource "ory_email_template" "test" {
  template_type  = "recovery_code_valid"
  subject        = "` + subject + `"
  body_html      = "` + bodyHTML + `"
  body_plaintext = "Your code is: {{ .RecoveryCode }}"
}
`
}

func testAccEmailTemplateResourceConfigNoSubject() string {
	return `
provider "ory" {}

resource "ory_email_template" "test" {
  template_type  = "verification_code_valid"
  body_html      = "<p>Your verification code is: {{ .VerificationCode }}</p>"
  body_plaintext = "Your verification code is: {{ .VerificationCode }}"
}
`
}
