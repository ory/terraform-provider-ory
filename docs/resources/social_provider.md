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

## Ephemeral (Write-Only) Credentials

In addition to `client_id`, `client_secret`, and `apple_private_key`, this resource
exposes **write-only** equivalents that are never written to Terraform state or
plan (Terraform 1.11+ [write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral/write-only)):

| Write-only attribute | Replaces | Version trigger |
|----------------------|----------|-----------------|
| `client_id_wo` | `client_id` | `client_id_wo_version` |
| `client_secret_wo` | `client_secret` | `client_secret_wo_version` |
| `apple_private_key_wo` | `apple_private_key` | `apple_private_key_wo_version` |

Each write-only attribute is mutually exclusive with its stateful counterpart. Use
them to feed credentials from an ephemeral source such as a Vault secret without
persisting them in state. Because write-only values are never stored, Terraform
cannot detect when the value changes on its own — change the matching
`*_wo_version` whenever the secret rotates so Terraform sends the new value to Ory.

```hcl
ephemeral "vault_kv_secret_v2" "google_oauth" {
  mount = "secret"
  name  = "ory/google-oauth"
}

resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"

  client_id_wo             = ephemeral.vault_kv_secret_v2.google_oauth.data["client_id"]
  client_id_wo_version     = "1"
  client_secret_wo         = ephemeral.vault_kv_secret_v2.google_oauth.data["client_secret"]
  client_secret_wo_version = "1"

  scope = ["email", "profile"]
}
```

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

# Google Sign-In that refreshes identity traits from OIDC claims on every login
resource "ory_social_provider" "google_sync_on_login" {
  provider_id   = "google-sync"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]

  # "automatic" re-runs the claims mapper on each login and updates the identity.
  # Omit or set "never" (the default) to keep the identity unchanged after sign-up.
  update_identity_on_login = "automatic"
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
```

## Mapper URL

The `mapper_url` attribute controls how OIDC claims are mapped to Ory identity traits. It accepts:

- A URL pointing to a hosted Jsonnet file
- A base64-encoded Jsonnet template prefixed with `base64://`

If not set, the provider uses a default mapper that extracts the email claim.

~> **Note:** Ory rewrites the `mapper_url` you configure into an opaque, content-addressed URL (for example `https://storage.googleapis.com/.../<hash>.jsonnet`). The provider preserves the value exactly as you wrote it in your configuration and does **not** read the transformed value back into state, so a configured `mapper_url` no longer produces a perpetual diff. Because the transformed URL cannot be reversed, `mapper_url` is not populated on `terraform import`; add it back to your configuration after importing.

## Label

The `label` attribute sets a human-readable label for the provider. This is displayed on the login button in the default UI (e.g., "Sign in with Corporate SSO"). If not set, the default label for the provider type is used.

## Account Linking Mode

The `account_linking_mode` attribute controls how accounts are linked when a user signs in with this provider and a matching identity already exists:

| Value | Description |
|-------|-------------|
| `automatic` | Automatically links the social sign-in to the existing identity without user interaction. |
| `confirm_with_existing_credential` | Requires the user to verify ownership of the existing account before linking (default behavior). |

```hcl
resource "ory_social_provider" "google" {
  provider_id          = "google"
  provider_type        = "google"
  client_id            = var.google_client_id
  client_secret        = var.google_client_secret
  scope                = ["email", "profile"]
  label                = "Sign in with Corporate Google"
  account_linking_mode = "automatic"
}
```

~> **Note:** `account_linking_mode = "automatic"` is different from `auto_link = true`. The `account_linking_mode` is returned by the API and can be used independently, while `auto_link` is write-only and requires `enable_oidc_auto_link_policy` to be enabled at the project level.

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

## Upstream MFA (AAL2 Elevation)

When an upstream OpenID Connect provider performs multi-factor authentication, the resulting Ory session can be marked as AAL2 (Authentication Assurance Level 2) so the user is not prompted for a second factor again. Use `aal2_acr_values` and/or `aal2_amr_values` to opt in:

