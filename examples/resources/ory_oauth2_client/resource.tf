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

# Client with custom token lifespans
resource "ory_oauth2_client" "api_gateway" {
  client_name = "API Gateway"
  grant_types = ["client_credentials"]
  scope       = "api:read api:write"

  # Short-lived access tokens for M2M
  client_credentials_grant_access_token_lifespan = "15m"

  # Logout session tracking
  backchannel_logout_uri              = "https://gateway.example.com/logout"
  backchannel_logout_session_required = true
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

# Client with inline JWKS for private_key_jwt authentication
resource "ory_oauth2_client" "with_jwks" {
  client_name                = "Service with JWKS"
  grant_types                = ["client_credentials"]
  token_endpoint_auth_method = "private_key_jwt"
  scope                      = "api:read api:write"

  jwks = jsonencode({
    keys = [
      {
        kid = "my-signing-key"
        kty = "RSA"
        alg = "RS256"
        use = "sig"
        e   = "AQAB"
        n   = "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
      }
    ]
  })
}

# Device Authorization flow (CLI tools, IoT devices)
resource "ory_oauth2_client" "cli_tool" {
  client_name                = "CLI Tool"
  grant_types                = ["urn:ietf:params:oauth:grant-type:device_code", "refresh_token"]
  response_types             = ["code"]
  token_endpoint_auth_method = "none"
  scope                      = "openid offline_access"
}

# Same-apply: Create project and OAuth2 client together
# Use resource-level credentials when the project doesn't exist yet
resource "ory_oauth2_client" "same_apply" {
  project_slug    = ory_project.main.slug
  project_api_key = ory_project_api_key.main.value

  client_name                = "Created with Project"
  grant_types                = ["client_credentials"]
  token_endpoint_auth_method = "client_secret_post"
  scope                      = "api:read api:write"
}

output "api_service_client_id" {
  value = ory_oauth2_client.api_service.client_id
}

output "api_service_client_secret" {
  value     = ory_oauth2_client.api_service.client_secret
  sensitive = true
}
