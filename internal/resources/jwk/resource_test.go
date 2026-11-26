//go:build acceptance

package jwk_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/acctest"
	"github.com/ory/terraform-provider-ory/internal/client"
)

func importStateJWKID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_json_web_key_set.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_json_web_key_set.test")
	}
	projectID := rs.Primary.Attributes["project_id"]
	if projectID == "" {
		return "", fmt.Errorf("project_id is empty in state; cannot build composite import ID")
	}
	setID := rs.Primary.Attributes["set_id"]
	if setID == "" {
		return "", fmt.Errorf("set_id is empty in state; cannot build composite import ID")
	}
	return fmt.Sprintf("%s/%s", projectID, setID), nil
}

// testCheckKeysContainPrivateMaterial verifies that the keys JSON output
// contains private key fields for the given algorithm.
func testCheckKeysContainPrivateMaterial(resourceName, algorithm string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		keysJSON := rs.Primary.Attributes["keys"]
		if keysJSON == "" {
			return fmt.Errorf("keys attribute is empty")
		}

		var jwks struct {
			Keys []map[string]interface{} `json:"keys"`
		}
		if err := json.Unmarshal([]byte(keysJSON), &jwks); err != nil {
			return fmt.Errorf("failed to parse keys JSON: %w", err)
		}
		if len(jwks.Keys) == 0 {
			return fmt.Errorf("keys JSON contains no keys")
		}

		key := jwks.Keys[0]

		// Check for private key fields based on algorithm type
		var requiredFields []string
		switch algorithm {
		case "RS256":
			requiredFields = []string{"d", "p", "q", "dp", "dq", "qi"}
		case "ES256", "ES512":
			requiredFields = []string{"d"}
		case "HS256", "HS512":
			requiredFields = []string{"k"}
		default:
			return fmt.Errorf("unsupported algorithm %q for private key material check", algorithm)
		}

		for _, field := range requiredFields {
			if _, exists := key[field]; !exists {
				return fmt.Errorf("keys output missing private key field %q for algorithm %s", field, algorithm)
			}
		}
		return nil
	}
}

func TestAccJWKResource_basic(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		t.Skip("ORY_PROJECT_ID must be set for JWK acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"ProjectID": projectID,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "id"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "project_id", projectID),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "set_id", "tf-test-jwks"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "key_id", "tf-test-key"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "algorithm", "RS256"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "use", "sig"),
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "keys"),
					testCheckKeysContainPrivateMaterial("ory_json_web_key_set.test", "RS256"),
				),
			},
			// ImportState using composite ID: project_id/set_id
			{
				ResourceName:      "ory_json_web_key_set.test",
				ImportState:       true,
				ImportStateIdFunc: importStateJWKID,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccJWKResource_withResourceCredentials(t *testing.T) {
	project := acctest.GetTestProject(t)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create with resource-level project credentials
			{
				Config: acctest.LoadTestConfig(t, "testdata/with_resource_credentials.tf.tmpl", map[string]string{
					"ProjectSlug":   project.Slug,
					"ProjectAPIKey": project.APIKey,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "id"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "set_id", "tf-test-jwks-creds"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "key_id", "tf-test-key-creds"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "algorithm", "RS256"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "use", "sig"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "project_slug", project.Slug),
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "keys"),
					testCheckKeysContainPrivateMaterial("ory_json_web_key_set.test", "RS256"),
				),
			},
			// ImportState — ignore resource-level credentials (not in API response)
			{
				ResourceName:            "ory_json_web_key_set.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"project_slug", "project_api_key"},
			},
		},
	})
}

func TestAccJWKResource_ec_key(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		t.Skip("ORY_PROJECT_ID must be set for JWK acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/ec_key.tf.tmpl", map[string]string{
					"ProjectID": projectID,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "id"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "algorithm", "ES256"),
					resource.TestCheckResourceAttr("ory_json_web_key_set.test", "use", "sig"),
					resource.TestCheckResourceAttrSet("ory_json_web_key_set.test", "keys"),
					testCheckKeysContainPrivateMaterial("ory_json_web_key_set.test", "ES256"),
				),
			},
			{
				ResourceName:      "ory_json_web_key_set.test",
				ImportState:       true,
				ImportStateIdFunc: importStateJWKID,
				ImportStateVerify: true,
			},
		},
	})
}

