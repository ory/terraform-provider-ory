# RSA signing key set
resource "ory_json_web_key_set" "signing" {
  set_id    = "token-signing-keys"
  key_id    = "rsa-sig-1"
  algorithm = "RS256"
  use       = "sig"
}

# ECDSA signing key set (smaller, faster)
resource "ory_json_web_key_set" "ecdsa_signing" {
  set_id    = "ecdsa-signing-keys"
  key_id    = "ec-sig-1"
  algorithm = "ES256"
  use       = "sig"
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
