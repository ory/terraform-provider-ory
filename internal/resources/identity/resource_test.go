//go:build acceptance

package identity_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccIdentityResource_basic(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccIdentityResourceConfig("test-basic-user"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "schema_id", "preset://username"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_identity.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Password is write-only, not returned on read
				ImportStateVerifyIgnore: []string{"password"},
			},
			// Update
			{
				Config: testAccIdentityResourceConfig("test-updated-user"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
				),
			},
		},
	})
}

func TestAccIdentityResource_withMetadata(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityResourceConfigWithMetadata("test-metadata-user"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "active"),
					resource.TestCheckResourceAttrSet("ory_identity.test", "metadata_public"),
				),
			},
		},
	})
}

func TestAccIdentityResource_inactive(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccIdentityResourceConfigInactive("test-inactive-user"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_identity.test", "id"),
					resource.TestCheckResourceAttr("ory_identity.test", "state", "inactive"),
				),
			},
		},
	})
}

func testAccIdentityResourceConfig(username string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_identity" "test" {
  schema_id = "preset://username"

  traits = jsonencode({
    username = %[1]q
  })

  state = "active"
}
`, username)
}

func testAccIdentityResourceConfigWithMetadata(username string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_identity" "test" {
  schema_id = "preset://username"

  traits = jsonencode({
    username = %[1]q
  })

  state = "active"

  metadata_public = jsonencode({
    role        = "admin"
    created_by  = "terraform"
  })
}
`, username)
}

func testAccIdentityResourceConfigInactive(username string) string {
	return fmt.Sprintf(`
provider "ory" {}

resource "ory_identity" "test" {
  schema_id = "preset://username"

  traits = jsonencode({
    username = %[1]q
  })

  state = "inactive"
}
`, username)
}
