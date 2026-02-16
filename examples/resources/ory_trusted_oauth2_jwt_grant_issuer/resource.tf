# Trust an external identity provider for JWT Bearer grant
resource "ory_trusted_oauth2_jwt_grant_issuer" "idp" {
  issuer     = "https://jwt-idp.example.com"
  scope      = ["openid", "offline_access"]
  expires_at = "2027-12-31T23:59:59Z"
  subject    = "service-account@example.com"

  jwk = jsonencode({
    kty = "RSA"
    kid = "my-key-id"
    alg = "RS256"
    use = "sig"
    n   = "..."
    e   = "AQAB"
  })
}
