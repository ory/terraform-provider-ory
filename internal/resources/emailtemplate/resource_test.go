//go:build acceptance
package emailtemplate_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
)

func TestAccEmailTemplateResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccEmailTemplateResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_email_template.test", "id"),
					resource.TestCheckResourceAttr("ory_email_template.test", "template_type", "recovery_code_valid"),
				),
			},
		},
	})
}

func testAccEmailTemplateResourceConfig() string {
	return `
provider "ory" {}

resource "ory_email_template" "test" {
  template_type  = "recovery_code_valid"
  subject        = "Your recovery code"
  body_html      = "<p>Your code is: {{ .RecoveryCode }}</p>"
  body_plaintext = "Your code is: {{ .RecoveryCode }}"
}
`
}
