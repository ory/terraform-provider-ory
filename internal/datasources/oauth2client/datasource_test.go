//go:build acceptance

package oauth2client_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccOAuth2ClientDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttrSet("data.ory_oauth2_client.test", "client_name"),
					resource.TestCheckResourceAttrSet("data.ory_oauth2_client.test", "scope"),
				),
			},
		},
	})
}
