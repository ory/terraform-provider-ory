//go:build acceptance

package organization_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
	"github.com/ory/terraform-provider-orynetwork/internal/testutil"
)

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
			// ImportState
			{
				ResourceName:      "ory_organization.test",
				ImportState:       true,
				ImportStateVerify: true,
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
