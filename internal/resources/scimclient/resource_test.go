//go:build acceptance

package scimclient_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

// testMapperURL is a base64:// Jsonnet mapper that copies the SCIM userName
// into the email trait. Ory validates the payload on write, so a real snippet
// is needed even in tests.
const testMapperURL = "base64://bG9jYWwgY2xhaW1zID0gc3RkLmV4dFZhcignY2xhaW1zJyk7CnsKICBpZGVudGl0eTogewogICAgdHJhaXRzOiB7CiAgICAgIGVtYWlsOiBjbGFpbXMuZW1haWwsCiAgICB9LAogIH0sCn0="

// SCIM clients belong to an organization, so they share the organization
// tests' gates: the B2B feature flag and a prod or stage project.
func testAccPreCheckSCIM(t *testing.T) {
	t.Helper()
	acctest.AccPreCheck(t)
	acctest.RequireB2BTests(t)

	env := os.Getenv("ORY_PROJECT_ENVIRONMENT")
	if env == "dev" || env == "" {
		t.Skip("SCIM client tests require ORY_PROJECT_ENVIRONMENT to be 'prod' or 'stage' (not 'dev')")
	}
}

// scimStatus calls the live SCIM endpoint of the test project with the given
// bearer secret and returns the HTTP status. The path takes the string
// client_id, not the row UUID. 200 means the secret is current, 401 means it
// is not, and 404 means the client is disabled or absent.
func scimStatus(t *testing.T, clientID, secret string) (int, error) {
	t.Helper()

	template := os.Getenv("ORY_PROJECT_API_URL")
	if template == "" {
		template = "https://%s.projects.oryapis.com"
	}
	endpoint := fmt.Sprintf(template, acctest.GetTestProject(t).Slug) + "/scim/" + clientID + "/v2/Users"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// checkSCIMStatus waits for the SCIM endpoint to answer want for the given
// secret. The check reads the running SCIM server, not Terraform state, so it
// proves a create, a rotation, or a disable took effect.
func checkSCIMStatus(t *testing.T, clientID, secretLabel, secret string, want int) resource.TestCheckFunc {
	t.Helper()
	return func(*terraform.State) error {
		return acctest.Eventually(func() error {
			got, err := scimStatus(t, clientID, secret)
			if err != nil {
				return err
			}
			if got != want {
				return fmt.Errorf("SCIM endpoint for %s answered %d with the %s secret, want %d", clientID, got, secretLabel, want)
			}
			return nil
		})
	}
}

// revisionClientIDs reads the SCIM client IDs straight from the normalized
// project revision.
func revisionClientIDs(t *testing.T) ([]string, error) {
	t.Helper()

	c, err := acctest.GetOryClient()
	if err != nil {
		return nil, fmt.Errorf("could not create client: %w", err)
	}
	revision, err := c.GetProjectNormalizedRevision(context.Background(), acctest.GetTestProjectID(t))
	if err != nil {
		return nil, fmt.Errorf("could not get normalized revision: %w", err)
	}
	clients, _ := revision["scim_clients"].([]interface{})
	ids := make([]string, 0, len(clients))
	for _, entry := range clients {
		m, _ := entry.(map[string]interface{})
		if id, ok := m["client_id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// checkClientsGone fails while any of the given client IDs is still stored.
func checkClientsGone(t *testing.T, clientIDs ...string) resource.TestCheckFunc {
	t.Helper()
	return func(*terraform.State) error {
		return acctest.Eventually(func() error {
			stored, err := revisionClientIDs(t)
			if err != nil {
				return err
			}
			for _, id := range clientIDs {
				if slices.Contains(stored, id) {
					return fmt.Errorf("SCIM client %s is still stored after destroy", id)
				}
			}
			return nil
		})
	}
}

func importStateSCIMClientID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		return rs.Primary.Attributes["project_id"] + "/" + rs.Primary.Attributes["client_id"], nil
	}
}

func basicConfig(t *testing.T, clientID, label, secret, state string) string {
	t.Helper()
	return acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
		"ClientID":  clientID,
		"Label":     label,
		"MapperURL": testMapperURL,
		"Secret":    secret,
		"State":     state,
	})
}

