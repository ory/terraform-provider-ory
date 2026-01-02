package identityschema_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
	"github.com/ory/terraform-provider-orynetwork/internal/testutil"
)

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
				Config: testAccIdentitySchemaResourceConfig(schemaID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity_schema.test", "id"),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "schema_id", schemaID),
				),
			},
		},
	})
}

func testAccIdentitySchemaResourceConfig(schemaID string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_identity_schema" "test" {
  schema_id = %[1]q
  schema    = jsonencode({
    "$id": "%[2]s/%[1]s.json",
    "$schema": "http://json-schema.org/draft-07/schema#",
    "title": "Test Schema %[1]s",
    "type": "object",
    "properties": {
      "traits": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "format": "email",
            "title": "Email",
            "ory.sh/kratos": {
              "credentials": {
                "password": {"identifier": true}
              },
              "verification": {"via": "email"},
              "recovery": {"via": "email"}
            }
          }
        },
        "required": ["email"]
      }
    }
  })
}
`, schemaID, testutil.ExampleAppURL)
}
