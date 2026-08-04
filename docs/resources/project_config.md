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

-> **Plan:** Available on all Ory Network plans. Some configuration options may require specific plan features.

This resource supports drift detection — `terraform plan` will detect changes made outside of Terraform (e.g., via Ory Console or API) for any attributes you have configured.

~> **Note:** Only attributes present in your Terraform configuration are tracked for drift. Attributes you have not configured will not appear in plan output, even if they have non-default values in the API.

~> **Not drift-checked:** A few attributes are accepted by the Ory API but never returned by it, so the provider keeps the configured value instead of reporting a change that no apply could settle. Removing one of these outside Terraform is not detected. They are `session_earliest_possible_extend`, `selfservice_methods_webauthn_config_rp_icon`, and `selfservice_methods_captcha_config_byo`. The same applies to secrets the API redacts, such as `smtp_connection_uri` and the courier HTTP auth credentials.

~> **Empty values:** The Ory API prunes empty values (`""`, `[]`, `{}`, and integer `0`) from the stored configuration: the apply succeeds, but the key never appears in later reads. The provider treats such a missing key as matching an empty configured value, so applying an empty value does not produce a diff on every plan. Drift is still reported whenever your configuration holds a non-empty value. A few attributes substitute a server default instead of storing the empty value — `account_experience_enabled_locales`, `selfservice_methods_passkey_config_rp_origins`, `webauthn_rp_origins` (default origins derived from the project slug), and `selfservice_methods_totp_config_issuer` (defaults to the project name). For those, plan keeps reporting the substituted default as a change, so omit the attribute instead of setting it empty.

~> **Conditionally reported:** Some attributes are only returned once the feature they belong to is configured. The `courier_smtp_*` attributes are reported after `smtp_connection_uri` is set, and the `courier_http_request_config_*` attributes after `courier_delivery_strategy = "http"`. Setting one without its prerequisite leaves the value unset on the server, which plan then reports on every run.

~> **Plan-gated features:** Some settings require a feature that your project's subscription must include, and the Ory API rejects them with an `HTTP 403 (feature_not_available)` otherwise. Notably, `selfservice_methods_oidc_enable_auto_link_policy = true` requires the OIDC auto-link policy (`use_auto_link`), an enterprise feature that Ory enables per project on request — being on an enterprise plan does not enable it automatically. If an apply fails with a `feature_not_available` error, the provider surfaces the feature name and a request ID; contact Ory support or your account executive to enable the feature.

## Example Usage

```terraform
# Basic project configuration
resource "ory_project_config" "basic" {
  cors_enabled                                            = true
  cors_origins                                            = ["https://app.example.com"]
  selfservice_methods_password_config_min_password_length = 10
  session_lifespan                                        = "720h0m0s" # 30 days
}

# Project configuration with a write-only (ephemeral) SMTP connection URI from Vault.
# smtp_connection_uri_wo is never stored in Terraform state or plan (Terraform 1.11+).
# Bump smtp_connection_uri_wo_version whenever the secret rotates so Terraform re-sends it.
ephemeral "vault_kv_secret_v2" "smtp" {
  mount = "secret"
  name  = "ory/smtp"
}

resource "ory_project_config" "with_write_only_smtp" {
  smtp_connection_uri_wo         = ephemeral.vault_kv_secret_v2.smtp.data["connection_uri"]
  smtp_connection_uri_wo_version = "1"
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
  session_lifespan                 = "168h0m0s" # 7 days
  session_earliest_possible_extend = "24h"      # Only extend sessions in the last 24h to avoid excessive writes
  session_cookie_same_site         = "Strict"
  session_cookie_persistent        = true

  # Password Policy
  selfservice_methods_password_config_min_password_length                 = 12
  selfservice_methods_password_config_identifier_similarity_check_enabled = true
  selfservice_methods_password_config_haveibeenpwned_enabled              = true
  selfservice_methods_password_config_max_breaches                        = 0

  # Authentication Methods
  selfservice_methods_password_enabled             = true
  selfservice_methods_code_enabled                 = true
  selfservice_methods_code_mfa_enabled             = true # Code as a second factor for MFA
  selfservice_methods_oidc_enabled                 = true # Required for social providers (Google, GitHub, etc.)
  selfservice_methods_oidc_enable_auto_link_policy = true # Allow social providers with auto_link to link existing identities
  selfservice_methods_passkey_enabled              = true
  selfservice_methods_profile_enabled              = true # Allow users to update profile traits via settings flow

  # Code Method Configuration
  selfservice_methods_code_config_lifespan                            = "15m0s" # How long a code remains valid
  selfservice_methods_code_config_missing_credential_fallback_enabled = true    # Use code as fallback when primary credential is missing

  # Flow Controls
  selfservice_flows_registration_enabled = true
  selfservice_flows_recovery_enabled     = true
  selfservice_flows_verification_enabled = true

  # Settings Flow
  selfservice_flows_settings_lifespan                   = "30m0s" # How long a settings flow session is valid
  selfservice_flows_settings_privileged_session_max_age = "15m0s" # Re-auth required for privileged changes after this duration

  # Verification Flow
  selfservice_flows_verification_use                       = "code"  # Use one-time code for verification (or "link")
  selfservice_flows_verification_lifespan                  = "30m0s" # How long a verification flow session is valid
  selfservice_flows_verification_notify_unknown_recipients = false   # Don't send verification emails to unknown addresses

  # MFA
  selfservice_methods_totp_enabled                    = true
  selfservice_methods_totp_config_issuer              = "MyApp"
  selfservice_methods_webauthn_enabled                = true
  selfservice_methods_webauthn_config_rp_display_name = "MyApp"
  selfservice_methods_webauthn_config_rp_id           = "app.example.com"
  webauthn_rp_origins                                 = ["https://app.example.com"]
  selfservice_methods_webauthn_config_passwordless    = true
  selfservice_methods_lookup_secret_enabled           = true
  mfa_enforcement                                     = "optional"
  selfservice_flows_settings_required_aal             = "aal1"

  # URLs
  default_return_url = "https://app.example.com/dashboard"
  allowed_return_urls = [
    "https://app.example.com/dashboard",
    "https://app.example.com/settings"
  ]

  # Account Experience Branding
  # Logos/favicons must be inline data URIs (the API does not fetch remote
  # URLs), e.g. "data:image/png;base64,${filebase64("logo.png")}".
  # Theme variables are maps of color tokens (see the AccountExperienceColors
  # API model) to CSS color values.
  account_experience_default_locale = "en"
  # account_experience_logo_light    = "data:image/png;base64,${filebase64("${path.module}/assets/logo.png")}"
  # account_experience_favicon_light = "data:image/png;base64,${filebase64("${path.module}/assets/favicon.png")}"
  # account_experience_theme_variables_light = {
  #   ax_background_default             = "#fafafa"
  #   brand_500                         = "#0066ff"
  #   button_primary_background_default = "#0066ff"
  # }

  # OAuth2 Token Lifespans
  oauth2_ttl_access_token          = "1h0m0s"
  oauth2_ttl_refresh_token         = "720h0m0s"
  oauth2_ttl_auth_code             = "30m0s"
  oauth2_ttl_id_token              = "1h0m0s"
  oauth2_ttl_login_consent_request = "30m0s"

  # OAuth2 Strategies
  oauth2_strategies_access_token    = "jwt"
  oauth2_strategies_jwt_scope_claim = "list"
  oauth2_strategies_scope           = "wildcard"

  # OAuth2 PKCE
  oauth2_pkce_enforced                    = false
  oauth2_pkce_enforced_for_public_clients = true

  # OAuth2 Claims
  oauth2_allowed_top_level_claims = ["amr", "acr"]
  oauth2_mirror_top_level_claims  = false

  # OAuth2 Issuer URL (custom issuer for OAuth2/OIDC tokens)
  oauth2_urls_self_issuer = "https://auth.example.com"

  # OAuth2 Cookie Settings
  oauth2_serve_cookies_same_site_mode              = "Strict"
  oauth2_serve_cookies_same_site_legacy_workaround = false

  # Keto Namespaces (for fine-grained authorization)
  keto_namespaces = ["documents", "folders", "groups"]
}

# Login style controls how authentication methods are presented.
# Default is "unified" (all methods on one screen).
# Use "identifier_first" to collect the identifier before showing auth methods.
resource "ory_project_config" "identifier_first" {
  selfservice_flows_login_style        = "identifier_first"
  selfservice_methods_password_enabled = true
  selfservice_methods_code_enabled     = true
}

# Self-hosted UI configuration (custom login/registration pages)
resource "ory_project_config" "self_hosted_ui" {
  selfservice_flows_login_ui_url        = "https://auth.example.com/login"
  selfservice_flows_registration_ui_url = "https://auth.example.com/registration"
  selfservice_flows_recovery_ui_url     = "https://auth.example.com/recovery"
  selfservice_flows_verification_ui_url = "https://auth.example.com/verification"
  selfservice_flows_settings_ui_url     = "https://auth.example.com/settings"
  selfservice_flows_error_ui_url        = "https://auth.example.com/error"

  selfservice_methods_password_enabled   = true
  selfservice_flows_registration_enabled = true
  selfservice_flows_recovery_enabled     = true
  selfservice_flows_verification_enabled = true
}

# SMTP configuration for custom email delivery.
#
# The URI scheme selects the security mode:
#   smtp://  -> STARTTLS (typical for port 587)
#   smtps:// -> Implicit TLS (typical for port 465)
#
# Append ?disable_starttls=true for cleartext (local dev only) or
# ?skip_ssl_verify=true to skip certificate verification.
# See the "SMTP Security Modes" section of the resource docs for details.
resource "ory_project_config" "with_smtp" {
  # STARTTLS on port 587 (recommended for most providers)
  smtp_connection_uri       = var.smtp_connection_uri
  courier_smtp_from_address = "noreply@example.com"
  courier_smtp_from_name    = "MyApp"
  smtp_headers = {
    "X-SES-CONFIGURATION-SET" = "my-config-set"
  }

  selfservice_methods_password_enabled = true
}

variable "smtp_connection_uri" {
  type      = string
  sensitive = true
  # Examples:
  #   STARTTLS:      smtp://user:pass@smtp.example.com:587
  #   Implicit TLS:  smtps://user:pass@smtp.example.com:465
  #   Cleartext:     smtp://user:pass@localhost:1025/?disable_starttls=true
  description = "SMTP connection URI. Scheme selects the security mode (smtp:// = STARTTLS, smtps:// = implicit TLS)."
}

# Native-only flows: explicitly clear browser return URLs
# Useful when the project only supports native (mobile/CLI) login flows
# and should not have any browser redirect URLs configured.
resource "ory_project_config" "native_only" {
  default_return_url  = ""
  allowed_return_urls = []

  selfservice_methods_password_enabled = true
  selfservice_methods_code_enabled     = true
}

# Session tokenizer templates (JWT tokenization for /sessions/whoami)
resource "ory_project_config" "with_tokenizer" {
  session_tokenizer_templates = {
    my_jwt = {
      ttl               = "1h"
      jwks_url          = "base64://eyJrZXlzIjpbXX0="
      claims_mapper_url = "base64://bG9jYWwgcGF5bG9hZCA9IHN0ZC5leHRWYXIoJ3BheWxvYWQnLCB7fSk7CnsKICBzZXNzaW9uX2lkOiBwYXlsb2FkLnNlc3Npb24uaWQsCn0="
      subject_source    = "id"
    }
    short_lived = {
      ttl      = "5m"
      jwks_url = "base64://eyJrZXlzIjpbXX0="
    }
  }
}

# Courier HTTP delivery (webhook-based email/SMS delivery)
resource "ory_project_config" "with_courier_http" {
  courier_delivery_strategy = "http"

  courier_http_request_config = {
    url    = "https://mail-api.example.com/send"
    method = "POST"
    body   = "base64://ewogICJyZWNpcGllbnQiOiAge3sgLnJlY2lwaWVudCB9fSwKICAiYm9keSI6IHt7IC5ib2R5IH19Cn0="
    auth = {
      type     = "basic_auth"
      user     = "mailuser"
      password = var.mail_password
    }
  }

  # Per-channel delivery (e.g., SMS via Twilio)
  courier_channels = [
    {
      id = "sms"
      request_config = {
        url    = "https://sms-api.example.com/send"
        method = "POST"
        body   = "base64://ewogICJ0byI6IHt7IC5yZWNpcGllbnQgfX0sCiAgIm1lc3NhZ2UiOiB7eyAuYm9keSB9fQp9"
        auth = {
          type  = "api_key"
          name  = "Authorization"
          value = var.sms_api_key
          in    = "header"
        }
      }
    }
  ]
}

locals {
  courier_body = <<-JSONNET
    {
      "recipient": {{ .recipient }},
      "subject": {{ .subject }},
      "body": {{ .body }}
    }
  JSONNET
}

# The same courier HTTP delivery through the flat attributes. The body must be a
# `base64://` value: the Ory API rejects a plain Jsonnet string. The API stores
# the decoded payload and reports a storage URL, and the provider resolves that
# URL back to this value, so plan stays empty while the payload matches.
resource "ory_project_config" "with_courier_http_flat" {
  courier_delivery_strategy       = "http"
  courier_http_request_config_url = "https://mail-api.example.com/send"

  courier_http_request_config_body = "base64://${base64encode(local.courier_body)}"

  courier_http_request_config_auth_basic_auth_user     = "mailuser"
  courier_http_request_config_auth_basic_auth_password = var.mail_password
}

