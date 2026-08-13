//go:build acceptance

package project_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("ORY_WORKSPACE_API_KEY"); v == "" {
		t.Skip("ORY_WORKSPACE_API_KEY must be set for project acceptance tests")
	}
	// Project creation/deletion is expensive and may have quotas
	// Only run if explicitly enabled
	if os.Getenv("ORY_PROJECT_TESTS_ENABLED") != "true" {
		t.Skip("ORY_PROJECT_TESTS_ENABLED must be 'true' to run project tests (creates/deletes real projects)")
	}
}

// TestAccProjectResource_basic tests the full CRUD lifecycle of a project.
// WARNING: This test creates and deletes a real Ory project.
// Only run this test if you have quota available and understand the implications.
// checkProjectDeleted asserts the API reports the project as deleted. Projects
// are soft deleted, so GetProject still returns HTTP 200 with state "deleted"
// rather than a 404, and a Delete that no-ops would leave state "running" while
// Terraform reported a clean destroy. Terraform state cannot see that, hence the
// direct read. See issue #333.
func checkProjectDeleted(t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			// The resource is gone from state, which is the normal post-destroy
			// shape. Without an id there is nothing left to look up.
			return nil
		}
		projectID := rs.Primary.ID
		if projectID == "" {
			return nil
		}
		return acctest.Eventually(func() error {
			state, err := acctest.ProjectState(t, projectID)
			if err != nil {
				// A purged project reads as not found, which also means deleted.
				return nil
			}
			if state != "deleted" {
				return fmt.Errorf("project %s reports state %q after destroy, want %q",
					projectID, state, "deleted")
			}
			return nil
		})
	}
}

func TestAccProjectResource_basic(t *testing.T) {
	projectName := testProjectName("basic")
	updatedName := projectName + "-updated"
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkProjectDeleted(t, "ory_project.test"),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Name": projectName, "Environment": "dev"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project.test", "id"),
					resource.TestCheckResourceAttr("ory_project.test", "name", projectName),
					resource.TestCheckResourceAttr("ory_project.test", "environment", "dev"),
					resource.TestCheckResourceAttrSet("ory_project.test", "slug"),
					resource.TestCheckResourceAttr("ory_project.test", "state", "running"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update name
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Name": updatedName, "Environment": "dev"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project.test", "id"),
					resource.TestCheckResourceAttr("ory_project.test", "name", updatedName),
					resource.TestCheckResourceAttr("ory_project.test", "environment", "dev"),
				),
			},
		},
	})
}

// TestAccProjectResource_prodEnvironment tests creating a production project.
func TestAccProjectResource_prodEnvironment(t *testing.T) {
	projectName := testProjectName("prod")
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Name": projectName, "Environment": "prod"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project.test", "id"),
					resource.TestCheckResourceAttr("ory_project.test", "environment", "prod"),
				),
			},
		},
	})
}

// TestAccProjectResource_regions tests creating projects in different home regions.
func TestAccProjectResource_regions(t *testing.T) {
	regions := []string{"eu-central", "us-east", "us-west", "asia-northeast", "global"}

	for _, region := range regions {
		t.Run(region, func(t *testing.T) {
			projectName := testProjectName("region-" + region)
			acctest.RunTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
				Steps: []resource.TestStep{
					{
						Config: acctest.LoadTestConfig(t, "testdata/region.tf.tmpl", map[string]string{
							"Name":        projectName,
							"Environment": "dev",
							"HomeRegion":  region,
						}),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttrSet("ory_project.test", "id"),
							resource.TestCheckResourceAttr("ory_project.test", "name", projectName),
							resource.TestCheckResourceAttr("ory_project.test", "environment", "dev"),
							resource.TestCheckResourceAttr("ory_project.test", "home_region", region),
							resource.TestCheckResourceAttrSet("ory_project.test", "slug"),
							resource.TestCheckResourceAttr("ory_project.test", "state", "running"),
						),
					},
					// ImportState
					{
						ResourceName:      "ory_project.test",
						ImportState:       true,
						ImportStateVerify: true,
					},
				},
			})
		})
	}
}

// TestAccProjectResource_environmentInPlaceUpdate verifies that changing the
// environment tier on an existing project updates it in place — the project
// keeps its ID (and therefore all of its child resources) — instead of
// destroying and recreating it. Regression test for issue #289, where a
// one-line environment change forced a full project replacement.
func TestAccProjectResource_environmentInPlaceUpdate(t *testing.T) {
	projectName := testProjectName("env-inplace")
	var projectID string

	config := func(env string) string {
		return acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"Name": projectName, "Environment": env})
	}

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create a stage project and remember its ID.
			{
				Config: config("stage"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project.test", "environment", "stage"),
					captureAttr("ory_project.test", "id", &projectID),
				),
			},
			// Upgrade stage -> prod in place: environment changes, ID does not.
			{
				Config: config("prod"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project.test", "environment", "prod"),
					resource.TestCheckResourceAttrPtr("ory_project.test", "id", &projectID),
				),
			},
			// Downgrade prod -> dev in place: still the same project.
			{
				Config: config("dev"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_project.test", "environment", "dev"),
					resource.TestCheckResourceAttrPtr("ory_project.test", "id", &projectID),
				),
			},
			// Import round-trips cleanly after the in-place changes.
			{
				ResourceName:      "ory_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// captureAttr stores the value of a resource attribute so it can be compared
// across later test steps (e.g. to assert an ID did not change).
func captureAttr(resourceName, attr string, dest *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		*dest = rs.Primary.Attributes[attr]
		return nil
	}
}

// testProjectName generates a project name with the e2e prefix for hard deletion support.
func testProjectName(suffix string) string {
	return fmt.Sprintf("%s-tf-%s-%d", acctest.TestProjectPrefix, suffix, time.Now().UnixNano())
}
