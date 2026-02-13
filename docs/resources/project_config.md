---
page_title: "ory_project_config Resource - ory"
subcategory: ""
description: |-
  Configures an Ory Network project's settings.
---

# ory_project_config (Resource)

Configures an Ory Network project's settings.

This resource manages the configuration of an Ory Network project, including authentication methods,
password policies, session settings, CORS, and more.

This resource supports drift detection — `terraform plan` will detect changes made outside of Terraform (e.g., via Ory Console or API) for any attributes you have configured.

~> **Note:** Only attributes present in your Terraform configuration are tracked for drift. Attributes you have not configured will not appear in plan output, even if they have non-default values in the API.

## Example Usage

```terraform
# Basic project configuration
resource "ory_project_config" "basic" {
  cors_enabled        = true
  cors_origins        = ["https://app.example.com"]
  password_min_length = 10
  session_lifespan    = "720h" # 30 days
}

# Full security configuration
resource "ory_project_config" "secure" {
  # Public CORS
  cors_enabled = true
  cors_origins = ["https://app.example.com", "https://admin.example.com"]

  # Admin CORS
  cors_admin_enabled = true
  cors_admin_origins = ["https://admin.example.com"]

  # Sessions
  session_lifespan          = "168h" # 7 days
  session_cookie_same_site  = "Strict"
  session_cookie_persistent = true

  # Password Policy
  password_min_length            = 12
  password_identifier_similarity = true
  password_check_haveibeenpwned  = true
  password_max_breaches          = 0

  # Authentication Methods
  enable_password = true
  enable_code     = true
  enable_passkey  = true

  # Flow Controls
  enable_registration = true
  enable_recovery     = true
  enable_verification = true

  # MFA
  enable_totp              = true
  totp_issuer              = "MyApp"
  enable_webauthn          = true
  webauthn_rp_display_name = "MyApp"
  webauthn_rp_id           = "app.example.com"
  webauthn_rp_origins      = ["https://app.example.com"]
  webauthn_passwordless    = true
  enable_lookup_secret     = true
  mfa_enforcement          = "optional"
  required_aal             = "aal1"

  # URLs
  default_return_url = "https://app.example.com/dashboard"
  allowed_return_urls = [
    "https://app.example.com/dashboard",
    "https://app.example.com/settings"
  ]

  # Account Experience Branding
  account_experience_name           = "MyApp"
  account_experience_logo_url       = "https://cdn.example.com/logo.png"
  account_experience_favicon_url    = "https://cdn.example.com/favicon.ico"
  account_experience_default_locale = "en"

  # OAuth2 Token Lifespans
  oauth2_access_token_lifespan  = "1h"
  oauth2_refresh_token_lifespan = "720h"

  # Keto Namespaces (for fine-grained authorization)
  keto_namespaces = ["documents", "folders", "groups"]
}

# Self-hosted UI configuration (custom login/registration pages)
resource "ory_project_config" "self_hosted_ui" {
  login_ui_url        = "https://auth.example.com/login"
  registration_ui_url = "https://auth.example.com/registration"
  recovery_ui_url     = "https://auth.example.com/recovery"
  verification_ui_url = "https://auth.example.com/verification"
  settings_ui_url     = "https://auth.example.com/settings"
  error_ui_url        = "https://auth.example.com/error"

  enable_password     = true
  enable_registration = true
  enable_recovery     = true
  enable_verification = true
}

# SMTP configuration for custom email delivery
resource "ory_project_config" "with_smtp" {
  smtp_connection_uri = var.smtp_connection_uri
  smtp_from_address   = "noreply@example.com"
  smtp_from_name      = "MyApp"
  smtp_headers = {
    "X-SES-CONFIGURATION-SET" = "my-config-set"
  }

  enable_password = true
}

variable "smtp_connection_uri" {
  type        = string
  sensitive   = true
  description = "SMTP connection URI (e.g., smtps://user:pass@smtp.example.com:465)"
}
```

## Duration Format

Time-based attributes use Go duration strings. Examples:

| Duration | Meaning |
|----------|---------|
| `30m` | 30 minutes |
| `1h` | 1 hour |
| `24h0m0s` | 24 hours |
| `168h` | 7 days |
| `720h` | 30 days |
| `8760h` | 365 days |

## Import

Import using the project ID:

```shell
terraform import ory_project_config.main <project-id>
```

### Avoiding "Forces Replacement" After Import

After importing, if Terraform shows `project_id forces replacement`, ensure your configuration matches:

**Option 1: Explicit project_id**
```hcl
resource "ory_project_config" "main" {
  project_id = "the-exact-project-id-you-imported"
  # ... other settings
}
```

**Option 2: Use provider default** (recommended)
```hcl
provider "ory" {
  project_id = "the-exact-project-id-you-imported"
}

resource "ory_project_config" "main" {
  # project_id inherits from provider
  # ... other settings
}
```

## CORS Configuration

This resource supports CORS configuration for both public and admin endpoints:

- **Public CORS** (`cors_enabled`, `cors_origins`) — Controls CORS for public-facing endpoints (login, registration, etc.)
- **Admin CORS** (`cors_admin_enabled`, `cors_admin_origins`) — Controls CORS for admin API endpoints

## Notes

