//go:build acceptance

package projectapikey_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccProjectAPIKeyResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_project_api_key.test", "id"),
					resource.TestCheckResourceAttr("ory_project_api_key.test", "name", "tf-test-key"),
					resource.TestCheckResourceAttrSet("ory_project_api_key.test", "value"),
					resource.TestCheckResourceAttrSet("ory_project_api_key.test", "owner_id"),
				),
			},
			// Import using composite ID: project_id/key_id
			{
				ResourceName:      "ory_project_api_key.test",
				ImportState:       true,
				ImportStateIdFunc: importStateProjectAPIKeyID,
				ImportStateVerify: true,
				// value is only returned on creation
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}

func importStateProjectAPIKeyID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_project_api_key.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_project_api_key.test")
	}
	projectID := rs.Primary.Attributes["project_id"]
	keyID := rs.Primary.ID
	return fmt.Sprintf("%s/%s", projectID, keyID), nil
}
