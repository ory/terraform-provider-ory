//go:build acceptance

package projectconfig_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccProjectConfigResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccProjectConfigResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "cors_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "password_min_length", "10"),
				),
			},
			// ImportState - after import, Read only refreshes fields that are
			// non-null in state. Since import only sets id/project_id, config
			// fields won't be populated until the user runs terraform apply.
			{
				ResourceName:      "ory_project_config.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"cors_enabled", "cors_origins", "password_min_length",
					"smtp_connection_uri",
				},
			},
		},
	})
}

func testAccProjectConfigResourceConfig() string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_project_config" "test" {
  cors_enabled        = true
  cors_origins        = ["%s"]
  password_min_length = 10
}
`, testutil.ExampleAppURL)
}

func TestAccProjectConfigResource_mfaPolicy(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigResourceMFAConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "enable_totp", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "totp_issuer", "TerraformTest"),
				),
			},
		},
	})
}

func testAccProjectConfigResourceMFAConfig() string {
	return `
provider "ory" {}

resource "ory_project_config" "test" {
  enable_totp  = true
  totp_issuer  = "TerraformTest"
}
`
}

func TestAccProjectConfigResource_accountExperience(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigResourceAccountExperienceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_name", "TF Test App"),
					resource.TestCheckResourceAttr("ory_project_config.test", "account_experience_default_locale", "en"),
				),
			},
		},
	})
}

func testAccProjectConfigResourceAccountExperienceConfig() string {
	return `
provider "ory" {}

resource "ory_project_config" "test" {
  account_experience_name           = "TF Test App"
  account_experience_default_locale = "en"
}
`
}

func TestAccProjectConfigResource_adminCORS(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigResourceAdminCORSConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_config.test", "id"),
					resource.TestCheckResourceAttr("ory_project_config.test", "cors_admin_enabled", "true"),
					resource.TestCheckResourceAttr("ory_project_config.test", "cors_admin_origins.#", "1"),
				),
			},
		},
	})
}

func testAccProjectConfigResourceAdminCORSConfig() string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_project_config" "test" {
  cors_admin_enabled = true
  cors_admin_origins = ["%s"]
}
`, testutil.ExampleAppURL)
}
