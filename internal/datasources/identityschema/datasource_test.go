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
