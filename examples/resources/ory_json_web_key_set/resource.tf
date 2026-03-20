# RSA signing key set (project_id from provider config)
resource "ory_json_web_key_set" "signing" {
  set_id    = "token-signing-keys"
  key_id    = "rsa-sig-1"
  algorithm = "RS256"
  use       = "sig"
}

# ECDSA signing key set with explicit project_id
resource "ory_json_web_key_set" "ecdsa_signing" {
  project_id = var.ory_project_id
  set_id     = "ecdsa-signing-keys"
  key_id     = "ec-sig-1"
  algorithm  = "ES256"
  use        = "sig"
}

# Encryption key set
resource "ory_json_web_key_set" "encryption" {
  set_id    = "encryption-keys"
  key_id    = "rsa-enc-1"
  algorithm = "RS256"
  use       = "enc"
}

output "signing_key_set_id" {
  value = ory_json_web_key_set.signing.id
}

# Access the full JWKS (includes private key material)
output "signing_jwks" {
  value     = ory_json_web_key_set.signing.keys
  sensitive = true
}

# Same-apply: Create project and JWK set together
# Use resource-level credentials when the project doesn't exist yet
resource "ory_json_web_key_set" "same_apply" {
  project_slug    = ory_project.main.slug
  project_api_key = ory_project_api_key.main.value

  set_id    = "token-signing-keys"
  key_id    = "rsa-sig-1"
  algorithm = "RS256"
  use       = "sig"
}
