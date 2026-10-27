//go:build acceptance

package eventstream_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func importStateEventStreamID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		projectID := rs.Primary.Attributes["project_id"]
		streamID := rs.Primary.ID
		return fmt.Sprintf("%s/%s", projectID, streamID), nil
	}
}

func testAccPreCheckEventStream(t *testing.T) {
	acctest.AccPreCheck(t)
	acctest.RequireEventStreamTests(t)
	// Event stream tests require the project UUID to be registered as the
	// ExternalId in the IAM role's trust policy. The Ory API validates this.
	for _, env := range []string{"ORY_EVENT_STREAM_TOPIC_ARN", "ORY_EVENT_STREAM_ROLE_ARN", "ORY_PROJECT_ID"} {
		if os.Getenv(env) == "" {
			t.Skipf("%s must be set for event stream tests", env)
		}
	}
}

// HTTPS streams need no AWS setup. The tests create paused streams, which
// Ory does not test against the endpoint, so no reachable endpoint is needed.
func testAccPreCheckEventStreamHTTPS(t *testing.T) {
	acctest.AccPreCheck(t)
	acctest.RequireEventStreamTests(t)
}

// checkStoredStream reads the stream behind resourceName from the API and
// passes it to check. State echoes the configuration, so only the API shows
// what Ory stored, with the endpoint credentials and the secret masked.
func checkStoredStream(t *testing.T, resourceName string, check func(*ory.EventStream) error) resource.TestCheckFunc {
	t.Helper()
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		c, err := acctest.GetOryClient()
		if err != nil {
			return err
		}
		stream, err := c.GetEventStream(context.Background(), rs.Primary.Attributes["project_id"], rs.Primary.ID)
		if err != nil {
			return err
		}
		return check(stream)
	}
}

// checkEventStreamsDestroyed fails while an event stream from state still
// exists on the test project.
func checkEventStreamsDestroyed(t *testing.T) resource.TestCheckFunc {
	t.Helper()
	return func(s *terraform.State) error {
		c, err := acctest.GetOryClient()
		if err != nil {
			return err
		}
		streams, err := c.ListEventStreams(context.Background(), acctest.GetTestProjectID(t))
		if err != nil {
			return err
		}
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ory_event_stream" {
				continue
			}
			for _, stream := range streams {
				if stream.GetId() == rs.Primary.ID {
					return fmt.Errorf("event stream %s still exists after destroy", rs.Primary.ID)
				}
			}
		}
		return nil
	}
}

// TestAccEventStreamResource_basic tests the full CRUD lifecycle of an event stream.
// Requires real AWS SNS topic, IAM role, and a dedicated Ory project whose UUID
// is registered as the ExternalId in the IAM role's trust policy.
//
// Required environment variables:
//
//	ORY_PROJECT_ID             - Ory project ID (UUID in IAM trust policy ExternalId)
//	ORY_EVENT_STREAM_TOPIC_ARN - Real AWS SNS topic ARN
//	ORY_EVENT_STREAM_ROLE_ARN  - Real AWS IAM role ARN with trust policy for Ory
func TestAccEventStreamResource_basic(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	topicArn := os.Getenv("ORY_EVENT_STREAM_TOPIC_ARN")
	roleArn := os.Getenv("ORY_EVENT_STREAM_ROLE_ARN")

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckEventStream(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkEventStreamsDestroyed(t),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"ProjectID": projectID,
					"TopicArn":  topicArn,
					"RoleArn":   roleArn,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "id"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "project_id", projectID),
					resource.TestCheckResourceAttr("ory_event_stream.test", "type", "sns"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "topic_arn", topicArn),
					resource.TestCheckResourceAttr("ory_event_stream.test", "role_arn", roleArn),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_type", "none"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "status", "active"),
					resource.TestCheckNoResourceAttr("ory_event_stream.test", "https_endpoint"),
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "created_at"),
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "updated_at"),
				),
			},
			// ImportState using composite ID: project_id/event_stream_id
			{
				ResourceName:      "ory_event_stream.test",
				ImportState:       true,
				ImportStateIdFunc: importStateEventStreamID("ory_event_stream.test"),
				ImportStateVerify: true,
				// Timestamp precision may differ between create and read
				ImportStateVerifyIgnore: []string{"created_at", "updated_at"},
			},
		},
	})
}

