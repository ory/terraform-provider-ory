# Basic project configuration
resource "ory_project_config" "basic" {
  cors_enabled                                            = true
  cors_origins                                            = ["https://app.example.com"]
  selfservice_methods_password_config_min_password_length = 10
  session_lifespan                                        = "720h0m0s" # 30 days
}

# Project configuration with a write-only (ephemeral) SMTP connection URI from Vault.
# smtp_connection_uri_wo is never stored in Terraform state or plan (Terraform 1.11+).
# Bump smtp_connection_uri_wo_version whenever the secret rotates so Terraform re-sends it.
ephemeral "vault_kv_secret_v2" "smtp" {
  mount = "secret"
  name  = "ory/smtp"
}

resource "ory_project_config" "with_write_only_smtp" {
  smtp_connection_uri_wo         = ephemeral.vault_kv_secret_v2.smtp.data["connection_uri"]
  smtp_connection_uri_wo_version = "1"
}

# Full security configuration
resource "ory_project_config" "secure" {
  # Public CORS
  cors_enabled = true
  cors_origins = ["https://app.example.com", "https://admin.example.com"]

  # Admin CORS
  cors_admin_enabled = true
  cors_admin_origins = ["https://admin.example.com"]

  # Sessions
  session_lifespan                 = "168h0m0s" # 7 days
  session_earliest_possible_extend = "24h"      # Only extend sessions in the last 24h to avoid excessive writes
  session_cookie_same_site         = "Strict"
  session_cookie_persistent        = true

  # Password Policy
  selfservice_methods_password_config_min_password_length                 = 12
  selfservice_methods_password_config_identifier_similarity_check_enabled = true
  selfservice_methods_password_config_haveibeenpwned_enabled              = true
  selfservice_methods_password_config_max_breaches                        = 0

  # Authentication Methods
  selfservice_methods_password_enabled             = true
  selfservice_methods_code_enabled                 = true
  selfservice_methods_code_mfa_enabled             = true # Code as a second factor for MFA
  selfservice_methods_oidc_enabled                 = true # Required for social providers (Google, GitHub, etc.)
  selfservice_methods_oidc_enable_auto_link_policy = true # Allow social providers with auto_link to link existing identities
  selfservice_methods_passkey_enabled              = true
  selfservice_methods_profile_enabled              = true # Allow users to update profile traits via settings flow

  # Code Method Configuration
  selfservice_methods_code_config_lifespan                            = "15m0s" # How long a code remains valid
  selfservice_methods_code_config_missing_credential_fallback_enabled = true    # Use code as fallback when primary credential is missing

  # Flow Controls
  selfservice_flows_registration_enabled = true
  selfservice_flows_recovery_enabled     = true
  selfservice_flows_verification_enabled = true

  # Settings Flow
  selfservice_flows_settings_lifespan                   = "30m0s" # How long a settings flow session is valid
  selfservice_flows_settings_privileged_session_max_age = "15m0s" # Re-auth required for privileged changes after this duration

  # Verification Flow
  selfservice_flows_verification_use                       = "code"  # Use one-time code for verification (or "link")
  selfservice_flows_verification_lifespan                  = "30m0s" # How long a verification flow session is valid
  selfservice_flows_verification_notify_unknown_recipients = false   # Don't send verification emails to unknown addresses

  # MFA
  selfservice_methods_totp_enabled                    = true
  selfservice_methods_totp_config_issuer              = "MyApp"
  selfservice_methods_webauthn_enabled                = true
  selfservice_methods_webauthn_config_rp_display_name = "MyApp"
  selfservice_methods_webauthn_config_rp_id           = "app.example.com"
  webauthn_rp_origins                                 = ["https://app.example.com"]
  selfservice_methods_webauthn_config_passwordless    = true
  selfservice_methods_lookup_secret_enabled           = true
  mfa_enforcement                                     = "optional"
  selfservice_flows_settings_required_aal             = "aal1"

  # URLs
  default_return_url = "https://app.example.com/dashboard"
  allowed_return_urls = [
    "https://app.example.com/dashboard",
    "https://app.example.com/settings"
  ]

  # Account Experience Branding
  # Logos/favicons must be inline data URIs (the API does not fetch remote
  # URLs), e.g. "data:image/png;base64,${filebase64("logo.png")}".
  # Theme variables are maps of color tokens (see the AccountExperienceColors
  # API model) to CSS color values.
  account_experience_default_locale = "en"
  # account_experience_logo_light    = "data:image/png;base64,${filebase64("${path.module}/assets/logo.png")}"
  # account_experience_favicon_light = "data:image/png;base64,${filebase64("${path.module}/assets/favicon.png")}"
  # account_experience_theme_variables_light = {
  #   ax_background_default             = "#fafafa"
  #   brand_500                         = "#0066ff"
  #   button_primary_background_default = "#0066ff"
  # }

  # OAuth2 Token Lifespans
  oauth2_ttl_access_token          = "1h0m0s"
  oauth2_ttl_refresh_token         = "720h0m0s"
  oauth2_ttl_auth_code             = "30m0s"
  oauth2_ttl_id_token              = "1h0m0s"
  oauth2_ttl_login_consent_request = "30m0s"

  # OAuth2 Strategies
  oauth2_strategies_access_token    = "jwt"
  oauth2_strategies_jwt_scope_claim = "list"
  oauth2_strategies_scope           = "wildcard"

  # OAuth2 PKCE
  oauth2_pkce_enforced                    = false
  oauth2_pkce_enforced_for_public_clients = true

  # OAuth2 Claims
  oauth2_allowed_top_level_claims = ["amr", "acr"]
  oauth2_mirror_top_level_claims  = false

  # OAuth2 Issuer URL (custom issuer for OAuth2/OIDC tokens)
  oauth2_urls_self_issuer = "https://auth.example.com"

  # OAuth2 Cookie Settings
  oauth2_serve_cookies_same_site_mode              = "Strict"
  oauth2_serve_cookies_same_site_legacy_workaround = false

  # Keto Namespaces (for fine-grained authorization)
  keto_namespaces = ["documents", "folders", "groups"]
}

