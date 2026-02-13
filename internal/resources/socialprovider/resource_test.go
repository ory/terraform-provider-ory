//go:build acceptance

package socialprovider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccSocialProviderResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf", nil),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_id", "test-google"),
					resource.TestCheckResourceAttr("ory_social_provider.test", "provider_type", "google"),
				),
			},
			// ImportState using provider_id
			{
				ResourceName:      "ory_social_provider.test",
				ImportState:       true,
				ImportStateId:     "test-google",
				ImportStateVerify: true,
				// client_secret is sensitive and not returned by API
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}
