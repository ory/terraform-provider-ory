# Google Sign-In
resource "ory_social_provider" "google" {
  provider_id   = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scopes        = ["email", "profile"]
}

# GitHub
resource "ory_social_provider" "github" {
  provider_id   = "github"
  client_id     = var.github_client_id
  client_secret = var.github_client_secret
  scopes        = ["user:email", "read:user"]
}

# Microsoft Azure AD
resource "ory_social_provider" "microsoft" {
  provider_id   = "microsoft"
  client_id     = var.azure_client_id
  client_secret = var.azure_client_secret
  tenant        = var.azure_tenant_id # or "common" for multi-tenant
  scopes        = ["openid", "profile", "email"]
}

# Generic OIDC Provider
resource "ory_social_provider" "corporate_sso" {
  provider_id    = "generic"
  provider_label = "Corporate SSO"
  client_id      = var.sso_client_id
  client_secret  = var.sso_client_secret
  issuer_url     = "https://sso.example.com"
  scopes         = ["openid", "profile", "email"]
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

variable "sso_client_id" {
  type = string
}

variable "sso_client_secret" {
  type      = string
  sensitive = true
}