// TestAccEventStreamResource_https covers an HTTPS stream through create,
// a scheme and endpoint change, an in-place switch to SNS, import, and
// delete. The first endpoint carries Basic credentials and a query token,
// which the API masks on read, so the empty plan after each step proves that
// the masked endpoint does not show as a diff.
func TestAccEventStreamResource_https(t *testing.T) {
	const (
		endpoint       = "https://tf-acc:basic-auth-value@example.com/ory-events?token=query-value"
		maskedEndpoint = "https://xxxxx@example.com/ory-events?token=xxxxx"
		secondEndpoint = "https://example.com/ory-events/v2"
		pausedTopicArn = "arn:aws:sns:us-east-1:123456789012:tf-acc-paused"
		pausedRoleArn  = "arn:aws:iam::123456789012:role/tf-acc-paused"
	)

	acctest.RunTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckEventStreamHTTPS(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkEventStreamsDestroyed(t),
		Steps: []resource.TestStep{
			{
				Config: acctest.LoadTestConfig(t, "testdata/https_api_key.tf.tmpl", map[string]string{
					"Endpoint": endpoint,
					"Secret":   "tf-acc-api-key",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "id"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "type", "https"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_endpoint", endpoint),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_type", "api_key"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_header_name", "X-Api-Key"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_secret", "tf-acc-api-key"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "status", "paused"),
					resource.TestCheckNoResourceAttr("ory_event_stream.test", "topic_arn"),
					resource.TestCheckNoResourceAttr("ory_event_stream.test", "role_arn"),
					checkStoredStream(t, "ory_event_stream.test", func(stream *ory.EventStream) error {
						if got := stream.HttpsEndpoint.Get(); got == nil || *got != maskedEndpoint {
							return fmt.Errorf("stored endpoint reported as %v, want %s", got, maskedEndpoint)
						}
						if stream.GetHttpsAuthSecret() == "" {
							return fmt.Errorf("Ory stored no secret for the stream")
						}
						return nil
					}),
				),
			},
			// Switch to a bearer token and a new endpoint, in place.
			{
				Config: acctest.LoadTestConfig(t, "testdata/https_bearer.tf.tmpl", map[string]string{
					"Endpoint": secondEndpoint,
					"Secret":   "tf-acc-bearer-token",
				}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ory_event_stream.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_endpoint", secondEndpoint),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_type", "bearer"),
					resource.TestCheckNoResourceAttr("ory_event_stream.test", "https_auth_header_name"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_secret", "tf-acc-bearer-token"),
					checkStoredStream(t, "ory_event_stream.test", func(stream *ory.EventStream) error {
						if stream.GetHttpsAuthType() != "bearer" || stream.GetHttpsAuthSecret() == "" {
							return fmt.Errorf("stored scheme %q with secret %q, want bearer with a secret",
								stream.GetHttpsAuthType(), stream.GetHttpsAuthSecret())
						}
						return nil
					}),
				),
			},
			// Switch the type to SNS in place. A paused SNS stream skips the
			// AWS check, so syntactically valid ARNs are enough.
			{
				Config: acctest.LoadTestConfig(t, "testdata/sns_paused.tf.tmpl", map[string]string{
					"TopicArn": pausedTopicArn,
					"RoleArn":  pausedRoleArn,
				}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ory_event_stream.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_event_stream.test", "type", "sns"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "topic_arn", pausedTopicArn),
					resource.TestCheckResourceAttr("ory_event_stream.test", "role_arn", pausedRoleArn),
					resource.TestCheckResourceAttr("ory_event_stream.test", "https_auth_type", "none"),
					resource.TestCheckNoResourceAttr("ory_event_stream.test", "https_endpoint"),
					resource.TestCheckNoResourceAttr("ory_event_stream.test", "https_auth_secret"),
				),
			},
			{
				ResourceName:            "ory_event_stream.test",
				ImportState:             true,
				ImportStateIdFunc:       importStateEventStreamID("ory_event_stream.test"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"created_at", "updated_at"},
			},
		},
	})
}

// TestAccEventStreamResource_httpsWriteOnlySecret verifies that the
// write-only secret reaches Ory but never Terraform state, and that bumping
// the version trigger sends it again. Write-only arguments need Terraform
// 1.11+.
func TestAccEventStreamResource_httpsWriteOnlySecret(t *testing.T) {
	config := func(secret, version string) string {
		return acctest.LoadTestConfig(t, "testdata/https_write_only_secret.tf.tmpl", map[string]string{
			"Secret":  secret,
			"Version": version,
		})
	}
	secretNotInState := []statecheck.StateCheck{
		statecheck.ExpectKnownValue("ory_event_stream.wo", tfjsonpath.New("https_auth_secret_wo"), knownvalue.Null()),
		statecheck.ExpectKnownValue("ory_event_stream.wo", tfjsonpath.New("https_auth_secret"), knownvalue.Null()),
	}
	secretStored := checkStoredStream(t, "ory_event_stream.wo", func(stream *ory.EventStream) error {
		if stream.GetHttpsAuthSecret() == "" {
			return fmt.Errorf("Ory stored no secret for the stream")
		}
		return nil
	})

	acctest.RunTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheckEventStreamHTTPS(t) },
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		CheckDestroy:             checkEventStreamsDestroyed(t),
		Steps: []resource.TestStep{
			{
				Config: config("tf-acc-wo-token-one", "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_event_stream.wo", "https_auth_type", "bearer"),
					resource.TestCheckResourceAttr("ory_event_stream.wo", "https_auth_secret_wo_version", "1"),
					secretStored,
				),
				ConfigStateChecks: secretNotInState,
			},
			// Rotate by bumping the version trigger.
			{
				Config: config("tf-acc-wo-token-two", "2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ory_event_stream.wo", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("ory_event_stream.wo", "https_auth_secret_wo_version", "2"),
					secretStored,
				),
				ConfigStateChecks: secretNotInState,
			},
			{
				ResourceName:      "ory_event_stream.wo",
				ImportState:       true,
				ImportStateIdFunc: importStateEventStreamID("ory_event_stream.wo"),
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"created_at",
					"updated_at",
					"https_auth_secret_wo",
					"https_auth_secret_wo_version",
				},
			},
		},
	})
}
