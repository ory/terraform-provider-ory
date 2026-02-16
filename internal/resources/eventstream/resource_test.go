//go:build acceptance

package eventstream_test

import (
	"fmt"
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
}

func TestAccEventStreamResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckEventStream(t) },
		ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
					"TopicArn": "arn:aws:sns:us-east-1:123456789012:test-topic",
					"RoleArn":  "arn:aws:iam::123456789012:role/test-role",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "id"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "type", "sns"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "topic_arn", "arn:aws:sns:us-east-1:123456789012:test-topic"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "role_arn", "arn:aws:iam::123456789012:role/test-role"),
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
			// Update topic_arn
			{
				Config: acctest.LoadTestConfig(t, "testdata/updated.tf.tmpl", map[string]string{
					"TopicArn": "arn:aws:sns:us-east-1:123456789012:test-topic-updated",
					"RoleArn":  "arn:aws:iam::123456789012:role/test-role",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("ory_event_stream.test", "id"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "type", "sns"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "topic_arn", "arn:aws:sns:us-east-1:123456789012:test-topic-updated"),
					resource.TestCheckResourceAttr("ory_event_stream.test", "role_arn", "arn:aws:iam::123456789012:role/test-role"),
				),
			},
		},
	})
}
