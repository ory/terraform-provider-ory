# Look up an OAuth2 client by client ID
data "ory_oauth2_client" "app" {
  id = "existing-client-id"
}

output "client_name" {
  value = data.ory_oauth2_client.app.client_name
}

output "grant_types" {
  value = data.ory_oauth2_client.app.grant_types
}
