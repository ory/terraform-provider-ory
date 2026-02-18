# Basic project configuration
resource "ory_project_config" "basic" {
  cors_enabled        = true
  cors_origins        = ["https://app.example.com"]
  password_min_length = 10
  session_lifespan    = "720h" # 30 days
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
  session_lifespan          = "168h" # 7 days
  session_cookie_same_site  = "Strict"
  session_cookie_persistent = true

  # Password Policy
  password_min_length            = 12
  password_identifier_similarity = true
  password_check_haveibeenpwned  = true
  password_max_breaches          = 0

  # Authentication Methods
  enable_password = true
  enable_code     = true
  enable_passkey  = true

  # Flow Controls
  enable_registration = true
  enable_recovery     = true
  enable_verification = true

  # MFA
  enable_totp              = true
  totp_issuer              = "MyApp"
  enable_webauthn          = true
  webauthn_rp_display_name = "MyApp"
  webauthn_rp_id           = "app.example.com"
  webauthn_rp_origins      = ["https://app.example.com"]
  webauthn_passwordless    = true
  enable_lookup_secret     = true
  mfa_enforcement          = "optional"
  required_aal             = "aal1"

  # URLs
  default_return_url = "https://app.example.com/dashboard"
  allowed_return_urls = [
    "https://app.example.com/dashboard",
    "https://app.example.com/settings"
  ]

  # Account Experience Branding
  account_experience_name           = "MyApp"
  account_experience_logo_url       = "https://cdn.example.com/logo.png"
  account_experience_favicon_url    = "https://cdn.example.com/favicon.ico"
  account_experience_default_locale = "en"

  # OAuth2 Token Lifespans
  oauth2_access_token_lifespan  = "1h"
  oauth2_refresh_token_lifespan = "720h"

  # Keto Namespaces (for fine-grained authorization)
  keto_namespaces = ["documents", "folders", "groups"]
}

# Self-hosted UI configuration (custom login/registration pages)
resource "ory_project_config" "self_hosted_ui" {
  login_ui_url        = "https://auth.example.com/login"
  registration_ui_url = "https://auth.example.com/registration"
  recovery_ui_url     = "https://auth.example.com/recovery"
  verification_ui_url = "https://auth.example.com/verification"
  settings_ui_url     = "https://auth.example.com/settings"
  error_ui_url        = "https://auth.example.com/error"

  enable_password     = true
  enable_registration = true
  enable_recovery     = true
  enable_verification = true
}

# SMTP configuration for custom email delivery
resource "ory_project_config" "with_smtp" {
  smtp_connection_uri = var.smtp_connection_uri
  smtp_from_address   = "noreply@example.com"
  smtp_from_name      = "MyApp"
  smtp_headers = {
    "X-SES-CONFIGURATION-SET" = "my-config-set"
  }

  enable_password = true
}

variable "smtp_connection_uri" {
  type        = string
  sensitive   = true
  description = "SMTP connection URI (e.g., smtps://user:pass@smtp.example.com:465)"
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
