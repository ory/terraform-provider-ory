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

# Post-login webhook for SAML single sign-on (SSO) logins
resource "ory_action" "after_login_saml" {
  flow        = "login"
  timing      = "after"
  auth_method = "saml"
  url         = "https://api.example.com/webhooks/after-login-saml"
  method      = "POST"
}

# Post-settings webhook for profile/trait updates. The profile method exists only
# on the settings flow.
resource "ory_action" "after_profile_update" {
  flow        = "settings"
  timing      = "after"
  auth_method = "profile"
  url         = "https://api.example.com/webhooks/profile-updated"
  method      = "POST"
}

# Webhook with write-only (ephemeral) authentication secrets sourced from Vault.
# The *_wo secrets are never stored in Terraform state or plan (Terraform 1.11+).
# Bump the matching *_wo_version whenever a secret rotates so Terraform re-sends it.
ephemeral "vault_kv_secret_v2" "webhook_auth" {
  mount = "secret"
  name  = "ory/webhook-auth"
}

resource "ory_action" "secured_webhook" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/secured"
  method      = "POST"

  webhook_auth_type                           = "basic_auth"
  webhook_auth_basic_auth_user                = "webhook-user"
  webhook_auth_basic_auth_password_wo         = ephemeral.vault_kv_secret_v2.webhook_auth.data["password"]
  webhook_auth_basic_auth_password_wo_version = "1"
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
# The verification (and recovery) flow is not scoped by authentication method,
# so auth_method is omitted here — the hook always runs after verification.
resource "ory_action" "sync_verified" {
  flow   = "verification"
  timing = "after"
  url    = "https://api.example.com/webhooks/user-verified"
  method = "POST"
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

# Webhook with basic auth
resource "ory_action" "with_basic_auth" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/secured"
  method      = "POST"

  webhook_auth_type                = "basic_auth"
  webhook_auth_basic_auth_user     = var.webhook_user
  webhook_auth_basic_auth_password = var.webhook_password
}

# Webhook with API key auth (sent as header)
resource "ory_action" "with_api_key" {
  flow        = "login"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/login"
  method      = "POST"

  webhook_auth_type          = "api_key"
  webhook_auth_api_key_name  = "X-API-KEY"
  webhook_auth_api_key_value = var.api_key
  webhook_auth_api_key_in    = "header"
}
