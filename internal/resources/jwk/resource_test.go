//go:build acceptance

package jwk_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func importStateJWKID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_json_web_key_set.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_json_web_key_set.test")
	}
	projectID := rs.Primary.Attributes["project_id"]
	if projectID == "" {
		return "", fmt.Errorf("project_id is empty in state; cannot build composite import ID")
	}
	setID := rs.Primary.Attributes["set_id"]
	if setID == "" {
		return "", fmt.Errorf("set_id is empty in state; cannot build composite import ID")
	}
	return fmt.Sprintf("%s/%s", projectID, setID), nil
}

// testCheckKeysContainPrivateMaterial verifies that the keys JSON output
// contains private key fields for the given algorithm.
func testCheckKeysContainPrivateMaterial(resourceName, algorithm string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		keysJSON := rs.Primary.Attributes["keys"]
		if keysJSON == "" {
			return fmt.Errorf("keys attribute is empty")
		}

		var jwks struct {
			Keys []map[string]interface{} `json:"keys"`
		}
		if err := json.Unmarshal([]byte(keysJSON), &jwks); err != nil {
			return fmt.Errorf("failed to parse keys JSON: %w", err)
		}
		if len(jwks.Keys) == 0 {
			return fmt.Errorf("keys JSON contains no keys")
		}

		key := jwks.Keys[0]

		// Check for private key fields based on algorithm type
		var requiredFields []string
		switch algorithm {
		case "RS256":
			requiredFields = []string{"d", "p", "q", "dp", "dq", "qi"}
		case "ES256", "ES512":
			requiredFields = []string{"d"}
		case "HS256", "HS512":
			requiredFields = []string{"k"}
		}

		for _, field := range requiredFields {
			if _, exists := key[field]; !exists {
				return fmt.Errorf("keys output missing private key field %q for algorithm %s", field, algorithm)
			}
		}
		return nil
	}
}

func TestAccJWKResource_basic(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		t.Skip("ORY_PROJECT_ID must be set for JWK acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"ProjectID": projectID,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "id"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "project_id", projectID),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "set_id", "tf-test-jwks"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "key_id", "tf-test-key"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "algorithm", "RS256"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "use", "sig"),
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "keys"),
					testCheckKeysContainPrivateMaterial("ory_json_web_key_set.test", "RS256"),
				),
			},
			// ImportState using composite ID: project_id/set_id
			{
				ResourceName:      "ory_json_web_key_set.test",
				ImportState:       true,
				ImportStateIdFunc: importStateJWKID,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccJWKResource_ec_key(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		t.Skip("ORY_PROJECT_ID must be set for JWK acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/ec_key.tf.tmpl", map[string]string{
					"ProjectID": projectID,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "id"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "algorithm", "ES256"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "use", "sig"),
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "keys"),
					testCheckKeysContainPrivateMaterial("ory_json_web_key_set.test", "ES256"),
				),
			},
			{
				ResourceName:      "ory_json_web_key_set.test",
				ImportState:       true,
				ImportStateIdFunc: importStateJWKID,
				ImportStateVerify: true,
			},
		},
	})
}