# Login style controls how authentication methods are presented.
# Default is "unified" (all methods on one screen).
# Use "identifier_first" to collect the identifier before showing auth methods.
resource "ory_project_config" "identifier_first" {
  selfservice_flows_login_style        = "identifier_first"
  selfservice_methods_password_enabled = true
  selfservice_methods_code_enabled     = true
}

# Self-hosted UI configuration (custom login/registration pages)
resource "ory_project_config" "self_hosted_ui" {
  selfservice_flows_login_ui_url        = "https://auth.example.com/login"
  selfservice_flows_registration_ui_url = "https://auth.example.com/registration"
  selfservice_flows_recovery_ui_url     = "https://auth.example.com/recovery"
  selfservice_flows_verification_ui_url = "https://auth.example.com/verification"
  selfservice_flows_settings_ui_url     = "https://auth.example.com/settings"
  selfservice_flows_error_ui_url        = "https://auth.example.com/error"

  selfservice_methods_password_enabled   = true
  selfservice_flows_registration_enabled = true
  selfservice_flows_recovery_enabled     = true
  selfservice_flows_verification_enabled = true
}

# SMTP configuration for custom email delivery.
#
# The URI scheme selects the security mode:
#   smtp://  -> STARTTLS (typical for port 587)
#   smtps:// -> Implicit TLS (typical for port 465)
#
# Append ?disable_starttls=true for cleartext (local dev only) or
# ?skip_ssl_verify=true to skip certificate verification.
# See the "SMTP Security Modes" section of the resource docs for details.
resource "ory_project_config" "with_smtp" {
  # STARTTLS on port 587 (recommended for most providers)
  smtp_connection_uri       = var.smtp_connection_uri
  courier_smtp_from_address = "noreply@example.com"
  courier_smtp_from_name    = "MyApp"
  smtp_headers = {
    "X-SES-CONFIGURATION-SET" = "my-config-set"
  }

  selfservice_methods_password_enabled = true
}

variable "smtp_connection_uri" {
  type      = string
  sensitive = true
  # Examples:
  #   STARTTLS:      smtp://user:pass@smtp.example.com:587
  #   Implicit TLS:  smtps://user:pass@smtp.example.com:465
  #   Cleartext:     smtp://user:pass@localhost:1025/?disable_starttls=true
  description = "SMTP connection URI. Scheme selects the security mode (smtp:// = STARTTLS, smtps:// = implicit TLS)."
}

# Native-only flows: explicitly clear browser return URLs
# Useful when the project only supports native (mobile/CLI) login flows
# and should not have any browser redirect URLs configured.
resource "ory_project_config" "native_only" {
  default_return_url  = ""
  allowed_return_urls = []

  selfservice_methods_password_enabled = true
  selfservice_methods_code_enabled     = true
}

