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
	// Event stream creation requires real AWS resources because the API validates
	// connectivity by assuming the IAM role and publishing a test message to SNS.
	if os.Getenv("ORY_EVENT_STREAM_TOPIC_ARN") == "" || os.Getenv("ORY_EVENT_STREAM_ROLE_ARN") == "" {
		t.Skip("ORY_EVENT_STREAM_TOPIC_ARN and ORY_EVENT_STREAM_ROLE_ARN must be set with real AWS ARNs")
	}
}

// TestAccEventStreamResource_basic tests the full CRUD lifecycle of an event stream.
// Requires real AWS SNS topic and IAM role because the API validates connectivity.
//
// Required environment variables:
//
//	ORY_EVENT_STREAM_TOPIC_ARN - Real AWS SNS topic ARN
//	ORY_EVENT_STREAM_ROLE_ARN  - Real AWS IAM role ARN with trust policy for Ory
func TestAccEventStreamResource_basic(t *testing.T) {
	topicArn := os.Getenv("ORY_EVENT_STREAM_TOPIC_ARN")
	roleArn := os.Getenv("ORY_EVENT_STREAM_ROLE_ARN")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckEventStream(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"TopicArn": topicArn,
					"RoleArn":  roleArn,
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "id"),
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
