//go:build acceptance

package organization_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
	"github.com/ory/terraform-provider-orynetwork/internal/testutil"
)

func importStateOrganizationID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_organization.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_organization.test")
	}
	projectID := rs.Primary.Attributes["project_id"]
	orgID := rs.Primary.ID
	return fmt.Sprintf("%s/%s", projectID, orgID), nil
}

func testAccPreCheckB2B(t *testing.T) {
	acctest.AccPreCheck(t)
	acctest.RequireB2BTests(t)

	// Organizations are not available in development (dev) projects
	// They require 'prod' or 'stage' environment
	env := os.Getenv("ORY_PROJECT_ENVIRONMENT")
	if env == "dev" || env == "" {
		t.Skip("Organization tests require ORY_PROJECT_ENVIRONMENT to be 'prod' or 'stage' (not 'dev')")
	}
}

func TestAccOrganizationResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckB2B(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccOrganizationResourceConfig("Test Organization"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_organization.test", "id"),
					resource.TestCheckResourceAttr("ory_organization.test", "label", "Test Organization"),
					resource.TestCheckResourceAttrSet("ory_organization.test", "created_at"),
				),
			},
			// ImportState using composite ID: project_id/org_id
			{
				ResourceName:      "ory_organization.test",
				ImportState:       true,
				ImportStateIdFunc: importStateOrganizationID,
				ImportStateVerify: true,
				// created_at timestamp precision differs between create and read
				ImportStateVerifyIgnore: []string{"created_at"},
			},
			// Update
			{
				Config: testAccOrganizationResourceConfig("Test Organization Updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_organization.test", "id"),
					resource.TestCheckResourceAttr("ory_organization.test", "label", "Test Organization Updated"),
				),
			},
		},
	})
}

func TestAccOrganizationResource_withDomains(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckB2B(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationResourceConfigWithDomains("Org with Domains", []string{testutil.ExampleEmailDomain, fmt.Sprintf("test.%s", testutil.ExampleEmailDomain)}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_organization.test", "id"),
					resource.TestCheckResourceAttr("ory_organization.test", "label", "Org with Domains"),
					resource.TestCheckResourceAttr("ory_organization.test", "domains.#", "2"),
				),
			},
			// Update domains
			{
				Config: testAccOrganizationResourceConfigWithDomains("Org with Domains", []string{testutil.ExampleEmailDomain, fmt.Sprintf("test.%s", testutil.ExampleEmailDomain), fmt.Sprintf("new.%s", testutil.ExampleEmailDomain)}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_organization.test", "domains.#", "3"),
				),
			},
		},
	})
}

func testAccOrganizationResourceConfig(label string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_organization" "test" {
  label = %[1]q
}
`, label)
}

func testAccOrganizationResourceConfigWithDomains(label string, domains []string) string {
	// Build domain list for HCL
	domainList := ""
	for i, d := range domains {
		if i > 0 {
			domainList += ", "
		}
		domainList += fmt.Sprintf("%q", d)
	}

	return fmt.Sprintf(`
provider "ory" {}

resource "ory_organization" "test" {
  label   = %[1]q
  domains = [%[2]s]
}
`, label, domainList)
}
