# Publish Ory events to an AWS SNS topic
resource "ory_event_stream" "events" {
  type      = "sns"
  topic_arn = "arn:aws:sns:us-east-1:123456789012:ory-events"
  role_arn  = "arn:aws:iam::123456789012:role/ory-event-publisher"
}

# Send Ory events to an HTTPS endpoint, authenticated with a bearer token
variable "event_stream_token" {
  type      = string
  sensitive = true
}

resource "ory_event_stream" "webhook" {
  type              = "https"
  https_endpoint    = "https://events.example.com/ory"
  https_auth_type   = "bearer"
  https_auth_secret = var.event_stream_token
}

# Send Ory events with an API key header, and keep the stream paused
resource "ory_event_stream" "paused" {
  type                   = "https"
  https_endpoint         = "https://events.example.com/ory-staging"
  https_auth_type        = "api_key"
  https_auth_header_name = "X-Api-Key"
  https_auth_secret      = var.event_stream_token
  status                 = "paused"
}

output "event_stream_id" {
  value = ory_event_stream.events.id
}
