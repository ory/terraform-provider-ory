# Publish Ory events to an AWS SNS topic
resource "ory_event_stream" "events" {
  type      = "sns"
  topic_arn = "arn:aws:sns:us-east-1:123456789012:ory-events"
  role_arn  = "arn:aws:iam::123456789012:role/ory-event-publisher"
}

output "event_stream_id" {
  value = ory_event_stream.events.id
}
