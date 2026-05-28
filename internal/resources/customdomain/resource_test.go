//go:build acceptance

package customdomain_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func importStateCustomDomainID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_custom_domain.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_custom_domain.test")
	}
	projectID := rs.Primary.Attributes["project_id"]
	domainID := rs.Primary.ID
	return fmt.Sprintf("%s/%s", projectID, domainID), nil
}

// cleanupStaleCustomDomains removes any existing custom domains from the test
// project. This prevents "Duplicate custom hostname found (1406)" errors when a
// previous test run failed before its destroy step could clean up.
func cleanupStaleCustomDomains(t *testing.T) {
	t.Helper()

	oryClient, err := acctest.GetOryClient()
	require.NoError(t, err, "failed to create Ory client for custom domain cleanup")

	projectID := acctest.GetTestProjectID(t)
	ctx := context.Background()

	domains, err := oryClient.ListCustomDomains(ctx, projectID)
	if err != nil {
		t.Logf("warning: could not list custom domains for cleanup: %s", err)
		return
	}

	for _, d := range domains {
		t.Logf("cleaning up stale custom domain: id=%s", d.GetId())
		require.NoError(t, oryClient.DeleteCustomDomain(ctx, projectID, d.GetId()),
			"failed to delete stale custom domain %s", d.GetId())
	}
}

func testAccPreCheckCustomDomain(t *testing.T) {
	acctest.AccPreCheck(t)
	if os.Getenv("ORY_CUSTOM_DOMAIN_HOSTNAME") == "" {
		t.Skip("ORY_CUSTOM_DOMAIN_HOSTNAME must be set for custom domain tests (e.g., test.example-e2e.orycname.dev)")
	}
	cleanupStaleCustomDomains(t)
}

// TestAccCustomDomainResource_basic tests the full CRUD lifecycle of a custom domain.
//
// Required environment variables:
//
//	ORY_CUSTOM_DOMAIN_HOSTNAME      - Hostname to use (e.g., test.example-e2e.orycname.dev)
//
// Optional environment variables:
//
//	ORY_CUSTOM_DOMAIN_COOKIE_DOMAIN - Cookie domain (default: example.com)
func TestAccCustomDomainResource_basic(t *testing.T) {
	hostname := os.Getenv("ORY_CUSTOM_DOMAIN_HOSTNAME")
	cookieDomain := os.Getenv("ORY_CUSTOM_DOMAIN_COOKIE_DOMAIN")
	if cookieDomain == "" {
		cookieDomain = "example.com"
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckCustomDomain(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Hostname":     hostname,
					"CookieDomain": cookieDomain,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_custom_domain.test", "id"),
					resource.TestCheckResourceAttr("ory_custom_domain.test", "hostname", hostname),
					resource.TestCheckResourceAttr("ory_custom_domain.test", "cookie_domain", cookieDomain),
					resource.TestCheckResourceAttrSet("ory_custom_domain.test", "verification_status"),
					resource.TestCheckResourceAttrSet("ory_custom_domain.test", "created_at"),
					resource.TestCheckResourceAttrSet("ory_custom_domain.test", "updated_at"),
				),
			},
			// Update - add CORS and custom UI base URL
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", map[string]string{
					"Hostname":     hostname,
					"CookieDomain": cookieDomain,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_custom_domain.test", "hostname", hostname),
					resource.TestCheckResourceAttr("ory_custom_domain.test", "cors_enabled", "true"),
					resource.TestCheckResourceAttr("ory_custom_domain.test", "cors_allowed_origins.0", "https://app.example.com"),
					resource.TestCheckResourceAttr("ory_custom_domain.test", "custom_ui_base_url", "https://"+hostname),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_custom_domain.test",
				ImportState:             true,
				ImportStateIdFunc:       importStateCustomDomainID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"created_at", "updated_at", "verification_status", "verification_errors", "ssl_status"},
			},
		},
	})
}
