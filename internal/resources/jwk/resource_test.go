//go:build acceptance

package jwk_test

import (
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
	setID := rs.Primary.Attributes["set_id"]
	return fmt.Sprintf("%s/%s", projectID, setID), nil
}

func TestAccJWKResource_basic(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")

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