variable "mail_password" {
  type        = string
  sensitive   = true
  description = "Password for courier HTTP basic auth"
}

variable "sms_api_key" {
  type        = string
  sensitive   = true
  description = "API key for SMS delivery service"
}

# OAuth2 token hook with API key authentication
resource "ory_project_config" "with_token_hook" {
  oauth2_token_hook = "https://example.com/token-hook"

  oauth2_token_hook_auth = {
    type  = "api_key"
    name  = "X-Api-Key"
    value = var.token_hook_api_key
    in    = "header"
  }
}

variable "token_hook_api_key" {
  type        = string
  sensitive   = true
  description = "API key sent to the OAuth2 token hook endpoint."
}

# Show the verification UI after registration and profile updates.
# Each attribute toggles the `show_verification_ui` hook for one flow while
# preserving any other hooks (e.g. `session`, `organization`) already set on
# the project.
resource "ory_project_config" "show_verification_ui" {
  selfservice_flows_verification_enabled = true

  # After password registration, redirect users to the verification UI.
  selfservice_flows_registration_after_password_hook_show_verification_ui = true
  # Same for social (OIDC) registration.
  selfservice_flows_registration_after_oidc_hook_show_verification_ui = true
  # After a profile settings update, redirect users to the verification UI.
  # This only controls the redirect. To gate the address change itself on
  # verification, see selfservice_flows_settings_after_profile_hook_verify_new_address
  # below.
  selfservice_flows_settings_after_profile_hook_show_verification_ui = true
}

# The three toggles on the Ory Console "Email verification" page. Like the
# hooks above, each one is an entry in a flow's hooks array, so other hooks at
# the same path are preserved.
resource "ory_project_config" "email_verification_hooks" {
  selfservice_flows_verification_enabled = true

  # "Require verified address for login": block sign-in until the identity has
  # a verified address. feature_flags_legacy_require_verified_login_error only
  # changes how the failure is reported, it does not enable the check.
  #
  # The Ory Console toggle writes the password flow only. Set the OIDC flow too
  # to cover social logins.
  selfservice_flows_login_after_password_hook_require_verified_address = true
  selfservice_flows_login_after_oidc_hook_require_verified_address     = true

  # "Verify new addresses": require verification of an address the user just
  # added or changed in profile settings.
  selfservice_flows_settings_after_profile_hook_verify_new_address = true

  # "Notify previous addresses": email the identity's previous addresses when
  # its verified addresses change. Recipients is one of "removed" (the Ory
  # default), "all_verified", or "all".
  selfservice_flows_settings_after_profile_hook_notify_previous_addresses            = true
  selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients = "all_verified"
}

# Automatically sign users in after they register with email + password.
# OIDC, WebAuthn and Passkey flows already issue a session on registration,
# so this toggle only affects the password flow — it mirrors the Ory Console
# "Enable sign in after registration" toggle. Existing hooks at the same path
# (e.g. `organization`) are preserved.
resource "ory_project_config" "sign_in_after_registration" {
  selfservice_flows_registration_after_password_hook_session = true
}
```

## Courier HTTP Request Body

Set `courier_http_request_config_body` to a `base64://` value that carries the Jsonnet template inline. The Ory API rejects a plain Jsonnet string, and it rejects a URL outside its own storage:

```hcl
resource "ory_project_config" "main" {
  courier_delivery_strategy       = "http"
  courier_http_request_config_url = "https://mail.example.com/send"

  courier_http_request_config_body = "base64://${base64encode(file("${path.module}/courier_body.jsonnet"))}"
}
```