const (
	suppliedSetID = "tf-test-jwks-supplied"
	importedSetID = "tf-test-jwks-imported"
)

// publicECKey returns a new public-only P-256 key.
func publicECKey(t *testing.T, kid string) ory.JsonWebKey {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	point := key.PublicKey().Bytes() // 0x04 || X || Y
	return ory.JsonWebKey{
		Kty: "EC", Crv: ory.PtrString("P-256"), Kid: kid, Use: "sig", Alg: "ES256",
		X: ory.PtrString(base64.RawURLEncoding.EncodeToString(point[1:33])),
		Y: ory.PtrString(base64.RawURLEncoding.EncodeToString(point[33:])),
	}
}

// publicRSAKey returns a new public-only RSA key.
func publicRSAKey(t *testing.T, kid string) ory.JsonWebKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return ory.JsonWebKey{
		Kty: "RSA", Kid: kid, Use: "sig", Alg: "RS256",
		N: ory.PtrString(base64.RawURLEncoding.EncodeToString(key.N.Bytes())),
		E: ory.PtrString(base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())),
	}
}

// prettyJWKS encodes keys as an indented JWKS document, like a key file on disk.
func prettyJWKS(t *testing.T, keys ...ory.JsonWebKey) string {
	t.Helper()
	doc, err := json.MarshalIndent(ory.JsonWebKeySet{Keys: keys}, "", "  ")
	require.NoError(t, err)
	return string(doc)
}

// testCheckPublicKeys verifies that the keys attribute holds exactly the given
// kids, in kid order, with no private key material.
func testCheckPublicKeys(resourceName string, kids ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		var set struct {
			Keys []map[string]interface{} `json:"keys"`
		}
		if err := json.Unmarshal([]byte(rs.Primary.Attributes["keys"]), &set); err != nil {
			return fmt.Errorf("failed to parse keys JSON: %w", err)
		}
		var got []string
		for _, key := range set.Keys {
			got = append(got, fmt.Sprint(key["kid"]))
			for _, private := range []string{"d", "p", "q", "dp", "dq", "qi", "k"} {
				if _, exists := key[private]; exists {
					return fmt.Errorf("key %v holds private key member %q", key["kid"], private)
				}
			}
		}
		if !slices.Equal(got, kids) {
			return fmt.Errorf("keys holds kids %v, want %v", got, kids)
		}
		return nil
	}
}

func testCheckSetDestroyed(setID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c, err := acctest.GetOryClient()
		if err != nil {
			return err
		}
		_, err = c.GetJsonWebKeySet(context.Background(), setID)
		if client.IsNotFoundStatus(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("JWK set %s still exists after destroy", setID)
	}
}

