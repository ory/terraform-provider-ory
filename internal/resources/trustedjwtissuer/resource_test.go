//go:build acceptance

package trustedjwtissuer_test

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccTrustedOAuth2JwtGrantIssuerResource_basic(t *testing.T) {
	expiresAt := time.Now().AddDate(5, 0, 0).UTC().Format(time.RFC3339)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Issuer":    "https://jwt-idp.example.com",
					"ExpiresAt": expiresAt,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_trusted_oauth2_jwt_grant_issuer.test", "id"),
					resource.TestCheckResourceAttr("ory_trusted_oauth2_jwt_grant_issuer.test", "issuer", "https://jwt-idp.example.com"),
					resource.TestCheckResourceAttr("ory_trusted_oauth2_jwt_grant_issuer.test", "scope.#", "2"),
					resource.TestCheckResourceAttrSet("ory_trusted_oauth2_jwt_grant_issuer.test", "created_at"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_trusted_oauth2_jwt_grant_issuer.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"jwk"},
			},
		},
	})
}