// TestAccSCIMClientResource_basic covers create, update, disable, secret
// rotation, import, and delete. Every secret change is verified against the
// live SCIM endpoint, because the API never returns the secret.
func TestAccSCIMClientResource_basic(t *testing.T) {
	const (
		clientID  = "tf-acc-scim-basic"
		secretOne = "tf-acc-scim-secret-one"
		secretTwo = "tf-acc-scim-secret-two"
	)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSCIM(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkClientsGone(t, clientID),
		Steps: []resource.TestStep{
			{
				Config: basicConfig(t, clientID, "Okta SCIM", secretOne, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_scim_client.test", "id", clientID),
					resource.TestCheckResourceAttr("ory_scim_client.test", "client_id", clientID),
					resource.TestCheckResourceAttr("ory_scim_client.test", "label", "Okta SCIM"),
					resource.TestCheckResourceAttr("ory_scim_client.test", "state", "enabled"),
					resource.TestCheckResourceAttr("ory_scim_client.test", "mapper_url", testMapperURL),
					resource.TestCheckResourceAttr("ory_scim_client.test", "authorization_header_secret", secretOne),
					resource.TestCheckResourceAttrSet("ory_scim_client.test", "project_id"),
					resource.TestCheckResourceAttrPair("ory_scim_client.test", "organization_id", "ory_organization.test", "id"),
					checkSCIMStatus(t, clientID, "configured", secretOne, http.StatusOK),
					checkSCIMStatus(t, clientID, "wrong", "not-the-secret", http.StatusUnauthorized),
				),
			},
			// Relabel and disable. The secret is unchanged, and a disabled
			// client answers 404.
			{
				Config: basicConfig(t, clientID, "Okta SCIM Disabled", secretOne, "disabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_scim_client.test", "label", "Okta SCIM Disabled"),
					resource.TestCheckResourceAttr("ory_scim_client.test", "state", "disabled"),
					checkSCIMStatus(t, clientID, "configured", secretOne, http.StatusNotFound),
				),
			},
			// Re-enable and rotate the secret.
			{
				Config: basicConfig(t, clientID, "Okta SCIM", secretTwo, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_scim_client.test", "state", "enabled"),
					resource.TestCheckResourceAttr("ory_scim_client.test", "authorization_header_secret", secretTwo),
					checkSCIMStatus(t, clientID, "new", secretTwo, http.StatusOK),
					checkSCIMStatus(t, clientID, "old", secretOne, http.StatusUnauthorized),
				),
			},
			// Re-applying the same config must produce no diff.
			{
				Config:   basicConfig(t, clientID, "Okta SCIM", secretTwo, "enabled"),
				PlanOnly: true,
			},
			// Import through the composite ID. The secret is never returned,
			// and mapper_url imports as the stored object-storage URL.
			{
				ResourceName:            "ory_scim_client.test",
				ImportState:             true,
				ImportStateIdFunc:       importStateSCIMClientID("ory_scim_client.test"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"authorization_header_secret", "mapper_url"},
			},
		},
	})
}

// TestAccSCIMClientResource_writeOnlySecret verifies that the write-only
// secret reaches the SCIM server but never Terraform state, and that bumping
// the version trigger rotates it. Write-only arguments need Terraform 1.11+.
func TestAccSCIMClientResource_writeOnlySecret(t *testing.T) {
	const (
		clientID  = "tf-acc-scim-wo"
		secretOne = "tf-acc-scim-wo-secret-one"
		secretTwo = "tf-acc-scim-wo-secret-two"
	)
	config := func(secret, version string) string {
		return acctest.LoadTestConfig(t, "testdata/write_only_secret.tf.tmpl", map[string]string{
			"ClientID":  clientID,
			"MapperURL": testMapperURL,
			"Secret":    secret,
			"Version":   version,
		})
	}

	acctest.RunTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheckSCIM(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkClientsGone(t, clientID),
		Steps: []resource.TestStep{
			{
				Config: config(secretOne, "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_scim_client.wo", "client_id", clientID),
					resource.TestCheckResourceAttr("ory_scim_client.wo", "authorization_header_secret_wo_version", "1"),
					resource.TestCheckResourceAttr("ory_scim_client.wo", "state", "enabled"),
					checkSCIMStatus(t, clientID, "write-only", secretOne, http.StatusOK),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_scim_client.wo", tfjsonpath.New("authorization_header_secret_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_scim_client.wo", tfjsonpath.New("authorization_header_secret"), knownvalue.Null()),
				},
			},
			// Rotate by bumping the version trigger.
			{
				Config: config(secretTwo, "2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_scim_client.wo", "authorization_header_secret_wo_version", "2"),
					checkSCIMStatus(t, clientID, "new write-only", secretTwo, http.StatusOK),
					checkSCIMStatus(t, clientID, "old write-only", secretOne, http.StatusUnauthorized),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_scim_client.wo", tfjsonpath.New("authorization_header_secret_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_scim_client.wo", tfjsonpath.New("authorization_header_secret"), knownvalue.Null()),
				},
			},
			{
				Config:   config(secretTwo, "2"),
				PlanOnly: true,
			},
			{
				ResourceName:      "ory_scim_client.wo",
				ImportState:       true,
				ImportStateIdFunc: importStateSCIMClientID("ory_scim_client.wo"),
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"authorization_header_secret",
					"authorization_header_secret_wo",
					"authorization_header_secret_wo_version",
					"mapper_url",
				},
			},
		},
	})
}

