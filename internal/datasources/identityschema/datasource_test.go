//go:build acceptance

package identityschema_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestAccIdentitySchemaDataSource_basic(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create a schema via resource, then look it up via data source using the API-assigned ID
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"SchemaID": schemaID,
					"AppURL":   testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The data source's id matches the resource's API-assigned id (may be a hash)
					resource.TestCheckResourceAttrPair(
						"data.ory_identity_schema.test", "id",
						"ory_identity_schema.test", "id",
					),
					resource.TestCheckResourceAttrSet("data.ory_identity_schema.test", "schema"),
				),
			},
		},
	})
}

func TestAccIdentitySchemaDataSource_viaProjectID(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create a schema, then look it up via data source using project_id (console API path)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_project_id.tf.tmpl", map[string]string{
					"SchemaID":  schemaID,
					"AppURL":    testutil.ExampleAppURL,
					"ProjectID": acctest.GetTestProjectID(t),
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.ory_identity_schema.test", "id",
						"ory_identity_schema.test", "id",
					),
					resource.TestCheckResourceAttrSet("data.ory_identity_schema.test", "schema"),
					resource.TestCheckResourceAttr("data.ory_identity_schema.test", "project_id", acctest.GetTestProjectID(t)),
				),
			},
		},
	})
}

// TestAccIdentitySchemaDataSource_bootstrapProject tests the bootstrap scenario
// where a brand-new project is created and an existing workspace-level schema is
// looked up via project_id. This exercises the code path that creates a temporary
// API key to call the Kratos API when no project credentials are configured.
func TestAccIdentitySchemaDataSource_bootstrapProject(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-bootstrap-%d", suffix)
	projectName := fmt.Sprintf("%s-bootstrap-%d", acctest.TestProjectPrefix, suffix)

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
					// The data source found the schema via the bootstrap project
					resource.TestCheckResourceAttrPair(
						"data.ory_identity_schema.via_bootstrap", "id",
						"ory_identity_schema.seed", "id",
					),
					// Schema content is not empty
					resource.TestCheckResourceAttrSet("data.ory_identity_schema.via_bootstrap", "schema"),
					// project_id is set on the data source
					resource.TestCheckResourceAttrSet("data.ory_identity_schema.via_bootstrap", "project_id"),
				),
			},
		},
	})
}

func TestAccIdentitySchemaDataSource_viaProjectIDContentMatch(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-content-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create a schema, then look it up via both API paths and verify
			// the schema content matches (not empty "{}").
			// This catches the bug where the console API returns empty schema
			// bodies for schemas whose URLs have been transformed to HTTPS.
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_project_id_content_check.tf.tmpl", map[string]string{
					"SchemaID":  schemaID,
					"AppURL":    testutil.ExampleAppURL,
					"ProjectID": acctest.GetTestProjectID(t),
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Both data sources find the same schema
					resource.TestCheckResourceAttrPair(
						"data.ory_identity_schema.via_project_id", "id",
						"data.ory_identity_schema.via_kratos_api", "id",
					),
					// Both return the same schema content
					resource.TestCheckResourceAttrPair(
						"data.ory_identity_schema.via_project_id", "schema",
						"data.ory_identity_schema.via_kratos_api", "schema",
					),
					// Schema content is not empty
					resource.TestCheckResourceAttrSet("data.ory_identity_schema.via_project_id", "schema"),
					resource.TestCheckResourceAttrSet("data.ory_identity_schema.via_kratos_api", "schema"),
				),
			},
		},
	})
}
