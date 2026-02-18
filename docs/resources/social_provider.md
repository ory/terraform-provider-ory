---
page_title: "ory_social_provider Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Network social sign-in provider (Google, GitHub, etc.).
---

# ory_social_provider (Resource)

Manages an Ory Network social sign-in provider (Google, GitHub, etc.).

Social providers are configured as part of the project's OIDC authentication method. Each provider is identified by a unique `provider_id` that is used in callback URLs.

-> **Plan:** Available on all Ory Network plans.

## Provider Types

The `provider_type` attribute determines which OAuth2/OIDC integration to use:

| Value | Description |
|-------|-------------|
| `google` | Google Sign-In |
| `github` | GitHub |
| `microsoft` | Microsoft / Azure AD (use `tenant` attribute) |
| `apple` | Apple Sign-In |
| `discord` | Discord |
| `facebook` | Facebook |
| `gitlab` | GitLab |
| `slack` | Slack |
| `spotify` | Spotify |
| `twitch` | Twitch |
| `generic` | Generic OIDC provider (requires `issuer_url`) |

~> **Note:** When using `provider_type = "generic"`, you **must** set `issuer_url` to the OIDC issuer URL. The provider uses OIDC discovery to find authorization and token endpoints automatically.

## Example Usage

```terraform
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
```

## Mapper URL

The `mapper_url` attribute controls how OIDC claims are mapped to Ory identity traits. It accepts:

- A URL pointing to a hosted Jsonnet file
- A base64-encoded Jsonnet template prefixed with `base64://`

If not set, the provider uses a default mapper that extracts the email claim.

~> **Note:** The `mapper_url` value may be transformed by the API (e.g., stored as a GCS URL). The provider only tracks this field if you explicitly set it in your configuration to avoid false drift detection.

## Important Behaviors

- **`provider_id` and `provider_type` cannot be changed** after creation. Changing either forces a new resource.
- **`client_secret` is write-only.** The API does not return secrets on read, so Terraform cannot detect external changes to the secret.
- **`tenant` maps to `microsoft_tenant`** in the Ory API. This is only used with `provider_type = "microsoft"`.
- **Deleting the last provider** resets the entire OIDC configuration to a disabled state with an empty providers array.

## Import

Import using the provider ID:

```shell
terraform import ory_social_provider.google google
```

The `provider_id` is the unique identifier you chose when creating the provider. After import, you must provide `client_secret` in your configuration since it cannot be read from the API.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `client_id` (String) OAuth2 client ID from the provider.
- `client_secret` (String, Sensitive) OAuth2 client secret from the provider.
- `provider_id` (String) Unique identifier for the provider (used in callback URLs).
- `provider_type` (String) Provider type (google, github, microsoft, apple, generic, etc.).

### Optional

- `auth_url` (String) Custom authorization URL (for non-standard providers).
- `issuer_url` (String) OIDC issuer URL (required for generic providers).
- `mapper_url` (String) Jsonnet mapper URL for claims mapping. Can be a URL or base64-encoded Jsonnet (base64://...). If not set, a default mapper that extracts email from claims will be used.
- `project_id` (String) Project ID. If not set, uses provider's project_id.
- `scope` (List of String) OAuth2 scopes to request.
- `tenant` (String) Tenant ID (for Microsoft/Azure providers).
- `token_url` (String) Custom token URL (for non-standard providers).

### Read-Only

- `id` (String) Resource ID (same as provider_id).
