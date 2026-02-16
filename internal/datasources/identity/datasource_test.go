//go:build acceptance

package identity_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccIdentityDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_identity.test", "id"),
					resource.TestCheckResourceAttrSet("data.ory_identity.test", "schema_id"),
					resource.TestCheckResourceAttrSet("data.ory_identity.test", "state"),
					resource.TestCheckResourceAttrSet("data.ory_identity.test", "traits"),
				),
			},
		},
	})
}
