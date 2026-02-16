# Register a dynamic OIDC client
# Requires dynamic client registration to be enabled on the project
resource "ory_oidc_dynamic_client" "app" {
  client_name    = "My Application"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid offline_access"
  redirect_uris  = ["https://app.example.com/callback"]
}

output "client_id" {
  value = ory_oidc_dynamic_client.app.client_id
}

output "client_secret" {
  value     = ory_oidc_dynamic_client.app.client_secret
  sensitive = true
}
