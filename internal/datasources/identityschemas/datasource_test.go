//go:build acceptance

package identityschemas_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccIdentitySchemasDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_identity_schemas.test", "schemas.#"),
				),
			},
		},
	})
}

// TestAccIdentitySchemasDataSource_bootstrapProject tests the bootstrap scenario
// where a brand-new project is created and workspace-level schemas are listed
// via project_id. This exercises the code path that creates a temporary API key
// to call the Kratos API when no project credentials are configured.
func TestAccIdentitySchemasDataSource_bootstrapProject(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-list-bootstrap-%d", suffix)
	projectName := fmt.Sprintf("%s-bootstrap-list-%d", acctest.TestProjectPrefix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
			acctest.RequireProjectTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/bootstrap_project.tf.tmpl", map[string]string{
					"SchemaID":    schemaID,
					"AppURL":      testutil.ExampleAppURL,
					"ProjectName": projectName,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The data source returns schemas (should include workspace-scoped ones)
					resource.TestCheckResourceAttrSet("data.ory_identity_schemas.via_bootstrap", "schemas.#"),
					// project_id is set
					resource.TestCheckResourceAttrSet("data.ory_identity_schemas.via_bootstrap", "project_id"),
				),
			},
		},
	})
}

func TestAccIdentitySchemasDataSource_viaProjectID(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-list-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_project_id.tf.tmpl", map[string]string{
					"SchemaID":  schemaID,
					"AppURL":    testutil.ExampleAppURL,
					"ProjectID": acctest.GetTestProjectID(t),
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_identity_schemas.test", "schemas.#"),
					resource.TestCheckResourceAttr("data.ory_identity_schemas.test", "project_id", acctest.GetTestProjectID(t)),
				),
			},
		},
	})
}