# Session tokenizer templates (JWT tokenization for /sessions/whoami)
resource "ory_project_config" "with_tokenizer" {
  session_tokenizer_templates = {
    my_jwt = {
      ttl               = "1h"
      jwks_url          = "base64://eyJrZXlzIjpbXX0="
      claims_mapper_url = "base64://bG9jYWwgcGF5bG9hZCA9IHN0ZC5leHRWYXIoJ3BheWxvYWQnLCB7fSk7CnsKICBzZXNzaW9uX2lkOiBwYXlsb2FkLnNlc3Npb24uaWQsCn0="
      subject_source    = "id"
    }
    short_lived = {
      ttl      = "5m"
      jwks_url = "base64://eyJrZXlzIjpbXX0="
    }
  }
}

# Courier HTTP delivery (webhook-based email/SMS delivery)
resource "ory_project_config" "with_courier_http" {
  courier_delivery_strategy = "http"

  courier_http_request_config = {
    url    = "https://mail-api.example.com/send"
    method = "POST"
    body   = "base64://ewogICJyZWNpcGllbnQiOiAge3sgLnJlY2lwaWVudCB9fSwKICAiYm9keSI6IHt7IC5ib2R5IH19Cn0="
    auth = {
      type     = "basic_auth"
      user     = "mailuser"
      password = var.mail_password
    }
  }

  # Per-channel delivery (e.g., SMS via Twilio)
  courier_channels = [
    {
      id = "sms"
      request_config = {
        url    = "https://sms-api.example.com/send"
        method = "POST"
        body   = "base64://ewogICJ0byI6IHt7IC5yZWNpcGllbnQgfX0sCiAgIm1lc3NhZ2UiOiB7eyAuYm9keSB9fQp9"
        auth = {
          type  = "api_key"
          name  = "Authorization"
          value = var.sms_api_key
          in    = "header"
        }
      }
    }
  ]
}

locals {
  courier_body = <<-JSONNET
    {
      "recipient": {{ .recipient }},
      "subject": {{ .subject }},
      "body": {{ .body }}
    }
  JSONNET
}

# The same courier HTTP delivery through the flat attributes. The body must be a
# `base64://` value: the Ory API rejects a plain Jsonnet string. The API stores
# the decoded payload and reports a storage URL, and the provider resolves that
# URL back to this value, so plan stays empty while the payload matches.
resource "ory_project_config" "with_courier_http_flat" {
  courier_delivery_strategy       = "http"
  courier_http_request_config_url = "https://mail-api.example.com/send"

  courier_http_request_config_body = "base64://${base64encode(local.courier_body)}"

  courier_http_request_config_auth_basic_auth_user     = "mailuser"
  courier_http_request_config_auth_basic_auth_password = var.mail_password
}

variable "mail_password" {
  type        = string
  sensitive   = true
  description = "Password for courier HTTP basic auth"
}

variable "sms_api_key" {
  type        = string
  sensitive   = true
  description = "API key for SMS delivery service"
}

# OAuth2 token hook with API key authentication
resource "ory_project_config" "with_token_hook" {
  oauth2_token_hook = "https://example.com/token-hook"

  oauth2_token_hook_auth = {
    type  = "api_key"
    name  = "X-Api-Key"
    value = var.token_hook_api_key
    in    = "header"
  }
}

variable "token_hook_api_key" {
  type        = string
  sensitive   = true
  description = "API key sent to the OAuth2 token hook endpoint."
}

# Show the verification UI after registration and profile updates.
# Each attribute toggles the `show_verification_ui` hook for one flow while
# preserving any other hooks (e.g. `session`, `organization`) already set on
# the project.
resource "ory_project_config" "show_verification_ui" {
  selfservice_flows_verification_enabled = true

  # After password registration, redirect users to the verification UI.
  selfservice_flows_registration_after_password_hook_show_verification_ui = true
  # Same for social (OIDC) registration.
  selfservice_flows_registration_after_oidc_hook_show_verification_ui = true
  # When users change their email in profile settings, force re-verification.
  selfservice_flows_settings_after_profile_hook_show_verification_ui = true
}

# Automatically sign users in after they register with email + password.
# OIDC, WebAuthn and Passkey flows already issue a session on registration,
# so this toggle only affects the password flow — it mirrors the Ory Console
# "Enable sign in after registration" toggle. Existing hooks at the same path
# (e.g. `organization`) are preserved.
resource "ory_project_config" "sign_in_after_registration" {
  selfservice_flows_registration_after_password_hook_session = true
}
