# Machine-to-machine client (Client Credentials flow)
resource "ory_oauth2_client" "api_service" {
  client_name                = "API Service"
  grant_types                = ["client_credentials"]
  token_endpoint_auth_method = "client_secret_post"
  scope                      = "read write admin"
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

  # Client metadata URIs
  client_uri = "https://app.example.com"
  logo_uri   = "https://app.example.com/logo.png"
  policy_uri = "https://app.example.com/privacy"
  tos_uri    = "https://app.example.com/terms"

  # OIDC logout
  frontchannel_logout_uri = "https://app.example.com/logout/frontchannel"
  backchannel_logout_uri  = "https://app.example.com/logout/backchannel"

  # Per-client CORS
  allowed_cors_origins = [
    "https://app.example.com",
    "https://admin.example.com"
  ]
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

output "api_service_client_id" {
  value = ory_oauth2_client.api_service.client_id
}

output "api_service_client_secret" {
  value     = ory_oauth2_client.api_service.client_secret
  sensitive = true
}
