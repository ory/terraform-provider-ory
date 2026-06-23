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

// TestAccIdentitySchemaDataSource_bootstrapWorkspaceLookup reproduces issue
// #138: a workspace-scoped schema is looked up with only a workspace API key
// (project credentials explicitly disabled and no project_id), so the lookup
// must succeed via the workspace endpoint rather than the project config or
// Kratos.
func TestAccIdentitySchemaDataSource_bootstrapWorkspaceLookup(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-ds-bootstrap-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/bootstrap_workspace_lookup.tf.tmpl", map[string]string{
					"SchemaID": schemaID,
					"AppURL":   testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
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