// TestAccSCIMClientResource_concurrentCreate verifies that several SCIM
// clients created in one apply all persist. Every write is a read-modify-write
// of the same revision array, so without serialization the last write wins
// and the others are lost. See issue #165 for the same race on providers.
func TestAccSCIMClientResource_concurrentCreate(t *testing.T) {
	clientIDs := []string{"tf-acc-scim-concurrent-one", "tf-acc-scim-concurrent-two", "tf-acc-scim-concurrent-three"}
	config := acctest.LoadTestConfig(t, "testdata/concurrent_create.tf.tmpl", map[string]string{
		"MapperURL": testMapperURL,
	})

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSCIM(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkClientsGone(t, clientIDs...),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_scim_client.one", "client_id", clientIDs[0]),
					resource.TestCheckResourceAttr("ory_scim_client.two", "client_id", clientIDs[1]),
					resource.TestCheckResourceAttr("ory_scim_client.three", "client_id", clientIDs[2]),
					func(*terraform.State) error {
						return acctest.Eventually(func() error {
							stored, err := revisionClientIDs(t)
							if err != nil {
								return err
							}
							for _, id := range clientIDs {
								if !slices.Contains(stored, id) {
									return fmt.Errorf("SCIM client %s is missing server-side, stored: %v", id, stored)
								}
							}
							return nil
						})
					},
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// TestAccSCIMClientResource_organizationDeletedOutOfBand verifies the cascade
// the API applies: deleting an organization deletes its SCIM clients. Both
// resources must drop out of state on refresh instead of failing the plan,
// and the next apply must recreate them.
func TestAccSCIMClientResource_organizationDeletedOutOfBand(t *testing.T) {
	const (
		clientID = "tf-acc-scim-org-cascade"
		secret   = "tf-acc-scim-cascade-secret"
	)
	var firstOrgID string

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSCIM(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkClientsGone(t, clientID),
		Steps: []resource.TestStep{
			{
				Config: basicConfig(t, clientID, "Cascade", secret, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["ory_organization.test"]
						if !ok {
							return fmt.Errorf("resource not found: ory_organization.test")
						}
						firstOrgID = rs.Primary.ID
						return nil
					},
					checkSCIMStatus(t, clientID, "configured", secret, http.StatusOK),
				),
			},
			{
				PreConfig: func() {
					c, err := acctest.GetOryClient()
					require.NoError(t, err, "could not create client")
					require.NoError(t, c.DeleteOrganization(context.Background(), acctest.GetTestProjectID(t), firstOrgID),
						"could not delete the organization out of band")
					require.NoError(t, checkClientsGone(t, clientID)(nil),
						"deleting the organization must delete its SCIM client server-side")
				},
				Config: basicConfig(t, clientID, "Cascade", secret, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["ory_organization.test"]
						if !ok {
							return fmt.Errorf("resource not found: ory_organization.test")
						}
						if rs.Primary.ID == firstOrgID {
							return fmt.Errorf("the organization was not recreated after the out-of-band delete")
						}
						return nil
					},
					resource.TestCheckResourceAttrPair("ory_scim_client.test", "organization_id", "ory_organization.test", "id"),
					checkSCIMStatus(t, clientID, "configured", secret, http.StatusOK),
				),
			},
		},
	})
}
