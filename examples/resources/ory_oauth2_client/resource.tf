# Machine-to-machine client (Client Credentials flow)
resource "ory_oauth2_client" "api_service" {
  client_name                = "API Service"
  grant_types                = ["client_credentials"]
  token_endpoint_auth_method = "client_secret_post"
  scope                      = "read write admin"
  audience                   = ["https://api.example.com"]
  access_token_strategy      = "jwt"
  contacts                   = ["api-team@example.com"]

  # Custom token lifespans for this client
  client_credentials_grant_access_token_lifespan = "30m"
}

# Web application (Authorization Code flow) with OIDC logout and metadata
resource "ory_oauth2_client" "web_app" {
  client_name    = "Web Application"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  redirect_uris = [
    "https://app.example.com/callback",
    "https://app.example.com/auth/callback"
  ]
  post_logout_redirect_uris  = ["https://app.example.com/logout"]
  token_endpoint_auth_method = "client_secret_basic"
  scope                      = "openid profile email offline_access"

  # First-party app: skip consent and logout consent screens
  skip_consent        = true
  skip_logout_consent = true
  subject_type        = "pairwise"
  contacts            = ["web-team@example.com"]

  # Client metadata URIs
  client_uri = "https://app.example.com"
  logo_uri   = "https://app.example.com/logo.png"
  policy_uri = "https://app.example.com/privacy"
  tos_uri    = "https://app.example.com/terms"

  # OIDC logout with session notifications
  frontchannel_logout_uri              = "https://app.example.com/logout/frontchannel"
  frontchannel_logout_session_required = true
  backchannel_logout_uri               = "https://app.example.com/logout/backchannel"
  backchannel_logout_session_required  = true

  # Per-client CORS
  allowed_cors_origins = [
    "https://app.example.com",
    "https://admin.example.com"
  ]

  # Per-grant token lifespans
  authorization_code_grant_access_token_lifespan  = "1h"
  authorization_code_grant_id_token_lifespan      = "1h"
  authorization_code_grant_refresh_token_lifespan = "720h"
  refresh_token_grant_access_token_lifespan       = "1h"
  refresh_token_grant_id_token_lifespan           = "1h"
  refresh_token_grant_refresh_token_lifespan      = "720h"

  # Custom metadata
  metadata = jsonencode({
    department = "engineering"
    tier       = "internal"
  })
}

# Single Page Application (Public client with PKCE)
resource "ory_oauth2_client" "spa" {
  client_name    = "Single Page App"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  redirect_uris = [
    "https://spa.example.com/callback",
    "http://localhost:3000/callback"
  ]
  token_endpoint_auth_method = "none"
  scope                      = "openid profile email"
}

# Device Authorization flow (CLI tools, IoT devices)
resource "ory_oauth2_client" "cli_tool" {
  client_name                = "CLI Tool"
  grant_types                = ["urn:ietf:params:oauth:grant-type:device_code", "refresh_token"]
  response_types             = ["code"]
  token_endpoint_auth_method = "none"
  scope                      = "openid offline_access"
}

output "api_service_client_id" {
  value = ory_oauth2_client.api_service.client_id
}

output "api_service_client_secret" {
  value     = ory_oauth2_client.api_service.client_secret
  sensitive = true
}
