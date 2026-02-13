# Google Sign-In
resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
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

# Apple Sign-In
resource "ory_social_provider" "apple" {
  provider_id   = "apple"
  provider_type = "apple"
  client_id     = var.apple_client_id
  client_secret = var.apple_client_secret
  scope         = ["email", "name"]
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

variable "apple_client_id" {
  type = string
}

variable "apple_client_secret" {
  type      = string
  sensitive = true
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
