//go:build acceptance

package oidcdynamicclient_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccOIDCDynamicClientResource_basic(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Name": "Test Dynamic Client",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oidc_dynamic_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oidc_dynamic_client.test", "client_name", "Test Dynamic Client"),
					resource.TestCheckResourceAttr("ory_oidc_dynamic_client.test", "scope", "openid"),
					resource.TestCheckResourceAttrSet("ory_oidc_dynamic_client.test", "client_secret"),
					resource.TestCheckResourceAttrSet("ory_oidc_dynamic_client.test", "registration_access_token"),
					resource.TestCheckResourceAttrSet("ory_oidc_dynamic_client.test", "registration_client_uri"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_oidc_dynamic_client.test",
				ImportState:       true,
				ImportStateVerify: true,
				// These are only returned on create, not on read via the admin API
				ImportStateVerifyIgnore: []string{"client_secret", "registration_access_token", "registration_client_uri"},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", map[string]string{
					"Name": "Test Dynamic Client Updated",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oidc_dynamic_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oidc_dynamic_client.test", "client_name", "Test Dynamic Client Updated"),
					resource.TestCheckResourceAttr("ory_oidc_dynamic_client.test", "scope", "openid offline_access"),
				),
			},
		},
	})
}
