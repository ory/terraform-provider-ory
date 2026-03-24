//go:build acceptance

package identityschema_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// testCheckResourceAttrNotEqual verifies that two resource attributes have different values.
func testCheckResourceAttrNotEqual(res1, attr1, res2, attr2 string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		r1, ok := s.RootModule().Resources[res1]
		if !ok {
			return fmt.Errorf("resource %s not found", res1)
		}
		r2, ok := s.RootModule().Resources[res2]
		if !ok {
			return fmt.Errorf("resource %s not found", res2)
		}
		v1, ok1 := r1.Primary.Attributes[attr1]
		if !ok1 {
			return fmt.Errorf("attribute %s not found on resource %s", attr1, res1)
		}
		v2, ok2 := r2.Primary.Attributes[attr2]
		if !ok2 {
			return fmt.Errorf("attribute %s not found on resource %s", attr2, res2)
		}
		if v1 == v2 {
			return fmt.Errorf("%s.%s (%q) should not equal %s.%s (%q)", res1, attr1, v1, res2, attr2, v2)
		}
		return nil
	}
}

func TestAccIdentitySchemaResource_basic(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-schema-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"SchemaID": schemaID, "AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity_schema.test", "id"),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "schema_id", schemaID),
				),
			},
		},
	})
}

func TestAccIdentitySchemaResource_uniqueContent(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID1 := fmt.Sprintf("tf-test-unique1-%d", suffix)
	schemaID2 := fmt.Sprintf("tf-test-unique2-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create two schemas with different content — they should get different IDs
			{
				Config: acctest.LoadTestConfig(t, "testdata/two_schemas.tf.tmpl", map[string]string{
					"SchemaID1": schemaID1,
					"SchemaID2": schemaID2,
					"AppURL":    testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity_schema.first", "id"),
					resource.TestCheckResourceAttrSet("ory_identity_schema.second", "id"),
					testCheckResourceAttrNotEqual("ory_identity_schema.first", "id", "ory_identity_schema.second", "id"),
				),
			},
		},
	})
}

func TestAccIdentitySchemaResource_setDefault(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-default-%d", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create without set_default
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"SchemaID": schemaID, "AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity_schema.test", "id"),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "schema_id", schemaID),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "set_default", "false"),
				),
			},
			// Update to set as default
			{
				Config: acctest.LoadTestConfig(t, "testdata/set_default.tf.tmpl", map[string]string{"SchemaID": schemaID, "AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity_schema.test", "id"),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "schema_id", schemaID),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "set_default", "true"),
				),
			},
		},
	})
}
