//go:build acceptance

package eventstream_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ory/terraform-provider-ory/internal/acctest"
)

func importStateEventStreamID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["ory_event_stream.test"]
	if !ok {
		return "", fmt.Errorf("resource not found: ory_event_stream.test")
	}
	projectID := rs.Primary.Attributes["project_id"]
	streamID := rs.Primary.ID
	return fmt.Sprintf("%s/%s", projectID, streamID), nil
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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckEventStream(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
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
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "created_at"),
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "updated_at"),
				),
			},
			// ImportState using composite ID: project_id/event_stream_id
			{
				ResourceName:      "ory_event_stream.test",
				ImportState:       true,
				ImportStateIdFunc: importStateEventStreamID,
				ImportStateVerify: true,
				// Timestamp precision may differ between create and read
				ImportStateVerifyIgnore: []string{"created_at", "updated_at"},
			},
		},
	})
}
