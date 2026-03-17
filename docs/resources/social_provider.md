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

## Apple Sign-In

Apple uses a non-standard authentication flow. Instead of a static `client_secret`, Apple requires:

- **`apple_team_id`** — Your Apple Developer Team ID (e.g., `KP76DQS54M`)
- **`apple_private_key_id`** — The key ID from the Apple Developer portal (e.g., `UX56C66723`)
- **`apple_private_key`** — The private key in PEM format (the contents of your `.p8` file)

Ory uses these to automatically generate the JWT `client_secret` required by Apple's OAuth2 flow. You do **not** need to set `client_secret` when using Apple-specific fields.

Alternatively, you may provide a pre-generated `client_secret` directly if you prefer to manage the JWT yourself.

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

# Google Sign-In with automatic account linking
resource "ory_social_provider" "google_auto_link" {
  provider_id   = "google-auto-link"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
  auto_link     = true # Requires enable_oidc_auto_link_policy = true in ory_project_config
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
```

## Mapper URL

The `mapper_url` attribute controls how OIDC claims are mapped to Ory identity traits. It accepts:

- A URL pointing to a hosted Jsonnet file
- A base64-encoded Jsonnet template prefixed with `base64://`

If not set, the provider uses a default mapper that extracts the email claim.

~> **Note:** The `mapper_url` value may be transformed by the API (e.g., stored as a GCS URL). The provider only tracks this field if you explicitly set it in your configuration to avoid false drift detection.

## Auto-Link

The `auto_link` attribute enables automatic account linking for a specific social provider. When set to `true`, if an identity with the same identifier (e.g., email) already exists, the social sign-in will automatically link to that existing identity instead of failing with a duplicate error.

~> **Write-only attribute:** `auto_link` is accepted by the API on create/update but is **not returned** on read. Terraform preserves the value from state. This means drift cannot be detected from the API, and `terraform import` will not populate this field. Removing `auto_link` from your configuration will automatically disable auto-linking server-side.

To use auto-linking, you must also enable the auto-link policy at the project level using `enable_oidc_auto_link_policy = true` in the `ory_project_config` resource:

```hcl
resource "ory_project_config" "main" {
  enable_oidc                  = true
  enable_oidc_auto_link_policy = true
}

resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
  auto_link     = true
}
```

~> **Security:** Auto-linking trusts that the social provider has verified the user's email. Only enable this for providers you trust to verify email addresses.

## Base Redirect URI

The `base_redirect_uri` attribute overrides the base URL Ory uses when constructing OIDC callback URLs. Use this when your project is accessible under a custom domain and you want callbacks to go to that domain rather than the default Ory project URL.

```hcl
resource "ory_social_provider" "google" {
  provider_id      = "google"
  provider_type    = "google"
  client_id        = var.google_client_id
  client_secret    = var.google_client_secret
  base_redirect_uri = "https://iam.example.com"
}
```

~> **Note:** `base_redirect_uri` is a **global** OIDC configuration setting, not per-provider. If you have multiple `ory_social_provider` resources and set `base_redirect_uri` in more than one, the last applied value will take effect for all providers.

## Important Behaviors

- **`provider_id` and `provider_type` cannot be changed** after creation. Changing either forces a new resource.
- **`client_secret` is write-only.** The API does not return secrets on read, so Terraform cannot detect external changes to the secret.
- **`tenant` maps to `microsoft_tenant`** in the Ory API. This is only used with `provider_type = "microsoft"`.
- **Apple-specific fields** (`apple_team_id`, `apple_private_key_id`, `apple_private_key`) are only valid with `provider_type = "apple"`. The `apple_private_key` is write-only (not returned by API).
- **Deleting the last provider** resets the entire OIDC configuration to a disabled state with an empty providers array.

## Import

Import using the provider ID:

```shell
terraform import ory_social_provider.google google
```

The `provider_id` is the unique identifier you chose when creating the provider. After import, you must provide write-only credentials in your configuration since they cannot be read from the API:

- **Non-Apple providers:** Set `client_secret`.
- **Apple providers:** Set either `client_secret` (pre-generated JWT) or all three Apple-specific fields (`apple_team_id`, `apple_private_key_id`, and `apple_private_key`).

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `client_id` (String) OAuth2 client ID from the provider.
- `provider_id` (String) Unique identifier for the provider (used in callback URLs).
- `provider_type` (String) Provider type (google, github, microsoft, apple, generic, etc.).

### Optional

- `apple_private_key` (String, Sensitive) Apple private key in PEM format (contents of the .p8 file). Required when provider_type is "apple" and client_secret is not set. Ory uses this to generate the JWT client secret automatically.
- `apple_private_key_id` (String) Apple private key ID from the Apple Developer portal (e.g., "UX56C66723"). Required when provider_type is "apple" and client_secret is not set.
- `apple_team_id` (String) Apple Developer Team ID (e.g., "KP76DQS54M"). Required when provider_type is "apple" and client_secret is not set.
- `auth_url` (String) Custom authorization URL (for non-standard providers).
- `auto_link` (Boolean) Enable automatic account linking for this provider. When true, if an identity with the same identifier (e.g., email) already exists, the social sign-in will automatically link to that identity instead of failing. Requires enable_oidc_auto_link_policy to be true in the project config (ory_project_config). This attribute is write-only — the API accepts it on create/update but does not return it on read, so Terraform preserves the value from state. On import, the value will not be populated. Removing this attribute from your configuration will automatically disable auto-linking server-side.
- `base_redirect_uri` (String) Override the base redirect URI for OIDC callbacks (e.g., "https://iam.example.com"). When set, Ory constructs callback URLs using this base instead of the default project domain. This is a global OIDC config setting — if multiple social providers set different values, the last applied value wins.
- `client_secret` (String, Sensitive) OAuth2 client secret from the provider. Required for all providers except Apple (where Ory generates the secret from apple_team_id, apple_private_key_id, and apple_private_key).
- `issuer_url` (String) OIDC issuer URL (required for generic providers).
- `mapper_url` (String) Jsonnet mapper URL for claims mapping. Can be a URL or base64-encoded Jsonnet (base64://...). If not set, a default mapper that extracts email from claims will be used.
- `project_id` (String) Project ID. If not set, uses provider's project_id.
- `scope` (List of String) OAuth2 scopes to request.
- `tenant` (String) Tenant ID (for Microsoft/Azure providers).
- `token_url` (String) Custom token URL (for non-standard providers).

### Read-Only

- `id` (String) Resource ID (same as provider_id).
