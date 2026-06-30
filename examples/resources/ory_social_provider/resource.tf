# Google Sign-In
resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
}

# Google Sign-In with write-only (ephemeral) credentials sourced from Vault.
# Write-only arguments (Terraform 1.11+) send the value to Ory without ever
# storing it in state or plan — ideal for secrets read from an ephemeral
# resource such as Vault. Bump the matching *_wo_version whenever a secret
# changes so Terraform re-sends it (write-only values cannot be diffed).
ephemeral "vault_kv_secret_v2" "google_oauth" {
  mount = "secret"
  name  = "ory/google-oauth"
}

resource "ory_social_provider" "google_write_only" {
  provider_id   = "google-wo"
  provider_type = "google"

  client_id_wo             = ephemeral.vault_kv_secret_v2.google_oauth.data["client_id"]
  client_id_wo_version     = "1"
  client_secret_wo         = ephemeral.vault_kv_secret_v2.google_oauth.data["client_secret"]
  client_secret_wo_version = "1"

  scope = ["email", "profile"]
}

# Google Sign-In with automatic account linking
resource "ory_social_provider" "google_auto_link" {
  provider_id   = "google-auto-link"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
  auto_link     = true # Requires enable_oidc_auto_link_policy = true in ory_project_config
}

# Google Sign-In with custom label and account linking
resource "ory_social_provider" "google_labeled" {
  provider_id          = "google-labeled"
  provider_type        = "google"
  client_id            = var.google_client_id
  client_secret        = var.google_client_secret
  scope                = ["email", "profile"]
  label                = "Sign in with Corporate Google"
  account_linking_mode = "automatic"
}

# Google Sign-In with FedCM (browser Federated Credential Management)
resource "ory_social_provider" "google_fedcm" {
  provider_id      = "google-fedcm"
  provider_type    = "google"
  client_id        = var.google_client_id
  client_secret    = var.google_client_secret
  scope            = ["email", "profile"]
  fedcm_config_url = "https://accounts.google.com/gsi/fedcm.json"
}

# Generic OIDC with a custom base redirect URI (e.g., when using a custom domain)
resource "ory_social_provider" "corporate_sso_custom_domain" {
  provider_id       = "corporate-sso-custom-domain"
  provider_type     = "generic"
  client_id         = var.sso_client_id
  client_secret     = var.sso_client_secret
  issuer_url        = "https://sso.example.com"
  scope             = ["openid", "profile", "email"]
  base_redirect_uri = "https://iam.example.com"
}

# GitHub
resource "ory_social_provider" "github" {
  provider_id   = "github"
  provider_type = "github"
  client_id     = var.github_client_id
  client_secret = var.github_client_secret
  scope         = ["user:email", "read:user"]
}

# Microsoft Azure AD
resource "ory_social_provider" "microsoft" {
  provider_id   = "microsoft"
  provider_type = "microsoft"
  client_id     = var.azure_client_id
  client_secret = var.azure_client_secret
  tenant        = var.azure_tenant_id # or "common" for multi-tenant
  scope         = ["openid", "profile", "email"]
}

# Apple Sign-In (using Apple-specific credentials)
resource "ory_social_provider" "apple" {
  provider_id          = "apple"
  provider_type        = "apple"
  client_id            = var.apple_service_id
  apple_team_id        = var.apple_team_id
  apple_private_key_id = var.apple_private_key_id
  apple_private_key    = var.apple_private_key
  scope                = ["email", "name"]
}

# Enterprise SSO that elevates the Ory session to AAL2 when the upstream
# provider asserts MFA via the `acr` or `amr` claims (works with Auth0, Okta,
# Keycloak, PingFederate, Entra ID v1, and other OIDC providers).
resource "ory_social_provider" "enterprise_sso" {
  provider_id   = "enterprise-sso"
  provider_type = "generic"
  client_id     = var.sso_client_id
  client_secret = var.sso_client_secret
  issuer_url    = "https://sso.example.com"
  scope         = ["openid", "profile", "email"]

  # Mark the Ory session as AAL2 when the ID token's `acr` claim matches any of these.
  aal2_acr_values = [
    "urn:mace:incommon:iap:silver",
    "https://refeds.org/profile/mfa",
  ]

  # Mark the Ory session as AAL2 when any of these values appear in the `amr` array (per RFC 8176).
  aal2_amr_values = ["mfa", "otp", "hwk"]
}

# Generic OIDC Provider with custom claims mapping
resource "ory_social_provider" "corporate_sso" {
  provider_id   = "corporate-sso"
  provider_type = "generic"
  client_id     = var.sso_client_id
  client_secret = var.sso_client_secret
  issuer_url    = "https://sso.example.com"
  scope         = ["openid", "profile", "email"]

  # Jsonnet mapper for custom claims mapping (base64-encoded)
  mapper_url = "base64://bG9jYWwgY2xhaW1zID0gc3RkLmV4dFZhcignY2xhaW1zJyk7CnsKICBpZGVudGl0eTogewogICAgdHJhaXRzOiB7CiAgICAgIGVtYWlsOiBjbGFpbXMuZW1haWwsCiAgICB9LAogIH0sCn0="
}

# Generic OIDC with custom authorization and token URLs
resource "ory_social_provider" "custom_provider" {
  provider_id   = "custom-idp"
  provider_type = "generic"
  client_id     = var.custom_client_id
  client_secret = var.custom_client_secret
  issuer_url    = "https://idp.example.com"
  auth_url      = "https://idp.example.com/custom/authorize"
  token_url     = "https://idp.example.com/custom/token"
  scope         = ["openid", "email"]
}

variable "google_client_id" {
  type = string
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}

variable "github_client_id" {
  type = string
}

variable "github_client_secret" {
  type      = string
  sensitive = true
}

variable "azure_client_id" {
  type = string
}

variable "azure_client_secret" {
  type      = string
  sensitive = true
}

variable "azure_tenant_id" {
  type = string
}

variable "apple_service_id" {
  description = "Apple Service ID (e.g., com.example.auth)"
  type        = string
}

variable "apple_team_id" {
  description = "Apple Developer Team ID"
  type        = string
}

variable "apple_private_key_id" {
  description = "Apple private key ID from the Developer portal"
  type        = string
}

variable "apple_private_key" {
  description = "Apple private key in PEM format (.p8 file contents)"
  type        = string
  sensitive   = true
}

variable "sso_client_id" {
  type = string
}

variable "sso_client_secret" {
  type      = string
  sensitive = true
}

variable "custom_client_id" {
  type = string
}

variable "custom_client_secret" {
  type      = string
  sensitive = true
}