The API stores the decoded payload and reports an `https://storage.googleapis.com/.../<sha512>.jsonnet` URL instead of the value you set. The provider compares the hash in that URL with your configured payload, so a matching payload produces no change in plan. A payload edited in the Ory Console does not match the hash, so the provider downloads it and plan shows the difference in `base64://` form.

The nested `courier_http_request_config` attribute and each entry in `courier_channels` take the same `base64://` value for their `body` field. For those two, the provider keeps the configured value whenever the API reports a URL, so a payload edited in the Ory Console is not reported as drift.

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

## SMTP Security Modes

The `smtp_connection_uri` attribute selects the SMTP security mode through the URI scheme and query parameters. The Ory API stores the SMTP configuration as a single URI, so there is no separate security-mode attribute — the scheme and query string fully describe the connection.

| Security mode | URI format | Typical port | When to use |
|---------------|------------|--------------|-------------|
| STARTTLS (recommended) | `smtp://user:pass@host:port/` | 587 | Most modern SMTP relays (SendGrid, Mailgun, SES SMTP, Postfix submission) |
| STARTTLS, skip certificate verification | `smtp://user:pass@host:port/?skip_ssl_verify=true` | 587 | Development or self-signed certs only |
| Cleartext (no encryption) | `smtp://user:pass@host:port/?disable_starttls=true` | 25, 1025 | Local development with [MailHog](https://github.com/mailhog/MailHog), [MailCatcher](https://mailcatcher.me), or similar |
| Implicit TLS | `smtps://user:pass@host:port/` | 465 | Legacy SMTPS endpoints that negotiate TLS before any plaintext |
| Implicit TLS, skip certificate verification | `smtps://user:pass@host:port/?skip_ssl_verify=true` | 465 | Development or self-signed certs only |
| Implicit TLS with server-name override | `smtps://user:pass@host:port/?server_name=mail.example.com` | 465 | When the SMTP host's certificate is issued for a different hostname (non-wildcard certs) |

~> **Common mistake:** Using `smtps://...:587` forces implicit TLS on a port that expects STARTTLS, which causes most providers to reject the connection or return TLS handshake errors. Use `smtp://...:587` for STARTTLS on port 587.

-> **Credential encoding:** Usernames and passwords must be URI-encoded if they contain special characters (e.g., `@`, `:`, `/`, `?`, `#`). Use [`urlencode`](https://developer.hashicorp.com/terraform/language/functions/urlencode) in Terraform to encode them safely.

-> **Write-only:** The Ory API does not return the SMTP connection URI in project-config responses (it contains credentials). The provider therefore treats `smtp_connection_uri` as write-only — it is sent on create and update but never read back, so the value in your configuration is authoritative. As a result, out-of-band changes to the SMTP URI are not detected, and the attribute will not show a diff if the API omits or masks the value.

For more detail, see the [Ory Kratos SMTP documentation](https://www.ory.com/docs/kratos/emails-sms/sending-emails-smtp).

## Verification After Registration / Settings

Three boolean attributes toggle the [`show_verification_ui`](https://www.ory.com/docs/kratos/self-service/flows/user-registration) post-flow hook for each authentication method:

- `selfservice_flows_registration_after_password_hook_show_verification_ui`
- `selfservice_flows_registration_after_oidc_hook_show_verification_ui`
- `selfservice_flows_settings_after_profile_hook_show_verification_ui`

When `true`, the provider adds the `show_verification_ui` hook to the corresponding flow; when `false`, it removes only that hook. Other hooks at the same path (for example `session` or `organization`) are preserved by reading the current hook list before each apply.

## Email Verification Hooks

The three toggles on the Ory Console **Email verification** page are also post-flow hooks, each with its own attribute:

| Ory Console toggle | Attribute | Flow |
|---|---|---|
| Require verified address for login | `selfservice_flows_login_after_password_hook_require_verified_address` | Password login |
| (not in the Console) | `selfservice_flows_login_after_oidc_hook_require_verified_address` | Social login |
| Verify new addresses | `selfservice_flows_settings_after_profile_hook_verify_new_address` | Profile settings |
| Notify previous addresses | `selfservice_flows_settings_after_profile_hook_notify_previous_addresses` | Profile settings |

The Ory Console toggle writes the password login flow only. The Ory config schema also supports the hook on the social login flow, so the provider exposes it there too. Set both to require a verified address on every login path.

```hcl
resource "ory_project_config" "main" {
  selfservice_flows_verification_enabled = true

  # Block sign-in until the identity has a verified address.
  selfservice_flows_login_after_password_hook_require_verified_address = true
  selfservice_flows_login_after_oidc_hook_require_verified_address     = true

  # Require verification of an address the user just added or changed.
  selfservice_flows_settings_after_profile_hook_verify_new_address = true

  # Email the previous addresses when the verified addresses change.
  selfservice_flows_settings_after_profile_hook_notify_previous_addresses            = true
  selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients = "all_verified"
}
```

`selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients` selects who is notified:

- `removed` notifies only addresses that no longer exist after the change. This is the Ory default.
- `all_verified` notifies every address that was verified before the change.
- `all` notifies every address on the identity before the change.

Leave the attribute out and the provider writes no scope at all, so Ory applies `removed`. Removing the attribute after a scope was already applied is different: the provider stops managing the scope and leaves the project on its current value, in line with how every other optional attribute on this resource behaves. Set it back to `removed` explicitly to return to the default.

-> **`verify_new_address` is not `show_verification_ui`.** `selfservice_flows_settings_after_profile_hook_verify_new_address` gates the address change on verification. `selfservice_flows_settings_after_profile_hook_show_verification_ui` only controls the redirect to the verification UI after the flow. Set both if you want the address gated *and* the user sent to the verification screen.

-> **`feature_flags_legacy_require_verified_login_error` is not an enable switch.** It controls how a failed verified-address check is reported, a form error instead of a `continue_with` step. It has no effect unless `selfservice_flows_login_after_password_hook_require_verified_address` is `true`.

The Ory Console additionally turns on `selfservice_flows_registration_after_password_hook_show_verification_ui` when you enable "Require verified address for login" on a project that signs users in after registration. The provider does not infer that for you. Set the attribute yourself if you want that behavior.

## Account Experience Branding

The hosted Account Experience pages (login, registration, recovery, ...) can be branded with a logo, favicon, and theme colors.

**Logos and favicons** (`account_experience_logo_light`, `account_experience_logo_dark`, `account_experience_favicon_light`, `account_experience_favicon_dark`) must be provided as an inline data URI — the Ory API does not fetch remote URLs. Use [`filebase64`](https://developer.hashicorp.com/terraform/language/functions/filebase64) to embed a local image:

```hcl
resource "ory_project_config" "main" {
  account_experience_logo_light    = "data:image/png;base64,${filebase64("${path.module}/assets/logo.png")}"
  account_experience_favicon_light = "data:image/png;base64,${filebase64("${path.module}/assets/favicon.png")}"
}
```

Supported content types: `image/png`, `image/svg+xml`, `image/x-icon`, `image/vnd.microsoft.icon`, `image/gif`, `image/jpeg`, `image/webp`. The API uploads the image and serves it from a content-addressed storage URL; the provider compares that URL's content hash against your data URI, so an unchanged image never shows a diff. Set the attribute to an empty string (`""`) to remove the image.

**Theme colors** (`account_experience_theme_variables_light`, `account_experience_theme_variables_dark`) are maps of color tokens to CSS color values:

```hcl
resource "ory_project_config" "main" {
  account_experience_theme_variables_light = {
    ax_background_default             = "#fafafa"
    brand_500                         = "#0066ff"
    button_primary_background_default = "#0066ff"
    button_primary_background_hover   = "#0052cc"
  }
}
```

Valid tokens are the fields of the [`AccountExperienceColors`](https://github.com/ory/client-go/blob/master/docs/AccountExperienceColors.md) API model (for example `ax_background_default`, `brand_50` through `brand_950`, `button_primary_*`, `input_*`, `interface_*`). The Ory Console theme editor (Branding → Theme) is an easy way to discover token names: configure colors there, then run `ory get project <id> --format json` and copy the `theme_variables_light`/`theme_variables_dark` objects.

~> **Note:** The API silently discards unrecognized color tokens. If a token you set keeps reappearing in `terraform plan`, check its spelling against the model linked above.

Set a theme map to `{}` to reset all colors to the defaults.

## Clearing Return URLs

To explicitly clear `default_return_url` and `allowed_return_urls` (e.g., for native-only flows with no browser redirects), set them to empty values:

```hcl
resource "ory_project_config" "native_only" {
  default_return_url  = ""
  allowed_return_urls = []
}
```

Omitting these attributes entirely (or setting them to `null`) leaves the existing API values unchanged.

## Notes

- Project config cannot be deleted — it always exists for a project
- Deleting this resource from Terraform state does not reset the project configuration
- The `project_id` attribute forces replacement if changed (you cannot move config to a different project)
- After `terraform import`, run `terraform plan` to reconcile your configuration with the current API state

## Coverage

This resource covers most simple (scalar and list) project configuration properties. Some settings are managed by dedicated resources (e.g., ory_social_provider, ory_identity_schema) or require custom handling.

### Migrating Deprecated Attribute Names

Some attributes have been renamed for consistency with the API spec. The old names still work but show deprecation warnings. A [migration script](https://github.com/ory/terraform-provider-ory/blob/main/scripts/migrate-deprecated-attrs.sh) is provided:

```bash
curl -sSfL https://raw.githubusercontent.com/ory/terraform-provider-ory/main/scripts/migrate-deprecated-attrs.sh -o migrate.sh
chmod +x migrate.sh
./migrate.sh .
terraform plan  # verify no changes
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `account_experience_contact_url` (String) Holds the URL to the account experience's Contact page.
- `account_experience_default_locale` (String) Default locale for the hosted login UI (e.g., 'en', 'de').
- `account_experience_enabled_locales` (List of String) Enabled locales for the hosted login UI. An empty list is replaced by the server's default locale set, which the next plan reports as a change; omit the attribute instead of setting it empty.
- `account_experience_favicon_dark` (String) Favicon for the hosted Account Experience UI (dark theme). Must be an inline data URI (e.g. data:image/png;base64,...) or a storage URL previously returned by the API. The API uploads the image and serves it from a content-addressed storage URL; the provider matches that URL against the data URI by content hash to detect drift. Set to an empty string to remove.
- `account_experience_favicon_light` (String) Favicon for the hosted Account Experience UI (light theme). Must be an inline data URI (e.g. data:image/png;base64,...) or a storage URL previously returned by the API. The API uploads the image and serves it from a content-addressed storage URL; the provider matches that URL against the data URI by content hash to detect drift. Set to an empty string to remove.
- `account_experience_hide_ory_branding` (Boolean) Whether to hide the Ory branding badge on the account experience.
- `account_experience_hide_registration_link` (Boolean) Whether to hide the registration link on the account experience login card.
- `account_experience_locale_behavior` (String) Locale behavior: 'respect_accept_language' or 'force_default'.
- `account_experience_logo_dark` (String) Logo for the hosted Account Experience UI (dark theme). Must be an inline data URI (e.g. data:image/png;base64,...) or a storage URL previously returned by the API. The API uploads the image and serves it from a content-addressed storage URL; the provider matches that URL against the data URI by content hash to detect drift. Set to an empty string to remove.
- `account_experience_logo_light` (String) Logo for the hosted Account Experience UI (light theme). Must be an inline data URI (e.g. data:image/png;base64,...) or a storage URL previously returned by the API. The API uploads the image and serves it from a content-addressed storage URL; the provider matches that URL against the data URI by content hash to detect drift. Set to an empty string to remove.
- `account_experience_privacy_policy_url` (String) Holds the URL to the account experience's Privacy Policy page.
- `account_experience_terms_of_service_url` (String) Holds the URL to the account experience's Terms of Service page.
- `account_experience_theme_variables_dark` (Map of String) Theme color variables for the hosted Account Experience UI (dark theme). Map of color tokens (e.g. ax_background_default, brand_500, button_primary_background_default) to CSS color values. Keys not recognized by the API are discarded. Set to an empty map to reset.
- `account_experience_theme_variables_light` (Map of String) Theme color variables for the hosted Account Experience UI (light theme). Map of color tokens (e.g. ax_background_default, brand_500, button_primary_background_default) to CSS color values. Keys not recognized by the API are discarded. Set to an empty map to reset.
- `allowed_return_urls` (List of String) List of allowed return URLs.
- `code_lifespan` (String, Deprecated) Lifespan of the code method's one-time codes (e.g., '15m0s'). Controls how long a code remains valid after being issued.
- `code_mfa_enabled` (Boolean, Deprecated) Enable the code method as a second factor for MFA. When enabled, users can use one-time codes as a second authentication factor.
- `code_missing_credential_fallback_enabled` (Boolean, Deprecated) Enable missing credential fallback for the code method. When enabled, allows the code method to be used as a fallback when the primary credential is missing.
- `cookies_same_site` (String) SameSite attribute for identity service cookies.
- `cors_admin_enabled` (Boolean) Enable CORS for the admin API.
- `cors_admin_origins` (List of String) Allowed CORS origins for the admin API.
- `cors_enabled` (Boolean) Enable CORS for the public API.
- `cors_origins` (List of String) Allowed CORS origins.
- `courier_channels` (Attributes List) Per-channel courier delivery configurations (e.g., SMS via Twilio). Each channel overrides the default delivery for a specific message channel. (see [below for nested schema](#nestedatt--courier_channels))
- `courier_delivery_strategy` (String) Courier delivery strategy: 'smtp' (default) or 'http'.
- `courier_http_request_config` (Attributes) HTTP request configuration for courier message delivery (used when courier_delivery_strategy is 'http'). (see [below for nested schema](#nestedatt--courier_http_request_config))
- `courier_http_request_config_auth_api_key_in` (String) Where to send the API key for HTTP courier auth ('header' or 'cookie').
- `courier_http_request_config_auth_api_key_name` (String) API key name for HTTP courier authentication.
- `courier_http_request_config_auth_api_key_value` (String, Sensitive) API key value for HTTP courier authentication.
- `courier_http_request_config_auth_basic_auth_password` (String, Sensitive) Password for HTTP courier basic authentication.
- `courier_http_request_config_auth_basic_auth_user` (String) Username for HTTP courier basic authentication.
- `courier_http_request_config_auth_type` (String) Authentication type for the courier HTTP request (basic_auth, api_key, or empty).
- `courier_http_request_config_body` (String) Jsonnet template for the HTTP courier request body, as a `base64://<base64-encoded-payload>` value. The API stores the payload in object storage and reports an https URL back, so the read path resolves that URL to the configured value.
- `courier_http_request_config_method` (String) HTTP method for the courier HTTP request.
- `courier_http_request_config_url` (String) URL of the remote HTTP email sending service.
- `courier_smtp_from_address` (String) Email address to send from.
- `courier_smtp_from_name` (String) Name to display as sender.
- `courier_smtp_local_name` (String) Local hostname used in SMTP HELO/EHLO commands.
- `default_return_url` (String) Default URL to redirect after flows.
- `disable_account_experience_welcome_screen` (Boolean) Disable the account experience welcome screen at /ui/welcome. Applied and read via the normalized project revision API.
- `enable_ax_v2` (Boolean) Enable the new account experience UI.
- `enable_code` (Boolean, Deprecated) Enable code-based authentication.
- `enable_lookup_secret` (Boolean, Deprecated) Enable backup/recovery codes.
- `enable_oidc` (Boolean, Deprecated) Enable OIDC (OpenID Connect) social sign-in. Must be enabled for social providers (e.g. Google, GitHub) to work.
- `enable_oidc_auto_link_policy` (Boolean, Deprecated) Enable the OIDC auto-link policy. When true, social sign-in providers with auto_link enabled (on ory_social_provider) can automatically link to existing identities that share the same identifier (e.g., email). This is an enterprise-gated feature that Ory must enable for the project (the `use_auto_link` feature flag); without the entitlement, setting this to true is rejected by the API with an HTTP 403 (feature_not_available).
- `enable_passkey` (Boolean, Deprecated) Enable Passkey authentication.
- `enable_password` (Boolean, Deprecated) Enable password authentication.
- `enable_profile` (Boolean, Deprecated) Enable the profile authentication method. When enabled, users can update their identity traits (e.g., name, address) via the settings flow.
- `enable_recovery` (Boolean, Deprecated) Enable password recovery flow.
- `enable_registration` (Boolean, Deprecated) Enable user registration.
- `enable_totp` (Boolean, Deprecated) Enable TOTP (Time-based One-Time Password).
- `enable_verification` (Boolean, Deprecated) Enable email verification flow.
- `enable_webauthn` (Boolean, Deprecated) Enable WebAuthn (hardware keys).
- `error_ui_url` (String, Deprecated) URL for the error UI.
- `feature_flags_cacheable_sessions` (Boolean) Enable session caching.
- `feature_flags_cacheable_sessions_max_age` (String) Maximum age for cached sessions (e.g. '5m').
- `feature_flags_choose_recovery_address` (Boolean) Allow users to choose which recovery address to use.
- `feature_flags_faster_session_extend` (Boolean) Enable faster session extension by skipping the session lookup.
- `feature_flags_legacy_continue_with_verification_ui` (Boolean) Deprecated. Restore legacy behavior of always including show_verification_ui in continue_with.
- `feature_flags_legacy_oidc_registration_node_group` (Boolean) Use legacy 'oidc' node group for OIDC registration when required fields are missing.
- `feature_flags_legacy_require_verified_login_error` (Boolean) Deprecated. Return a form error instead of continue_with when the login identifier is not verified.
- `feature_flags_password_profile_registration_node_group` (Boolean) Use password method group for profile registration node group.
- `feature_flags_refresh_login_choose_address` (Boolean) Render an address picker on the code refresh login screen
- `feature_flags_use_continue_with_transitions` (Boolean) Enable continue_with transitions for session flows.
- `identity_secrets_cipher` (List of String, Sensitive) Encryption secrets for identity data at rest.
- `identity_secrets_cookie` (List of String, Sensitive) Cookie signing secrets for the identity service.
- `identity_secrets_default` (List of String, Sensitive) Default signing secrets for the identity service.
- `identity_secrets_pagination` (List of String, Sensitive) Pagination encryption keys for the identity service.
- `keto_namespace_configuration` (String) URL pointing to an OPL file with the Keto namespace configuration.
- `keto_namespaces` (List of String) List of Keto namespace names to configure for Ory Permissions. Namespaces define the types of resources in your permission model (e.g., 'documents', 'folders'). Each namespace name must be unique.
- `keto_secrets_pagination` (List of String, Sensitive) Pagination encryption keys for the permission service.
- `login_style` (String, Deprecated) Login flow style: 'unified' (default) shows all auth methods on one screen, 'identifier_first' collects the identifier before showing auth methods.
- `login_ui_url` (String, Deprecated) URL for the login UI.
- `mfa_enforcement` (String) MFA enforcement level: 'none', 'optional', or 'required'.
- `oauth2_access_token_lifespan` (String, Deprecated) OAuth2 access token lifespan (e.g., '1h', '30m'). Requires Hydra service.
- `oauth2_access_token_strategy` (String, Deprecated) OAuth2 access token strategy ('jwt' or 'opaque').
- `oauth2_allowed_top_level_claims` (List of String) OAuth2 claims allowed as top-level fields in access tokens.
- `oauth2_auth_code_lifespan` (String, Deprecated) OAuth2 authorization code lifespan (e.g., '30m'). Requires Hydra service.
- `oauth2_client_credentials_default_grant_allowed_scope` (Boolean) Automatically grant the full authorized scope in OAuth2 client credentials flow.
- `oauth2_consent_url` (String, Deprecated) OAuth2 consent endpoint URL.
- `oauth2_cookies_same_site_legacy_workaround` (Boolean, Deprecated) Enable the SameSite=None legacy workaround for OAuth2 cookies. When enabled, a fallback cookie without SameSite is set alongside the main cookie for clients that don't support SameSite=None.
- `oauth2_cookies_same_site_mode` (String, Deprecated) SameSite attribute for OAuth2 cookies ('Lax', 'Strict', 'None').
- `oauth2_device_authorization_token_polling_interval` (String) How often a non-interactive device should poll the OAuth 2.0 Device Authorization Grant token endpoint
- `oauth2_device_authorization_user_code_entropy_preset` (String) Picks a preset for the OAuth 2.0 Device Authorization Grant user_code length and character set
- `oauth2_error_url` (String, Deprecated) OAuth2 error endpoint URL.
- `oauth2_exclude_not_before_claim` (Boolean) Exclude the `nbf` (not before) claim from access tokens.
- `oauth2_grant_jwt_iat_optional` (Boolean) Make the `iat` claim optional in JWT assertion grants (RFC 7523).
- `oauth2_grant_jwt_jti_optional` (Boolean) Make the `jti` claim optional in JWT assertion grants (RFC 7523).
- `oauth2_grant_jwt_max_ttl` (String) Maximum TTL for JWT assertions in grant flows (e.g. '720h').
- `oauth2_grant_jwt_omit_assertion_audience` (Boolean) The audience (`aud`) claim from the assertion JSON Web Token (JWT) in the JWT Profile for OAuth 2.0 Client Authentication and Authorization Grants (RFC7523) is omitted from the resulting access token
- `oauth2_grant_refresh_token_rotation_grace_period` (String) Grace period for refresh token rotation (e.g. '5s'). Set to '0s' to disable.
- `oauth2_grant_refresh_token_rotation_grace_reuse_count` (Number) OAuth2 Grant Refresh Token Rotation Grace Reuse Count. The maximum number of times a refresh token can be reused within the grace period. If set to `null` or `0`, the limit is disabled.
- `oauth2_id_token_lifespan` (String, Deprecated) OAuth2 ID token lifespan (e.g., '1h'). Requires Hydra service.
- `oauth2_issuer_url` (String, Deprecated) OAuth2 issuer URL. Overrides the default project URL used as the OAuth2/OIDC issuer.
- `oauth2_jwt_scope_claim` (String, Deprecated) How scopes are represented in JWT access tokens ('list', 'string', or 'both').
- `oauth2_login_consent_request_lifespan` (String, Deprecated) OAuth2 login/consent request lifespan (e.g., '30m'). Requires Hydra service.
- `oauth2_login_url` (String, Deprecated) OAuth2 login endpoint URL.
- `oauth2_logout_url` (String, Deprecated) OAuth2 logout endpoint URL.
- `oauth2_mirror_top_level_claims` (Boolean) Mirror top-level claims in OAuth2 ID tokens.
- `oauth2_pkce_enforced` (Boolean) Enforce PKCE for all OAuth2 clients.
- `oauth2_pkce_enforced_for_public_clients` (Boolean) Enforce PKCE for public OAuth2 clients only.
- `oauth2_preserve_ext_claims` (Boolean) Set to true to keep custom claims that are not promoted to the top level in the 'ext' claim. Only applies when mirror_top_level_claims is false.
- `oauth2_provider_headers` (Map of String) Custom HTTP headers for the OAuth2 provider integration.
- `oauth2_provider_override_return_to` (Boolean) Allow the OAuth2 provider to automatically set the return_to parameter.
- `oauth2_provider_url` (String) OAuth2 provider integration URL.
- `oauth2_refresh_token_hook` (String) Webhook URL called during OAuth2 token refresh to update access token claims.
- `oauth2_refresh_token_lifespan` (String, Deprecated) OAuth2 refresh token lifespan (e.g., '720h' for 30 days). Requires Hydra service.
- `oauth2_scope_strategy` (String, Deprecated) OAuth2 scope matching strategy ('exact', 'wildcard').
- `oauth2_secrets_cookie` (List of String, Sensitive) Cookie signing secrets for the OAuth2 service.
- `oauth2_secrets_pagination` (List of String, Sensitive) Pagination encryption keys for the OAuth2 service.
- `oauth2_secrets_system` (List of String, Sensitive) System-wide encryption secrets for the OAuth2 service.
- `oauth2_serve_cookies_same_site_legacy_workaround` (Boolean) Enable the SameSite=None legacy workaround for OAuth2 cookies. When enabled, a fallback cookie without SameSite is set alongside the main cookie for clients that don't support SameSite=None.
- `oauth2_serve_cookies_same_site_mode` (String) SameSite attribute for OAuth2 cookies ('Lax', 'Strict', 'None').
- `oauth2_strategies_access_token` (String) OAuth2 access token strategy ('jwt' or 'opaque').
- `oauth2_strategies_jwt_scope_claim` (String) How scopes are represented in JWT access tokens ('list', 'string', or 'both').
- `oauth2_strategies_scope` (String) OAuth2 scope matching strategy ('exact', 'wildcard').
- `oauth2_token_hook` (String) Webhook URL called during token issuance for all grant types to customize claims.
- `oauth2_token_hook_auth` (Attributes) Authentication configuration for the OAuth2 token hook (see `oauth2_token_hook`). Currently only `api_key` authentication is supported. (see [below for nested schema](#nestedatt--oauth2_token_hook_auth))
- `oauth2_token_prefix` (String) Sets a per-project Access Token, Refresh Token, and Authorization Code prefix
- `oauth2_ttl_access_token` (String) OAuth2 access token lifespan (e.g., '1h', '30m'). Requires Hydra service.
- `oauth2_ttl_auth_code` (String) OAuth2 authorization code lifespan (e.g., '30m'). Requires Hydra service.
- `oauth2_ttl_device_user_code` (String) How long the device_code and user_code in the OAuth 2.0 Device Authorization Grant remain valid
- `oauth2_ttl_id_token` (String) OAuth2 ID token lifespan (e.g., '1h'). Requires Hydra service.
- `oauth2_ttl_login_consent_request` (String) OAuth2 login/consent request lifespan (e.g., '30m'). Requires Hydra service.
- `oauth2_ttl_refresh_token` (String) OAuth2 refresh token lifespan (e.g., '720h' for 30 days). Requires Hydra service.
- `oauth2_urls_consent` (String) OAuth2 consent endpoint URL.
- `oauth2_urls_device_success` (String) Sets the URL the user is redirected to after successfully completing the OAuth 2.0 Device Authorization Grant user verification step
- `oauth2_urls_device_verification` (String) Sets the URL of the user verification page for the OAuth 2.0 Device Authorization Grant (RFC 8628)
- `oauth2_urls_error` (String) OAuth2 error endpoint URL.
- `oauth2_urls_login` (String) OAuth2 login endpoint URL.
- `oauth2_urls_logout` (String) OAuth2 logout endpoint URL.
- `oauth2_urls_post_logout_redirect` (String) Default redirect URL after OAuth2 logout.
- `oauth2_urls_registration` (String) Registration endpoint URL for the OAuth2 login and consent flow.
- `oauth2_urls_self_issuer` (String) OAuth2 issuer URL. Overrides the default project URL used as the OAuth2/OIDC issuer.
- `oauth2_webfinger_jwks_broadcast_keys` (List of String) JWK set IDs to broadcast via OIDC discovery.
- `oauth2_webfinger_oidc_discovery_auth_url` (String) Override the OAuth2 authorization URL in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_client_registration_url` (String) Override the dynamic client registration URL in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_device_authorization_url` (String) Override the OAuth 2.0 Device Authorization Endpoint URL that is advertised in the OpenID Connect discovery document (/.well-known/openid-configuration)
- `oauth2_webfinger_oidc_discovery_jwks_url` (String) Override the JWKS URL in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_supported_claims` (List of String) Supported claims advertised in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_supported_scope` (List of String) Supported scopes advertised in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_token_url` (String) Override the OAuth2 token URL in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_userinfo_url` (String) Override the userinfo endpoint URL in OIDC discovery.
- `oidc_dynamic_client_registration_default_scope` (List of String) Default OAuth2 scopes granted to dynamically registered clients.
- `oidc_dynamic_client_registration_enabled` (Boolean) Enable OpenID Connect dynamic client registration.
- `oidc_subject_identifiers_pairwise_salt` (String) Salt for the OIDC pairwise subject identifier algorithm.
- `oidc_subject_identifiers_supported_types` (List of String) Supported OIDC subject identifier types ('public', 'pairwise').
- `password_check_haveibeenpwned` (Boolean, Deprecated) Check passwords against HaveIBeenPwned.
- `password_identifier_similarity` (Boolean, Deprecated) Check password similarity to identifier.
- `password_max_breaches` (Number, Deprecated) Maximum allowed breaches in HaveIBeenPwned.
- `password_min_length` (Number, Deprecated) Minimum password length.
- `preview_default_read_consistency_level` (String) Default read consistency level for identity APIs ('strong' or 'eventual').
- `project_id` (String) Project ID to configure. If not set, uses provider's project_id.
- `recovery_ui_url` (String, Deprecated) URL for the password recovery UI.
- `registration_ui_url` (String, Deprecated) URL for the registration UI.
- `required_aal` (String, Deprecated) Required Authenticator Assurance Level for the settings flow: 'aal1' or 'highest_available'.
- `security_account_enumeration_mitigate` (Boolean) Mitigate account enumeration when using identifier-first login.
- `selfservice_default_browser_return_url` (String) Default browser return URL for self-service flows.
- `selfservice_flows_error_ui_url` (String) URL for the error UI.
- `selfservice_flows_login_after_code_default_browser_return_url` (String) Return URL after login via code method.
- `selfservice_flows_login_after_default_browser_return_url` (String) Default return URL after login.
- `selfservice_flows_login_after_lookup_secret_default_browser_return_url` (String) Return URL after login via lookup secret method.
- `selfservice_flows_login_after_oidc_default_browser_return_url` (String) Return URL after login via OIDC.
- `selfservice_flows_login_after_oidc_hook_require_verified_address` (Boolean) Enable the `require_verified_address` hook after an OIDC (social) login, blocking sign-in until the identity has a verified address. The Ory Console "Require verified address for login" toggle only writes the password flow, so set this attribute as well to cover social logins. Existing hooks at this path (e.g., `organization`) are preserved.
- `selfservice_flows_login_after_passkey_default_browser_return_url` (String) Return URL after login via passkey.
- `selfservice_flows_login_after_password_default_browser_return_url` (String) Return URL after login via password.
- `selfservice_flows_login_after_password_hook_require_verified_address` (Boolean) Enable the `require_verified_address` hook after a password login, blocking sign-in until the identity has a verified address. Mirrors the Ory Console "Require verified address for login" toggle. Use `feature_flags_legacy_require_verified_login_error` to control how the failure is reported, not whether the check runs. Existing hooks at this path (e.g., `organization`) are preserved.
- `selfservice_flows_login_after_totp_default_browser_return_url` (String) Return URL after login via TOTP.
- `selfservice_flows_login_after_webauthn_default_browser_return_url` (String) Return URL after login via WebAuthn.
- `selfservice_flows_login_lifespan` (String) Lifespan of the login flow (e.g. '1h').
- `selfservice_flows_login_style` (String) Login flow style: 'unified' (default) shows all auth methods on one screen, 'identifier_first' collects the identifier before showing auth methods.
- `selfservice_flows_login_ui_url` (String) URL for the login UI.
- `selfservice_flows_logout_after_default_browser_return_url` (String) Default return URL after logout.
- `selfservice_flows_recovery_after_default_browser_return_url` (String) Default return URL after recovery.
- `selfservice_flows_recovery_enabled` (Boolean) Enable password recovery flow.
- `selfservice_flows_recovery_lifespan` (String) Lifespan of the recovery flow (e.g. '1h').
- `selfservice_flows_recovery_notify_unknown_recipients` (Boolean) Send recovery notifications even when the email is not registered.
- `selfservice_flows_recovery_ui_url` (String) URL for the password recovery UI.
- `selfservice_flows_recovery_use` (String) Recovery strategy to use ('link' or 'code').
- `selfservice_flows_registration_after_code_default_browser_return_url` (String) Return URL after registration via code method.
- `selfservice_flows_registration_after_default_browser_return_url` (String) Default return URL after registration.
- `selfservice_flows_registration_after_oidc_default_browser_return_url` (String) Return URL after registration via OIDC.
- `selfservice_flows_registration_after_oidc_hook_show_verification_ui` (Boolean) Enable the `show_verification_ui` hook after a successful OIDC (social) registration. When true, users are redirected to the verification UI after registering via a social provider. Existing hooks at this path (e.g., `session`, `organization`) are preserved.
- `selfservice_flows_registration_after_passkey_default_browser_return_url` (String) Return URL after registration via passkey.
- `selfservice_flows_registration_after_password_default_browser_return_url` (String) Return URL after registration via password.
- `selfservice_flows_registration_after_password_hook_session` (Boolean) Enable the `session` hook after a successful password registration, automatically signing the user in. Mirrors the Ory Console "Enable sign in after registration" toggle. Existing hooks at this path (e.g., `organization`) are preserved.
- `selfservice_flows_registration_after_password_hook_show_verification_ui` (Boolean) Enable the `show_verification_ui` hook after a successful password registration. When true, users are redirected to the verification UI after registering with email + password. Existing hooks at this path (e.g., `session`, `organization`) are preserved.
- `selfservice_flows_registration_after_webauthn_default_browser_return_url` (String) Return URL after registration via WebAuthn.
- `selfservice_flows_registration_enable_legacy_one_step` (Boolean) Revert to legacy one-step registration instead of the two-step flow.
- `selfservice_flows_registration_enabled` (Boolean) Enable user registration.
- `selfservice_flows_registration_lifespan` (String) Lifespan of the registration flow (e.g. '1h').
- `selfservice_flows_registration_login_hints` (Boolean) Show login hints when a user tries to register with a duplicate account.
- `selfservice_flows_registration_ui_url` (String) URL for the registration UI.
- `selfservice_flows_settings_after_default_browser_return_url` (String) Default return URL after updating settings.
- `selfservice_flows_settings_after_lookup_secret_default_browser_return_url` (String) Return URL after updating lookup secrets in settings.
- `selfservice_flows_settings_after_oidc_default_browser_return_url` (String) Return URL after updating OIDC connections in settings.
- `selfservice_flows_settings_after_passkey_default_browser_return_url` (String) Return URL after updating passkey in settings.
- `selfservice_flows_settings_after_password_default_browser_return_url` (String) Return URL after updating password in settings.
- `selfservice_flows_settings_after_profile_default_browser_return_url` (String) Return URL after updating profile in settings.
- `selfservice_flows_settings_after_profile_hook_notify_previous_addresses` (Boolean) Enable the `notify_previous_addresses` hook after a profile settings update, emailing the identity's previous addresses when its verified addresses change. Mirrors the Ory Console "Notify previous addresses" toggle. Existing hooks at this path (e.g., `organization`) are preserved.
- `selfservice_flows_settings_after_profile_hook_notify_previous_addresses_recipients` (String) Which previous addresses the `notify_previous_addresses` hook notifies: `removed` for addresses that no longer exist after the change, `all_verified` for every address verified before the change, or `all` for every address on the identity before the change. Requires `selfservice_flows_settings_after_profile_hook_notify_previous_addresses` to be set. Leave this attribute out to let Ory apply its own default of `removed`. Note that removing it after a scope was already applied stops managing the scope rather than resetting it.
- `selfservice_flows_settings_after_profile_hook_show_verification_ui` (Boolean) Enable the `show_verification_ui` hook after a successful profile settings update. When true, users are redirected to the verification UI after updating their profile (e.g., changing their email). Existing hooks at this path (e.g., `organization`) are preserved.
- `selfservice_flows_settings_after_profile_hook_verify_new_address` (Boolean) Enable the `verify_new_address` hook after a profile settings update, requiring verification of an address the user just added or changed. Mirrors the Ory Console "Verify new addresses" toggle. Distinct from `selfservice_flows_settings_after_profile_hook_show_verification_ui`, which only controls the redirect to the verification UI. Existing hooks at this path (e.g., `organization`) are preserved.
- `selfservice_flows_settings_after_totp_default_browser_return_url` (String) Return URL after updating TOTP in settings.
- `selfservice_flows_settings_after_webauthn_default_browser_return_url` (String) Return URL after updating WebAuthn in settings.
- `selfservice_flows_settings_lifespan` (String) Lifespan of the settings flow (e.g., '30m0s'). Controls how long a settings flow session remains valid.
- `selfservice_flows_settings_privileged_session_max_age` (String) Maximum age of a privileged session for the settings flow (e.g., '15m0s'). After this duration, the user must re-authenticate to make privileged changes like password updates.
- `selfservice_flows_settings_required_aal` (String) Required Authenticator Assurance Level for the settings flow: 'aal1' or 'highest_available'.
- `selfservice_flows_settings_ui_url` (String) URL for the account settings UI.
- `selfservice_flows_verification_after_default_browser_return_url` (String) Default return URL after verification.
- `selfservice_flows_verification_enabled` (Boolean) Enable email verification flow.
- `selfservice_flows_verification_lifespan` (String) Lifespan of the verification flow (e.g., '30m0s'). Controls how long a verification flow session remains valid.
- `selfservice_flows_verification_notify_unknown_recipients` (Boolean) When enabled, verification emails are sent even if the email address is not associated with any known identity.
- `selfservice_flows_verification_ui_url` (String) URL for the verification UI.
- `selfservice_flows_verification_use` (String) Verification method to use: 'code' (one-time code) or 'link' (magic link).
- `selfservice_methods_captcha_config_allowed_domains` (List of String) Domains allowed for CAPTCHA verification. Once set, the API refuses to clear this list: applying an empty list keeps the previous domains and the following plan reports the difference. Replace the entries instead of emptying them.
- `selfservice_methods_captcha_config_byo` (Boolean) Use bring-your-own CAPTCHA widget instead of managed.
- `selfservice_methods_captcha_config_cf_turnstile_secret` (String, Sensitive) Cloudflare Turnstile managed site secret (private).
- `selfservice_methods_captcha_config_cf_turnstile_sitekey` (String) Cloudflare Turnstile site key for managed CAPTCHA.
- `selfservice_methods_captcha_config_legacy_inject_node` (Boolean) Inject CAPTCHA as a legacy UI node.
- `selfservice_methods_captcha_enabled` (Boolean) Enable CAPTCHA protection for self-service flows.
- `selfservice_methods_code_config_lifespan` (String) Lifespan of the code method's one-time codes (e.g., '15m0s'). Controls how long a code remains valid after being issued.
- `selfservice_methods_code_config_max_submissions` (Number) Maximum number of code submission attempts before invalidation.
- `selfservice_methods_code_config_missing_credential_fallback_enabled` (Boolean) Enable missing credential fallback for the code method. When enabled, allows the code method to be used as a fallback when the primary credential is missing.
- `selfservice_methods_code_enabled` (Boolean) Enable code-based authentication.
- `selfservice_methods_code_mfa_enabled` (Boolean) Enable the code method as a second factor for MFA. When enabled, users can use one-time codes as a second authentication factor.
- `selfservice_methods_code_passwordless_enabled` (Boolean) Enable passwordless login via the code method.
- `selfservice_methods_code_passwordless_login_fallback_enabled` (Boolean) Allow code-based login as a fallback for users registered with other methods.
- `selfservice_methods_deviceauthn_config_android_app_ids` (List of String) Allow-list of Android app signing-certificate digests that a device key may be bound to.
- `selfservice_methods_deviceauthn_config_first_factor` (Boolean) Device authentication may be used as the sole first factor.
- `selfservice_methods_deviceauthn_config_insecure_allow_relaxed_attestation` (Boolean) Device authentication accepts relaxed attestations for testing. Only allowed on development projects and forced off otherwise.
- `selfservice_methods_deviceauthn_config_ios_app_ids` (List of String) Allow-list of Apple App IDs that a device key may be bound to.
- `selfservice_methods_deviceauthn_config_ios_biometric_first_factor` (Boolean) An iOS biometric device key may be used as the sole first factor.
- `selfservice_methods_deviceauthn_config_pin_max_attempts` (Number) Consecutive wrong-PIN limit before a device key is locked.
- `selfservice_methods_deviceauthn_enabled` (Boolean) Device authentication is enabled
- `selfservice_methods_link_config_base_url` (String) Base URL for recovery, verification, and login links. Leave empty for automatic detection.
- `selfservice_methods_link_config_lifespan` (String) Lifespan of magic links (e.g. '1h').
- `selfservice_methods_link_enabled` (Boolean) Enable the magic link authentication method.
- `selfservice_methods_lookup_secret_enabled` (Boolean) Enable backup/recovery codes.
- `selfservice_methods_oidc_config_base_redirect_uri` (String) Base redirect URI for OIDC/social sign-in callbacks.
- `selfservice_methods_oidc_enable_auto_link_policy` (Boolean) Enable the OIDC auto-link policy. When true, social sign-in providers with auto_link enabled (on ory_social_provider) can automatically link to existing identities that share the same identifier (e.g., email). This is an enterprise-gated feature that Ory must enable for the project (the `use_auto_link` feature flag); without the entitlement, setting this to true is rejected by the API with an HTTP 403 (feature_not_available).
- `selfservice_methods_oidc_enabled` (Boolean) Enable OIDC (OpenID Connect) social sign-in. Must be enabled for social providers (e.g. Google, GitHub) to work.
- `selfservice_methods_passkey_config_rp_display_name` (String) Passkey relying party display name.
- `selfservice_methods_passkey_config_rp_id` (String) Passkey relying party ID (typically your domain).
- `selfservice_methods_passkey_config_rp_origins` (List of String) Allowed origins for passkey relying party verification. An empty list is replaced by the server's default origins (derived from the project slug and custom domains), which the next plan reports as a change; omit the attribute instead of setting it empty.
- `selfservice_methods_passkey_enabled` (Boolean) Enable Passkey authentication.
- `selfservice_methods_password_config_haveibeenpwned_enabled` (Boolean) Check passwords against HaveIBeenPwned.
- `selfservice_methods_password_config_identifier_similarity_check_enabled` (Boolean) Check password similarity to identifier.
- `selfservice_methods_password_config_ignore_network_errors` (Boolean) Ignore HaveIBeenPwned network errors during password validation.
- `selfservice_methods_password_config_max_breaches` (Number) Maximum allowed breaches in HaveIBeenPwned.
- `selfservice_methods_password_config_min_password_length` (Number) Minimum password length.
- `selfservice_methods_password_enabled` (Boolean) Enable password authentication.
- `selfservice_methods_profile_enabled` (Boolean) Enable the profile authentication method. When enabled, users can update their identity traits (e.g., name, address) via the settings flow.
- `selfservice_methods_saml_enabled` (Boolean) Enable SAML login method.
- `selfservice_methods_totp_config_issuer` (String) TOTP issuer name shown in authenticator apps. An empty string is replaced by the project name, which the next plan reports as a change; omit the attribute instead of setting it empty.
- `selfservice_methods_totp_enabled` (Boolean) Enable TOTP (Time-based One-Time Password).
- `selfservice_methods_webauthn_config_passwordless` (Boolean) Enable passwordless WebAuthn authentication.
- `selfservice_methods_webauthn_config_rp_display_name` (String) WebAuthn Relying Party display name.
- `selfservice_methods_webauthn_config_rp_icon` (String) Deprecated. WebAuthn relying party icon URL (ignored for security reasons).
- `selfservice_methods_webauthn_config_rp_id` (String) WebAuthn Relying Party ID (typically your domain).
- `selfservice_methods_webauthn_enabled` (Boolean) Enable WebAuthn (hardware keys).
- `session_cookie_persistent` (Boolean) Enable persistent session cookies (survive browser close).
- `session_cookie_same_site` (String) SameSite cookie attribute (Lax, Strict, None).
- `session_earliest_possible_extend` (String) Earliest time before session expiry when a session can be extended (e.g., '24h'). Setting this prevents excessive database writes when sessions are extended.
- `session_lifespan` (String) Session duration (e.g., '24h0m0s').
- `session_tokenizer_templates` (Attributes Map) JWT tokenizer templates for the /sessions/whoami endpoint. Each key is a template name, and the value configures how JWTs are generated. (see [below for nested schema](#nestedatt--session_tokenizer_templates))
- `session_whoami_required_aal` (String) Required AAL for session whoami endpoint: 'aal1', 'aal2', or 'highest_available'.
- `settings_lifespan` (String, Deprecated) Lifespan of the settings flow (e.g., '30m0s'). Controls how long a settings flow session remains valid.
- `settings_privileged_session_max_age` (String, Deprecated) Maximum age of a privileged session for the settings flow (e.g., '15m0s'). After this duration, the user must re-authenticate to make privileged changes like password updates.
- `settings_ui_url` (String, Deprecated) URL for the account settings UI.
- `smtp_connection_uri` (String, Sensitive) SMTP connection URI for sending emails. The URI scheme selects the security mode: `smtp://` uses STARTTLS (recommended for port 587), `smtps://` uses implicit TLS (recommended for port 465). Append `?disable_starttls=true` for cleartext or `?skip_ssl_verify=true` to skip certificate verification (development only). See the SMTP Security Modes section in the resource documentation for the full list.
- `smtp_connection_uri_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of smtp_connection_uri (Terraform 1.11+ write-only argument): the SMTP connection URI is sent to Ory but never stored in Terraform state or plan. Use this to source the value from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change smtp_connection_uri_wo_version to rotate it. Mutually exclusive with smtp_connection_uri.
- `smtp_connection_uri_wo_version` (String) Version trigger for smtp_connection_uri_wo. Change this value whenever the write-only smtp_connection_uri_wo changes so Terraform sends the new value to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless smtp_connection_uri_wo is set.
- `smtp_from_address` (String, Deprecated) Email address to send from.
- `smtp_from_name` (String, Deprecated) Name to display as sender.
- `smtp_headers` (Map of String) Custom SMTP headers for outbound emails.
- `totp_issuer` (String, Deprecated) TOTP issuer name shown in authenticator apps. An empty string is replaced by the project name, which the next plan reports as a change; omit the attribute instead of setting it empty.
- `verification_lifespan` (String, Deprecated) Lifespan of the verification flow (e.g., '30m0s'). Controls how long a verification flow session remains valid.
- `verification_notify_unknown_recipients` (Boolean, Deprecated) When enabled, verification emails are sent even if the email address is not associated with any known identity.
- `verification_ui_url` (String, Deprecated) URL for the verification UI.
- `verification_use` (String, Deprecated) Verification method to use: 'code' (one-time code) or 'link' (magic link).
- `webauthn_passwordless` (Boolean, Deprecated) Enable passwordless WebAuthn authentication.
- `webauthn_rp_display_name` (String, Deprecated) WebAuthn Relying Party display name.
- `webauthn_rp_id` (String, Deprecated) WebAuthn Relying Party ID (typically your domain).
- `webauthn_rp_origins` (List of String) Allowed origins for WebAuthn relying party. An empty list is replaced by the server's default origins (derived from the project slug and custom domains), which the next plan reports as a change; omit the attribute instead of setting it empty.

### Read-Only

- `id` (String) Resource ID (same as project_id).

<a id="nestedatt--courier_channels"></a>
### Nested Schema for `courier_channels`

Required:

- `id` (String) Channel identifier (e.g., 'sms').

Optional:

- `request_config` (Attributes) HTTP request configuration for this channel. (see [below for nested schema](#nestedatt--courier_channels--request_config))

<a id="nestedatt--courier_channels--request_config"></a>
### Nested Schema for `courier_channels.request_config`

Required:

- `method` (String) HTTP method (e.g., 'POST', 'PUT').
- `url` (String) Target URL for the HTTP request.

Optional:

- `auth` (Attributes) Authentication configuration for the HTTP request. (see [below for nested schema](#nestedatt--courier_channels--request_config--auth))
- `body` (String) Request body template. Supports base64:// scheme for Jsonnet templates.
- `headers` (Map of String) Additional HTTP headers to include.

<a id="nestedatt--courier_channels--request_config--auth"></a>
### Nested Schema for `courier_channels.request_config.auth`

Required:

- `type` (String) Authentication type: 'basic_auth' or 'api_key'.

Optional:

- `in` (String) Where to send the API key: 'header', 'cookie', or 'query'.
- `name` (String) Header/cookie/query parameter name for api_key auth.
- `password` (String, Sensitive) Password for basic_auth.
- `user` (String) Username for basic_auth.
- `value` (String, Sensitive) API key value for api_key auth.




<a id="nestedatt--courier_http_request_config"></a>
### Nested Schema for `courier_http_request_config`

Required:

- `method` (String) HTTP method (e.g., 'POST', 'PUT').
- `url` (String) Target URL for the HTTP request.

Optional:

- `auth` (Attributes) Authentication configuration for the HTTP request. (see [below for nested schema](#nestedatt--courier_http_request_config--auth))
- `body` (String) Request body template. Supports base64:// scheme for Jsonnet templates.
- `headers` (Map of String) Additional HTTP headers to include.

<a id="nestedatt--courier_http_request_config--auth"></a>
### Nested Schema for `courier_http_request_config.auth`

Required:

- `type` (String) Authentication type: 'basic_auth' or 'api_key'.

Optional:

- `in` (String) Where to send the API key: 'header', 'cookie', or 'query'.
- `name` (String) Header/cookie/query parameter name for api_key auth.
- `password` (String, Sensitive) Password for basic_auth.
- `user` (String) Username for basic_auth.
- `value` (String, Sensitive) API key value for api_key auth.



<a id="nestedatt--oauth2_token_hook_auth"></a>
### Nested Schema for `oauth2_token_hook_auth`

Required:

- `in` (String) Where to send the API key: `header` or `cookie`.
- `name` (String) Header or cookie name carrying the API key (e.g. `X-Api-Key`).
- `type` (String) Authentication type. Currently only `api_key` is supported.
- `value` (String, Sensitive) API key value sent to the token hook.


<a id="nestedatt--session_tokenizer_templates"></a>
### Nested Schema for `session_tokenizer_templates`

Required:

- `jwks_url` (String, Sensitive) JWKS URL for signing tokens. Must use base64:// scheme (e.g., 'base64://eyJrZXlzIjpbXX0=').

Optional:

- `claims_mapper_url` (String) Jsonnet claims mapper URL. Supports base64:// and https:// schemes.
- `subject_source` (String) Subject source for the JWT: 'id' (default) or 'external_id'.
- `ttl` (String) Token time-to-live duration (e.g., '1h', '30m'). Default: '1m'.
