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

  # MFA
  enable_totp              = true
  totp_issuer              = "MyApp"
  enable_webauthn          = true
  webauthn_rp_display_name = "MyApp"
  webauthn_rp_id           = "app.example.com"
  webauthn_rp_origins      = ["https://app.example.com"]
  webauthn_passwordless    = true

  # Account Experience Branding
  account_experience_name           = "MyApp"
  account_experience_logo_url       = "https://cdn.example.com/logo.png"
  account_experience_default_locale = "en"

  # OAuth2 Token Lifespans
  oauth2_access_token_lifespan  = "1h"
  oauth2_refresh_token_lifespan = "720h"
}