- Project config cannot be deleted — it always exists for a project
- Deleting this resource from Terraform state does not reset the project configuration
- The `project_id` attribute forces replacement if changed (you cannot move config to a different project)
- After `terraform import`, run `terraform plan` to reconcile your configuration with the current API state

## Coverage and Limitations

This resource exposes **60+ attributes** across 11 configuration categories:

| Category | Examples |
|----------|---------|
| Password settings | min length, identifier similarity, max breaches, haveibeenpwned |
| Session settings | cookie same site, lifespan, whoami-required AAL |
| CORS | public and admin origins, enabled/disabled |
| Authentication | passwordless, code, TOTP, passkey, WebAuthn, lookup secrets |
| Recovery / Verification | enabled, methods, notify unknown recipients |
| Account enumeration | mitigation enabled |
| Keto | namespace configuration |

### Not Yet Exposed

Some Ory project settings are not yet available through this resource. For settings not listed above, use one of these workarounds:

- **Ory Console** — [console.ory.sh](https://console.ory.sh)
- **Ory CLI** — `ory patch project --replace '/services/identity/config/...'`
- **API** — `PATCH /projects/{project_id}` with JSON Patch operations

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `account_experience_default_locale` (String) Default locale for the hosted login UI (e.g., 'en', 'de').
- `account_experience_favicon_url` (String) URL for the favicon in the hosted login UI.
- `account_experience_logo_url` (String) URL for the logo in the hosted login UI.
- `account_experience_name` (String) Application name shown in the hosted login UI.
- `account_experience_stylesheet` (String) Custom CSS stylesheet for the hosted login UI.
- `allowed_return_urls` (List of String) List of allowed return URLs.
- `cors_admin_enabled` (Boolean) Enable CORS for the admin API.
- `cors_admin_origins` (List of String) Allowed CORS origins for the admin API.
- `cors_enabled` (Boolean) Enable CORS for the public API.
- `cors_origins` (List of String) Allowed CORS origins.
- `default_return_url` (String) Default URL to redirect after flows.
- `enable_code` (Boolean) Enable code-based authentication.
- `enable_lookup_secret` (Boolean) Enable backup/recovery codes.
- `enable_passkey` (Boolean) Enable Passkey authentication.
- `enable_password` (Boolean) Enable password authentication.
- `enable_recovery` (Boolean) Enable password recovery flow.
- `enable_registration` (Boolean) Enable user registration.
- `enable_totp` (Boolean) Enable TOTP (Time-based One-Time Password).
- `enable_verification` (Boolean) Enable email verification flow.
- `enable_webauthn` (Boolean) Enable WebAuthn (hardware keys).
- `error_ui_url` (String) URL for the error UI.
- `keto_namespaces` (List of String) List of Keto namespace names to configure for Ory Permissions. Namespaces define the types of resources in your permission model (e.g., 'documents', 'folders'). Each namespace name must be unique.
- `login_ui_url` (String) URL for the login UI.
- `mfa_enforcement` (String) MFA enforcement level: 'none', 'optional', or 'required'.
- `oauth2_access_token_lifespan` (String) OAuth2 access token lifespan (e.g., '1h', '30m'). Requires Hydra service.
- `oauth2_refresh_token_lifespan` (String) OAuth2 refresh token lifespan (e.g., '720h' for 30 days). Requires Hydra service.
- `password_check_haveibeenpwned` (Boolean) Check passwords against HaveIBeenPwned.
- `password_identifier_similarity` (Boolean) Check password similarity to identifier.
- `password_max_breaches` (Number) Maximum allowed breaches in HaveIBeenPwned.
- `password_min_length` (Number) Minimum password length.
- `project_id` (String) Project ID to configure. If not set, uses provider's project_id.
- `recovery_ui_url` (String) URL for the password recovery UI.
- `registration_ui_url` (String) URL for the registration UI.
- `required_aal` (String) Required Authenticator Assurance Level for protected resources: 'aal1' or 'aal2'.
- `session_cookie_persistent` (Boolean) Enable persistent session cookies (survive browser close).
- `session_cookie_same_site` (String) SameSite cookie attribute (Lax, Strict, None).
- `session_lifespan` (String) Session duration (e.g., '24h0m0s').
- `session_whoami_required_aal` (String) Required AAL for session whoami endpoint: 'aal1', 'aal2', or 'highest_available'.
- `settings_ui_url` (String) URL for the account settings UI.
- `smtp_connection_uri` (String, Sensitive) SMTP connection URI for sending emails.
- `smtp_from_address` (String) Email address to send from.
- `smtp_from_name` (String) Name to display as sender.
- `smtp_headers` (Map of String) Custom headers to include in emails.
- `totp_issuer` (String) TOTP issuer name shown in authenticator apps.
- `verification_ui_url` (String) URL for the verification UI.
- `webauthn_passwordless` (Boolean) Enable passwordless WebAuthn authentication.
- `webauthn_rp_display_name` (String) WebAuthn Relying Party display name.
- `webauthn_rp_id` (String) WebAuthn Relying Party ID (typically your domain).
- `webauthn_rp_origins` (List of String) Allowed WebAuthn origins.

### Read-Only

- `id` (String) Resource ID (same as project_id).
