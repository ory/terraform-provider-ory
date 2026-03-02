//go:build acceptance

package socialprovider_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
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
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil),
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

// generateTestPrivateKey creates a fresh EC P-256 private key in PEM format
// at test runtime so no key material is committed to source code.
func generateTestPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test private key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal test private key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestAccSocialProviderResource_apple(t *testing.T) {
	tmplData := struct{ PrivateKey string }{PrivateKey: generateTestPrivateKey(t)}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AccPreCheck(t)
			acctest.RequireSocialProviderTests(t)
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create Apple provider with Apple-specific fields
			{
				Config: acctest.LoadTestConfig(t, "testdata/apple_basic.tf.tmpl", tmplData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_social_provider.test_apple", "id"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "provider_id", "test-apple"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "provider_type", "apple"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "client_id", "com.example.auth.service"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_team_id", "TESTTEAMID"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_private_key_id", "TESTKEYID1"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_social_provider.test_apple",
				ImportState:       true,
				ImportStateId:     "test-apple",
				ImportStateVerify: true,
				// Sensitive fields not returned by API
				ImportStateVerifyIgnore: []string{"client_secret", "apple_private_key"},
			},
			// Update Apple provider fields
			{
				Config: acctest.LoadTestConfig(t, "testdata/apple_updated.tf.tmpl", tmplData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "client_id", "com.example.auth.service.updated"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_team_id", "UPDATEDTID"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "apple_private_key_id", "UPDATEDKID"),
					resource.TestCheckResourceAttr("ory_social_provider.test_apple", "scope.#", "3"),
				),
			},
		},
	})
}