// TestAccJWKResource_suppliedKeys covers a set uploaded through jwks, as issue
// #463 asks: public-only keys, rotation in place, drift, import, and moving
// between supplied and generated keys.
func TestAccJWKResource_suppliedKeys(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		t.Skip("ORY_PROJECT_ID must be set for JWK acceptance tests")
	}

	first := publicECKey(t, "tf-test-supplied-1")
	second := publicRSAKey(t, "tf-test-supplied-2")
	outOfBand := publicECKey(t, "tf-test-out-of-band")

	oneKey := acctest.LoadTestConfig(t, "testdata/supplied_keys.tf.tmpl", map[string]string{
		"ProjectID": projectID,
		"SetID":     suppliedSetID,
		"JWKS":      prettyJWKS(t, first),
	})
	// The new key goes first, so the upload order differs from the kid order.
	twoKeys := acctest.LoadTestConfig(t, "testdata/supplied_keys.tf.tmpl", map[string]string{
		"ProjectID": projectID,
		"SetID":     suppliedSetID,
		"JWKS":      prettyJWKS(t, second, first),
	})
	generated := acctest.LoadTestConfig(t, "testdata/supplied_keys_generated.tf.tmpl", map[string]string{
		"ProjectID": projectID,
	})
	const name = "ory_json_web_key_set.test"

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             testCheckSetDestroyed(suppliedSetID),
		Steps: []resource.TestStep{
			// Create with one public-only key.
			{
				Config: oneKey,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "id", suppliedSetID),
					resource.TestCheckResourceAttr(name, "project_id", projectID),
					resource.TestCheckResourceAttrSet(name, "jwks"),
					resource.TestCheckNoResourceAttr(name, "key_id"),
					resource.TestCheckNoResourceAttr(name, "algorithm"),
					resource.TestCheckNoResourceAttr(name, "use"),
					testCheckPublicKeys(name, "tf-test-supplied-1"),
				),
			},
			// Rotate: add a second key in place.
			{
				Config: twoKeys,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
				Check: testCheckPublicKeys(name, "tf-test-supplied-1", "tf-test-supplied-2"),
			},
			// Replace the keys outside Terraform: the plan must upload jwks again.
			{
				PreConfig: func() {
					c, err := acctest.GetOryClient()
					require.NoError(t, err)
					_, err = c.SetJsonWebKeySet(context.Background(), suppliedSetID, ory.JsonWebKeySet{Keys: []ory.JsonWebKey{outOfBand}})
					require.NoError(t, err, "could not replace the keys out of band")
				},
				Config: twoKeys,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
				Check: testCheckPublicKeys(name, "tf-test-supplied-1", "tf-test-supplied-2"),
			},
			// Import. Ory cannot tell a supplied set from a generated one, so the
			// import tracks the first key like a generated set does.
			{
				ResourceName:            name,
				ImportState:             true,
				ImportStateIdFunc:       importStateJWKID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"jwks", "key_id", "algorithm", "use"},
			},
			// A generated key needs a new set, so moving to one replaces it.
			{
				Config: generated,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(name, "key_id", "tf-test-generated"),
					resource.TestCheckNoResourceAttr(name, "jwks"),
					testCheckKeysContainPrivateMaterial(name, "ES256"),
				),
			},
			// Moving back to supplied keys uploads them in place.
			{
				Config: oneKey,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(name, "key_id"),
					testCheckPublicKeys(name, "tf-test-supplied-1"),
				),
			},
		},
	})
}

// TestAccJWKResource_importSuppliedKeys adopts a set that was uploaded outside
// Terraform, as "ory import jwk" does. The import tracks the first key, so the
// first apply of the jwks configuration must upload it in place, not replace it.
func TestAccJWKResource_importSuppliedKeys(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	if projectID == "" {
		t.Skip("ORY_PROJECT_ID must be set for JWK acceptance tests")
	}

	key := publicECKey(t, "tf-test-imported-1")
	config := acctest.LoadTestConfig(t, "testdata/supplied_keys.tf.tmpl", map[string]string{
		"ProjectID": projectID,
		"SetID":     importedSetID,
		"JWKS":      prettyJWKS(t, key),
	})
	const name = "ory_json_web_key_set.test"

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             testCheckSetDestroyed(importedSetID),
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					c, err := acctest.GetOryClient()
					require.NoError(t, err)
					_, err = c.SetJsonWebKeySet(context.Background(), importedSetID, ory.JsonWebKeySet{Keys: []ory.JsonWebKey{key}})
					require.NoError(t, err, "could not upload the key set out of band")
				},
				Config:             config,
				ResourceName:       name,
				ImportState:        true,
				ImportStateId:      projectID + "/" + importedSetID,
				ImportStatePersist: true,
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(name, "jwks"),
					resource.TestCheckNoResourceAttr(name, "key_id"),
					resource.TestCheckNoResourceAttr(name, "algorithm"),
					resource.TestCheckNoResourceAttr(name, "use"),
					testCheckPublicKeys(name, "tf-test-imported-1"),
				),
			},
		},
	})
}

func TestAccJWKResource_suppliedKeysValidation(t *testing.T) {
	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      acctest.LoadTestConfig(t, "testdata/jwks_and_key_id.tf.tmpl", nil),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`cannot\s+be\s+specified\s+when`),
			},
			{
				Config:      acctest.LoadTestConfig(t, "testdata/no_key.tf.tmpl", nil),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`At\s+least\s+one\s+of\s+these\s+attributes`),
			},
			{
				Config: acctest.LoadTestConfig(t, "testdata/supplied_keys.tf.tmpl", map[string]string{
					"ProjectID": "unused",
					"SetID":     "tf-test-jwks-invalid",
					"JWKS":      `{"keys": []}`,
				}),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`at\s+least\s+one\s+key`),
			},
		},
	})
}
