package jwk_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-orynetwork/internal/acctest"
)

func TestAccJWKResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccJWKResourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "id"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "set_id", "tf-test-jwks"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "key_id", "tf-test-key"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "algorithm", "RS256"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "use", "sig"),
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "keys"),
				),
			},
		},
	})
}

func testAccJWKResourceConfig() string {
	return `
provider "ory" {}

resource "ory_json_web_key_set" "test" {
  set_id    = "tf-test-jwks"
  key_id    = "tf-test-key"
  algorithm = "RS256"
  use       = "sig"
}
`
}