- **`aal2_acr_values`** — A list of `acr` claim values. If the ID token returned by the upstream provider contains an `acr` claim matching any value in the list, Ory marks the session as AAL2. Works with providers that return the `acr` claim (Auth0, Okta, Keycloak, PingFederate, Entra ID v1, generic enterprise IdPs).
- **`aal2_amr_values`** — A list of `amr` values (per [RFC 8176](https://datatracker.ietf.org/doc/html/rfc8176), for example `mfa`, `otp`, `hwk`, `fpt`). If the upstream `amr` array contains any value in the list, Ory marks the session as AAL2.

Both attributes are optional and can be used together. Leave them unset to always issue AAL1 sessions through the provider.

```hcl
resource "ory_social_provider" "enterprise_sso" {
  provider_id   = "enterprise-sso"
  provider_type = "generic"
  client_id     = var.sso_client_id
  client_secret = var.sso_client_secret
  issuer_url    = "https://sso.example.com"
  scope         = ["openid", "profile", "email"]

  aal2_acr_values = ["urn:mace:incommon:iap:silver", "https://refeds.org/profile/mfa"]
  aal2_amr_values = ["mfa", "otp", "hwk"]
}
```

## PKCE

The `pkce` attribute controls whether the OAuth2 authorization code flow uses [Proof Key for Code Exchange (RFC 7636)](https://datatracker.ietf.org/doc/html/rfc7636) when redirecting users to the upstream provider:

| Value | Description |
|-------|-------------|
| `auto` (default) | Enable PKCE only when the upstream provider advertises support via OIDC discovery. |
| `force` | Always send the PKCE challenge. Use only with providers known to support it — providers that do not understand PKCE may reject the authorization request. |
| `never` | Disable PKCE for this provider. |

```hcl
resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]
  pkce          = "force"
}
```

## FedCM (Federated Credential Management)

The `fedcm_config_url` attribute enables sign-in with this provider through the browser's [FedCM API](https://developer.mozilla.org/en-US/docs/Web/API/FedCM_API) instead of a full-page OAuth2 redirect. Set it to the URL of the provider's FedCM configuration file. For Google, this is `https://accounts.google.com/gsi/fedcm.json`.

```hcl
resource "ory_social_provider" "google" {
  provider_id      = "google"
  provider_type    = "google"
  client_id        = var.google_client_id
  client_secret    = var.google_client_secret
  scope            = ["email", "profile"]
  fedcm_config_url = "https://accounts.google.com/gsi/fedcm.json"
}
```

Leave the attribute unset to disable FedCM for the provider. Removing it from your configuration clears the value server-side.

## Update Identity on Login

The `update_identity_on_login` attribute controls whether an identity's traits and metadata are refreshed from the upstream OIDC claims every time the user signs in:

| Value | Description |
|-------|-------------|
| `never` (default) | The identity is populated from the claims mapper on first sign-up and left unchanged on subsequent logins. |
| `automatic` | Ory re-runs the Jsonnet claims mapper on every login and updates the identity's traits and metadata to match the latest upstream claims. |

```hcl
resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scope         = ["email", "profile"]

  update_identity_on_login = "automatic"
}
```

Leave the attribute unset to use the Ory default (`never`). The API accepts only the values `never` and `automatic`.

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
- **`client_secret` is not returned on read.** The API does not return secrets, so Terraform cannot detect external changes to the secret. (For a value that is never stored in state at all, use the write-only `client_secret_wo` — see [Ephemeral (Write-Only) Credentials](#ephemeral-write-only-credentials).)
- **`tenant` maps to `microsoft_tenant`** in the Ory API. This is only used with `provider_type = "microsoft"`.
- **Apple-specific fields** (`apple_team_id`, `apple_private_key_id`, `apple_private_key`) are only valid with `provider_type = "apple"`. The `apple_private_key` is not returned by the API on read.
- **Deleting the last provider** resets the entire OIDC configuration to a disabled state with an empty providers array.

## Import

Import using the provider ID:

```shell
terraform import ory_social_provider.google google
```

The `provider_id` is the unique identifier you chose when creating the provider. After import, you must provide the secret credentials in your configuration since they cannot be read from the API (either the stateful attributes below or their `_wo` write-only equivalents):

- **Non-Apple providers:** Set `client_secret` (or `client_secret_wo`).
- **Apple providers:** Set either `client_secret` (pre-generated JWT) or all three Apple-specific fields (`apple_team_id`, `apple_private_key_id`, and `apple_private_key` / `apple_private_key_wo`).

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `provider_id` (String) Unique identifier for the provider (used in callback URLs).
- `provider_type` (String) Provider type (google, github, microsoft, apple, generic, etc.).

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `aal2_acr_values` (List of String) Upstream OpenID Connect `acr` claim values that elevate the resulting Ory session to AAL2. If the ID token returned by the upstream provider contains an `acr` claim matching any of these values, the user is not prompted for a second factor. Leave unset to always issue AAL1 sessions through this provider. Works with providers that return the `acr` claim (Auth0, Okta, Keycloak, PingFederate, Entra ID v1, generic enterprise IdPs).
- `aal2_amr_values` (List of String) Upstream OpenID Connect `amr` values (per RFC 8176, for example `mfa`, `otp`, `hwk`) that mark the session AAL2 when they appear in the upstream `amr` array. Leave unset to ignore the `amr` claim.
- `account_linking_mode` (String) Controls how accounts are linked when a user signs in with this provider and a matching identity already exists. "automatic" links without user interaction; "confirm_with_existing_credential" requires the user to verify ownership of the existing account first.
- `additional_id_token_audiences` (List of String) Additional audiences allowed in the ID Token. Only relevant in OIDC flows that submit an ID Token directly instead of using the callback from the OIDC provider (e.g., native mobile apps signing in with Google or Apple where the app and the OIDC client are registered with different audiences).
- `apple_private_key` (String, Sensitive) Apple private key in PEM format (contents of the .p8 file). Required when provider_type is "apple" and client_secret is not set. Ory uses this to generate the JWT client secret automatically. Exactly one of apple_private_key or apple_private_key_wo may be set.
- `apple_private_key_id` (String) Apple private key ID from the Apple Developer portal (e.g., "UX56C66723"). Required when provider_type is "apple" and client_secret is not set.
- `apple_private_key_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of apple_private_key (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the Apple private key from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change apple_private_key_wo_version to rotate it. Mutually exclusive with apple_private_key.
- `apple_private_key_wo_version` (String) Version trigger for apple_private_key_wo. Change this value whenever the write-only apple_private_key_wo changes so Terraform sends the new key to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless apple_private_key_wo is set.
- `apple_team_id` (String) Apple Developer Team ID (e.g., "KP76DQS54M"). Required when provider_type is "apple" and client_secret is not set.
- `auth_url` (String) Custom authorization URL (for non-standard providers).
- `auto_link` (Boolean) Enable automatic account linking for this provider. When true, if an identity with the same identifier (e.g., email) already exists, the social sign-in will automatically link to that identity instead of failing. Requires enable_oidc_auto_link_policy to be true in the project config (ory_project_config). This attribute is write-only — the API accepts it on create/update but does not return it on read, so Terraform preserves the value from state. On import, the value will not be populated. Removing this attribute from your configuration will automatically disable auto-linking server-side.
- `base_redirect_uri` (String, Deprecated) Override the base redirect URI for OIDC callbacks (e.g., "https://iam.example.com"). When set, Ory constructs callback URLs using this base instead of the default project domain. This is a global OIDC config setting — if multiple social providers set different values, the last applied value wins.
- `client_id` (String) OAuth2 client ID from the provider. Exactly one of client_id or client_id_wo must be set.
- `client_id_wo` (String, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of client_id (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the client ID from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change client_id_wo_version to force the new value to be sent. Mutually exclusive with client_id.
- `client_id_wo_version` (String) Version trigger for client_id_wo. Change this value whenever the write-only client_id_wo changes so Terraform sends the new value to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless client_id_wo is set.
- `client_secret` (String, Sensitive) OAuth2 client secret from the provider. Required for all providers except Apple (where Ory generates the secret from apple_team_id, apple_private_key_id, and apple_private_key). Exactly one of client_secret or client_secret_wo may be set.
- `client_secret_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of client_secret (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the client secret from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change client_secret_wo_version to rotate it. Mutually exclusive with client_secret.
- `client_secret_wo_version` (String) Version trigger for client_secret_wo. Change this value whenever the write-only client_secret_wo changes so Terraform sends the new secret to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless client_secret_wo is set.
- `fedcm_config_url` (String) URL of the provider's FedCM (Federated Credential Management) configuration file. When set, Ory can use the browser's FedCM API for sign-in with this provider instead of a full-page redirect. For example, Google's FedCM configuration is served at "https://accounts.google.com/gsi/fedcm.json". Leave unset to disable FedCM for this provider.
- `issuer_url` (String) OIDC issuer URL (required for generic providers).
- `label` (String) Human-readable label for the provider, displayed on the login button (e.g., "Sign in with Corporate SSO").
- `mapper_url` (String) Jsonnet mapper URL for claims mapping. Can be a URL or base64-encoded Jsonnet (base64://...). If not set, a default mapper that extracts email from claims will be used.
- `pkce` (String) PKCE (Proof Key for Code Exchange) behavior for the OAuth2 authorization code flow. "auto" (default) enables PKCE when the upstream provider advertises support via OIDC discovery; "force" always sends PKCE (use only with providers known to support it); "never" disables PKCE.
- `project_id` (String) Project ID. If not set, uses provider's project_id.
- `scope` (List of String) OAuth2 scopes to request.
- `tenant` (String) Tenant ID (for Microsoft/Azure providers).
- `token_url` (String) Custom token URL (for non-standard providers).
- `update_identity_on_login` (String) Controls whether the identity's traits and metadata are refreshed from the upstream OIDC claims on every login. "never" (the API default) keeps the identity as-is after the initial sign-up. "automatic" re-runs the Jsonnet claims mapper on each login and updates the identity accordingly. Leave unset to use the Ory default ("never").

### Read-Only

- `id` (String) Resource ID (same as provider_id).
