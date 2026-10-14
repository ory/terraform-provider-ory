//go:build acceptance

package identityschema_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ory "github.com/ory/client-go"

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

// captureSchemaAPIID records the API-side id of the schema this test created, so
// the destroy check can require that exact schema rather than accepting any
// non-empty list. The API rewrites schema_id to a content hash, so the id has to
// be read from state after apply instead of assumed.
func captureSchemaAPIID(resourceName string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("resource %s has an empty id", resourceName)
		}
		*into = rs.Primary.ID
		return nil
	}
}

// checkSchemaSurvivesDestroy asserts the schema this test created is still
// registered on the project after destroy. ory_identity_schema has an intentional
// no-op Delete, because Ory Network does not support deleting identity schemas
// (ory/network#262), and the resource warns as much. Pinning that keeps two
// failures visible: a Delete that starts really deleting, which would break
// identities still using the schema, and one that drops the schema from the
// project's list.
//
// apiID is filled in by captureSchemaAPIID during the apply step. Asserting that
// specific id, rather than a non-empty list, is what keeps this from passing on
// the preset schemas every project already has. See issue #333.
func checkSchemaSurvivesDestroy(t *testing.T, apiID *string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if *apiID == "" {
			return fmt.Errorf("no schema id was captured during the apply step")
		}
		return acctest.Eventually(func() error {
			value, ok, err := acctest.ProjectConfigValue(t, acctest.GetTestProject(t).ID,
				"/services/identity/config/identity/schemas")
			if err != nil {
				return fmt.Errorf("could not read project after destroy: %w", err)
			}
			if !ok {
				return fmt.Errorf("destroy removed the identity schemas list entirely")
			}
			schemas, _ := value.([]interface{})
			present := make([]string, 0, len(schemas))
			for _, s := range schemas {
				sm, _ := s.(map[string]interface{})
				id, _ := sm["id"].(string)
				if id == *apiID {
					return nil
				}
				present = append(present, id)
			}
			return fmt.Errorf("destroy removed schema %s from the project, remaining schemas: %v",
				*apiID, present)
		})
	}
}

func TestAccIdentitySchemaResource_basic(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-schema-%d", suffix)

	// Filled in by captureSchemaAPIID during the apply, read by CheckDestroy.
	var schemaAPIID string

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSchemaTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkSchemaSurvivesDestroy(t, &schemaAPIID),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{"SchemaID": schemaID, "AppURL": testutil.ExampleAppURL}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity_schema.test", "id"),
					resource.TestCheckResourceAttr("ory_identity_schema.test", "schema_id", schemaID),
					captureSchemaAPIID("ory_identity_schema.test", &schemaAPIID),
				),
			},
		},
	})
}

// TestAccIdentitySchemaResource_hashIDResolution verifies that Create resolves the
// canonical hash-based ID (not the user-provided schema_id) and that Read can find
// the schema after the API transforms both the ID and URL. This exercises the
// content-based fallback in Read that was added to handle the API transformation.
func TestAccIdentitySchemaResource_hashIDResolution(t *testing.T) {
	suffix := time.Now().UnixNano()
	schemaID := fmt.Sprintf("tf-test-hash-%d", suffix)

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
					// Verify the stored ID is the canonical hash, not the user-provided schema_id.
					// This proves Create resolved the hash and Read can find the schema after
					// the API transforms the ID and URL.
					resource.TestCheckResourceAttrWith("ory_identity_schema.test", "id", func(value string) error {
						if value == schemaID {
							return fmt.Errorf("expected canonical hash ID, but got user-provided schema_id %q", value)
						}
						return nil
					}),
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

	// Reset the default schema to preset://username after the test so that
	// cleanup of tf-test-* schemas doesn't leave the project with a
	// dangling default_schema_id reference.
	t.Cleanup(func() {
		resetDefaultSchemaToPreset(t)
	})

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

// resetDefaultSchemaToPreset resets the project's default_schema_id back to
// preset://username. This prevents cleanup of tf-test-* schemas from
// leaving the project with a broken default reference.
func resetDefaultSchemaToPreset(t *testing.T) {
	t.Helper()

	c, err := acctest.GetOryClient()
	if err != nil {
		t.Logf("Warning: could not create client for default schema reset: %v", err)
		return
	}

	projectID := acctest.GetTestProjectID(t)
	patches := []ory.JsonPatch{
		{
			Op:    "replace",
			Path:  "/services/identity/config/identity/default_schema_id",
			Value: "preset://username",
		},
	}

	if _, err := c.PatchProject(context.Background(), projectID, patches); err != nil {
		t.Logf("Warning: failed to reset default schema to preset://username: %v", err)
		return
	}

	t.Log("Reset default_schema_id to preset://username")
}
