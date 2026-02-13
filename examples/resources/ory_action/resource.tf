# Post-registration webhook
resource "ory_action" "welcome_email" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/welcome"
  method      = "POST"
  body        = <<-JSONNET
    function(ctx) {
      email: ctx.identity.traits.email,
      name: ctx.identity.traits.name,
      created_at: ctx.identity.created_at
    }
  JSONNET
}

# Pre-login validation
resource "ory_action" "validate_login" {
  flow          = "login"
  timing        = "before"
  url           = "https://api.example.com/webhooks/validate"
  method        = "POST"
  can_interrupt = true # Allow webhook to block login
}

# Async audit log (fire and forget)
resource "ory_action" "audit_log" {
  flow            = "settings"
  timing          = "after"
  auth_method     = "password"
  url             = "https://api.example.com/webhooks/audit"
  method          = "POST"
  response_ignore = true
}

# Post-verification sync
resource "ory_action" "sync_verified" {
  flow        = "verification"
  timing      = "after"
  auth_method = "code"
  url         = "https://api.example.com/webhooks/user-verified"
  method      = "POST"
}

# Post-registration enrichment (parse response to modify identity)
resource "ory_action" "enrich_identity" {
  flow           = "registration"
  timing         = "after"
  auth_method    = "password"
  url            = "https://api.example.com/webhooks/enrich"
  method         = "POST"
  response_parse = true # Parse the webhook response to update identity traits
  body           = <<-JSONNET
    function(ctx) {
      identity_id: ctx.identity.id,
      email: ctx.identity.traits.email
    }
  JSONNET
}
