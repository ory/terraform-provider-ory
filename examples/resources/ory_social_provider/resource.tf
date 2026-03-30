# Google Sign-In
resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
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
