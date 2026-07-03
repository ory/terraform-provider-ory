//go:build acceptance

package project_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

// TestAccProject_allowedProjectIDsBlocksDisallowed verifies the
// allowed_project_ids guardrail end to end: when the allowlist does not include
// the target project, the provider refuses the console operation (here a
// read-only GetProject via the data source) before any request reaches Ory.
func TestAccProject_allowedProjectIDsBlocksDisallowed(t *testing.T) {
	// A well-formed UUID that is deliberately not the test project.
	t.Setenv("ORY_ALLOWED_PROJECT_IDS", "00000000-0000-0000-0000-000000000000")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				// Terraform line-wraps rendered diagnostics, so match with flexible
				// whitespace between words that may be split across a newline.
				ExpectError: regexp.MustCompile(`is\s+not\s+in\s+the\s+allowed_project_ids\s+allowlist`),
			},
		},
	})
}

// TestAccProject_allowedProjectIDsAllowsListed verifies that when the target
// project is present in allowed_project_ids, operations proceed normally.
func TestAccProject_allowedProjectIDsAllowsListed(t *testing.T) {
	projectID := acctest.GetTestProjectID(t)
	t.Setenv("ORY_ALLOWED_PROJECT_IDS", projectID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ory_project.test", "id", projectID),
					resource.TestCheckResourceAttrSet("data.ory_project.test", "slug"),
				),
			},
		},
	})
}
