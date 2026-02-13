resource "ory_oauth2_client" "test" {
  client_name = "[[ .Name ]]"

  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid profile email"
  redirect_uris  = ["[[ .AppURL ]]/callback"]

  skip_consent        = true
  skip_logout_consent = true
  subject_type        = "public"
  contacts            = ["admin@[[ .EmailDomain ]]", "dev@[[ .EmailDomain ]]"]
}
