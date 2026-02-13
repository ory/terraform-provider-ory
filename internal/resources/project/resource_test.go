//go:build acceptance

package project_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

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
func TestAccProjectResource_basic(t *testing.T) {
	projectName := testProjectName("basic")
	updatedName := projectName + "-updated"
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
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

// testProjectName generates a project name with the e2e prefix for hard deletion support.
// The prefix is read from ORY_TEST_PROJECT_PREFIX env var (set by scripts/run-acceptance-tests.sh).
func testProjectName(suffix string) string {
	prefix := os.Getenv("ORY_TEST_PROJECT_PREFIX")
	if prefix != "" {
		return fmt.Sprintf("%s-tf-%s-%d", prefix, suffix, time.Now().UnixNano())
	}
	return fmt.Sprintf("tf-acc-test-%s-%d", suffix, time.Now().UnixNano())
}
