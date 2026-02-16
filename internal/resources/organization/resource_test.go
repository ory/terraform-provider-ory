//go:build acceptance

package organization_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
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
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Label": "Test Organization"}),
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
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Label": "Test Organization Updated"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_organization.test", "id"),
					resource.TestCheckResourceAttr("ory_organization.test", "label", "Test Organization Updated"),
				),
			},
		},
	})
}

func TestAccOrganizationResource_withDomains(t *testing.T) {
	twoDomains := fmt.Sprintf("[%q, %q]", testutil.ExampleEmailDomain, "test."+testutil.ExampleEmailDomain)
	threeDomains := fmt.Sprintf("[%q, %q, %q]", testutil.ExampleEmailDomain, "test."+testutil.ExampleEmailDomain, "new."+testutil.ExampleEmailDomain)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckB2B(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_domains.tf.tmpl", map[string]string{"Label": "Org with Domains", "DomainList": twoDomains}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_organization.test", "id"),
					resource.TestCheckResourceAttr("ory_organization.test", "label", "Org with Domains"),
					resource.TestCheckResourceAttr("ory_organization.test", "domains.#", "2"),
				),
			},
			// Update domains
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_domains.tf.tmpl", map[string]string{"Label": "Org with Domains", "DomainList": threeDomains}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_organization.test", "domains.#", "3"),
				),
			},
		},
	})
}
