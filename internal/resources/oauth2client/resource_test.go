//go:build acceptance

package oauth2client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// checkOAuth2ClientServerJWKSKid reads the OAuth2 client back from the Ory API
// and asserts its JWKS contains a key with the expected kid. Because jwks is
// supplied write-only (and therefore absent from state), this is the only way to
// confirm the write-only JWKS — and its rotation via jwks_wo_version — actually
// reached the server, so a no-op Update path cannot pass silently.
func checkOAuth2ClientServerJWKSKid(resourceName, expectedKid string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found in state: %s", resourceName)
		}
		clientID := rs.Primary.Attributes["client_id"]
		if clientID == "" {
			return fmt.Errorf("client_id is empty in state for %s", resourceName)
		}
		c, err := acctest.GetOryClient()
		if err != nil {
			return err
		}
		oauthClient, err := c.GetOAuth2Client(context.Background(), clientID)
		if err != nil {
			return fmt.Errorf("failed to read OAuth2 client %s: %w", clientID, err)
		}
		if oauthClient.Jwks == nil || len(oauthClient.Jwks.Keys) == 0 {
			return fmt.Errorf("expected JWKS with keys on the server for %s, got none", clientID)
		}
		for _, k := range oauthClient.Jwks.Keys {
			if k.Kid == expectedKid {
				return nil
			}
		}
		return fmt.Errorf("expected server JWKS for %s to contain kid %q", clientID, expectedKid)
	}
}

// TestAccOAuth2ClientResource_writeOnlyJWKS verifies that an inline JWKS supplied
// via jwks_wo is sent to the API but never written to Terraform state, and that
// bumping jwks_wo_version triggers an update. Write-only arguments require
// Terraform 1.11+.
func TestAccOAuth2ClientResource_writeOnlyJWKS(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck: func() { acctest.AccPreCheck(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_jwks.tf.tmpl", map[string]string{
					"Name":    "Test Client with write-only JWKS",
					"Version": "1",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.wo", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.wo", "token_endpoint_auth_method", "private_key_jwt"),
					resource.TestCheckResourceAttr("ory_oauth2_client.wo", "jwks_wo_version", "1"),
					// Confirm the write-only JWKS reached the server.
					checkOAuth2ClientServerJWKSKid("ory_oauth2_client.wo", "test-key-1"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_oauth2_client.wo", tfjsonpath.New("jwks_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_oauth2_client.wo", tfjsonpath.New("jwks"), knownvalue.Null()),
				},
			},
			// Rotate via version bump.
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_jwks.tf.tmpl", map[string]string{
					"Name":    "Test Client with write-only JWKS",
					"Version": "2",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_oauth2_client.wo", "jwks_wo_version", "2"),
					// The rotated key set (new kid) must have reached the server.
					checkOAuth2ClientServerJWKSKid("ory_oauth2_client.wo", "test-key-2"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ory_oauth2_client.wo", tfjsonpath.New("jwks_wo"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ory_oauth2_client.wo", tfjsonpath.New("jwks"), knownvalue.Null()),
				},
			},
			// Idempotency.
			{
				Config: acctest.LoadTestConfig(t, "testdata/write_only_jwks.tf.tmpl", map[string]string{
					"Name":    "Test Client with write-only JWKS",
					"Version": "2",
				}),
				PlanOnly: true,
			},
		},
	})
}

func TestAccOAuth2ClientResource_basic(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"Name": "Test API Client",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test API Client"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read"),
					// client_secret is only returned on create
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "client_secret"),
				),
			},
			// ImportState
			{
				ResourceName:      "ory_oauth2_client.test",
				ImportState:       true,
				ImportStateVerify: true,
				// client_secret is only returned on create, not on read
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
			// Update
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", map[string]string{
					"Name": "Test API Client Updated",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test API Client Updated"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read api:write"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withAudience(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_audience.tf.tmpl", map[string]string{
					"Name":   "Test Client with Audience",
					"APIURL": testutil.ExampleAPIURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with Audience"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "audience.#", "2"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withRedirectURIs(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_redirect_uris.tf.tmpl", map[string]string{
					"Name":   "Test Client with Redirects",
					"AppURL": testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with Redirects"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "redirect_uris.#", "2"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withNewFields(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_new_fields.tf.tmpl", map[string]string{
					"Name":   "Test Client Extended",
					"AppURL": testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Extended"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "allowed_cors_origins.#", "2"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_uri", testutil.ExampleAppURL),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "logo_uri", testutil.ExampleAppURL+"/logo.png"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "policy_uri", testutil.ExampleAppURL+"/privacy"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "tos_uri", testutil.ExampleAppURL+"/tos"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

func TestAccOAuth2ClientResource_withConsentAndSubjectType(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with skip_consent, skip_logout_consent, subject_type, contacts
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_consent.tf.tmpl", map[string]string{
					"Name":        "Test Client Consent",
					"AppURL":      testutil.ExampleAppURL,
					"EmailDomain": testutil.ExampleEmailDomain,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Consent"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "skip_consent", "true"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "skip_logout_consent", "true"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "subject_type", "public"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "contacts.#", "2"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

func TestAccOAuth2ClientResource_withJWKS(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with inline JWKS
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_jwks.tf.tmpl", map[string]string{
					"Name": "Test Client with JWKS",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with JWKS"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "token_endpoint_auth_method", "private_key_jwt"),
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "jwks"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
			// Update JWKS (change key ID and scope)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_jwks_updated.tf.tmpl", map[string]string{
					"Name": "Test Client with JWKS Updated",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client with JWKS Updated"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read api:write"),
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "jwks"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withResourceCredentials(t *testing.T) {
	project := acctest.GetTestProject(t)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with resource-level project credentials
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_resource_credentials.tf.tmpl", map[string]string{
					"Name":          "Test Client Resource Creds",
					"ProjectSlug":   project.Slug,
					"ProjectAPIKey": project.APIKey,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Resource Creds"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "project_slug", project.Slug),
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "client_secret"),
				),
			},
			// ImportState — ignore resource-level credentials (not in API response)
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret", "project_slug", "project_api_key"},
			},
		},
	})
}

func TestAccOAuth2ClientResource_withCustomClientID(t *testing.T) {
	clientID := fmt.Sprintf("tf-acc-test-%d", time.Now().UnixNano())

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with custom client_id
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_custom_client_id.tf.tmpl", map[string]string{
					"Name":     "Test Client Custom ID",
					"ClientID": clientID,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_id", clientID),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "id", clientID),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Custom ID"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read"),
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "client_secret"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
			// Update (change name/scope, client_id stays the same)
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_custom_client_id_updated.tf.tmpl", map[string]string{
					"Name":     "Test Client Custom ID Updated",
					"ClientID": clientID,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_id", clientID),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "id", clientID),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Custom ID Updated"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "scope", "api:read api:write"),
				),
			},
		},
	})
}

func TestAccOAuth2ClientResource_withTokenLifespans(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_lifespans.tf.tmpl", map[string]string{
					"Name":   "Test Client Lifespans",
					"AppURL": testutil.ExampleAppURL,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_oauth2_client.test", "id"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_name", "Test Client Lifespans"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "authorization_code_grant_access_token_lifespan", "1h"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "authorization_code_grant_refresh_token_lifespan", "720h"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "client_credentials_grant_access_token_lifespan", "30m"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "backchannel_logout_session_required", "true"),
					resource.TestCheckResourceAttr("ory_oauth2_client.test", "frontchannel_logout_session_required", "true"),
				),
			},
			// ImportState
			{
				ResourceName:            "ory_oauth2_client.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}
