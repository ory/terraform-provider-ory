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

## Example Usage

```terraform
# Basic project configuration
resource "ory_project_config" "basic" {
  cors_enabled                                            = true
  cors_origins                                            = ["https://app.example.com"]
  selfservice_methods_password_config_min_password_length = 10
  session_lifespan                                        = "720h0m0s" # 30 days
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
  session_lifespan          = "168h0m0s" # 7 days
  session_cookie_same_site  = "Strict"
  session_cookie_persistent = true

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
  # (removed: account_experience_name, account_experience_logo_url, account_experience_favicon_url)
  account_experience_default_locale = "en"

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

# SMTP configuration for custom email delivery
resource "ory_project_config" "with_smtp" {
  smtp_connection_uri       = var.smtp_connection_uri
  courier_smtp_from_address = "noreply@example.com"
  courier_smtp_from_name    = "MyApp"
  smtp_headers = {
    "X-SES-CONFIGURATION-SET" = "my-config-set"
  }

  selfservice_methods_password_enabled = true
}

variable "smtp_connection_uri" {
  type        = string
  sensitive   = true
  description = "SMTP connection URI (e.g., smtps://user:pass@smtp.example.com:465)"
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

- `account_experience_default_locale` (String) Default locale for the hosted login UI (e.g., 'en', 'de').
- `account_experience_enabled_locales` (List of String) Enabled locales for the hosted login UI.
- `account_experience_favicon_dark` (String) URL for the dark theme favicon in the hosted login UI.
- `account_experience_favicon_light` (String) URL for the light theme favicon in the hosted login UI.
- `account_experience_hide_ory_branding` (Boolean) Whether to hide the Ory branding badge on the account experience.
- `account_experience_hide_registration_link` (Boolean) Whether to hide the registration link on the account experience login card.
- `account_experience_locale_behavior` (String) Locale behavior: 'respect_accept_language' or 'force_default'.
- `account_experience_logo_dark` (String) URL for the dark theme logo in the hosted login UI.
- `account_experience_logo_light` (String) URL for the light theme logo in the hosted login UI.
- `account_experience_theme_variables_dark` (String) URL for dark theme CSS variables in the hosted login UI.
- `account_experience_theme_variables_light` (String) URL for light theme CSS variables in the hosted login UI.
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
- `courier_http_request_config_body` (String) Base64-encoded Jsonnet template for the HTTP courier request body.
- `courier_http_request_config_method` (String) HTTP method for the courier HTTP request.
- `courier_http_request_config_url` (String) URL of the remote HTTP email sending service.
- `courier_smtp_from_address` (String) Email address to send from.
- `courier_smtp_from_name` (String) Name to display as sender.
- `courier_smtp_local_name` (String) Local hostname used in SMTP HELO/EHLO commands.
- `courier_templates_login_code_valid_email_body_html` (String) HTML body template for valid login-by-code emails.
- `courier_templates_login_code_valid_email_body_plaintext` (String) Plaintext body template for valid login-by-code emails.
- `courier_templates_login_code_valid_email_subject` (String) Subject template for valid login-by-code emails.
- `courier_templates_login_code_valid_sms_body_plaintext` (String) Plaintext body template for valid login-by-code SMS messages.
- `courier_templates_recovery_code_invalid_email_body_html` (String) HTML body template for invalid recovery-by-code emails.
- `courier_templates_recovery_code_invalid_email_body_plaintext` (String) Plaintext body template for invalid recovery-by-code emails.
- `courier_templates_recovery_code_invalid_email_subject` (String) Subject template for invalid recovery-by-code emails.
- `courier_templates_recovery_code_valid_email_body_html` (String) HTML body template for valid recovery-by-code emails.
- `courier_templates_recovery_code_valid_email_body_plaintext` (String) Plaintext body template for valid recovery-by-code emails.
- `courier_templates_recovery_code_valid_email_subject` (String) Subject template for valid recovery-by-code emails.
- `courier_templates_recovery_invalid_email_body_html` (String) HTML body template for invalid recovery emails.
- `courier_templates_recovery_invalid_email_body_plaintext` (String) Plaintext body template for invalid recovery emails.
- `courier_templates_recovery_invalid_email_subject` (String) Subject template for invalid recovery emails.
- `courier_templates_recovery_valid_email_body_html` (String) HTML body template for valid recovery emails.
- `courier_templates_recovery_valid_email_body_plaintext` (String) Plaintext body template for valid recovery emails.
- `courier_templates_recovery_valid_email_subject` (String) Subject template for valid recovery emails.
- `courier_templates_registration_code_valid_email_body_html` (String) HTML body template for valid registration-by-code emails.
- `courier_templates_registration_code_valid_email_body_plaintext` (String) Plaintext body template for valid registration-by-code emails.
- `courier_templates_registration_code_valid_email_subject` (String) Subject template for valid registration-by-code emails.
- `courier_templates_registration_code_valid_sms_body_plaintext` (String) Plaintext body template for valid registration-by-code SMS messages.
- `courier_templates_verification_code_invalid_email_body_html` (String) HTML body template for invalid verification-by-code emails.
- `courier_templates_verification_code_invalid_email_body_plaintext` (String) Plaintext body template for invalid verification-by-code emails.
- `courier_templates_verification_code_invalid_email_subject` (String) Subject template for invalid verification-by-code emails.
- `courier_templates_verification_code_valid_email_body_html` (String) HTML body template for valid verification-by-code emails.
- `courier_templates_verification_code_valid_email_body_plaintext` (String) Plaintext body template for valid verification-by-code emails.
- `courier_templates_verification_code_valid_email_subject` (String) Subject template for valid verification-by-code emails.
- `courier_templates_verification_code_valid_sms_body_plaintext` (String) Plaintext body template for valid verification-by-code SMS messages.
- `courier_templates_verification_invalid_email_body_html` (String) HTML body template for invalid verification emails.
- `courier_templates_verification_invalid_email_body_plaintext` (String) Plaintext body template for invalid verification emails.
- `courier_templates_verification_invalid_email_subject` (String) Subject template for invalid verification emails.
- `courier_templates_verification_valid_email_body_html` (String) HTML body template for valid verification emails.
- `courier_templates_verification_valid_email_body_plaintext` (String) Plaintext body template for valid verification emails.
- `courier_templates_verification_valid_email_subject` (String) Subject template for valid verification emails.
- `default_return_url` (String) Default URL to redirect after flows.
- `disable_account_experience_welcome_screen` (Boolean) Disable the account experience welcome screen at /ui/welcome.
- `enable_ax_v2` (Boolean) Enable the new account experience UI.
- `enable_code` (Boolean, Deprecated) Enable code-based authentication.
- `enable_lookup_secret` (Boolean, Deprecated) Enable backup/recovery codes.
- `enable_oidc` (Boolean, Deprecated) Enable OIDC (OpenID Connect) social sign-in. Must be enabled for social providers (e.g. Google, GitHub) to work.
- `enable_oidc_auto_link_policy` (Boolean, Deprecated) Enable the OIDC auto-link policy. When true, social sign-in providers with auto_link enabled (on ory_social_provider) can automatically link to existing identities that share the same identifier (e.g., email).
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
- `oauth2_error_url` (String, Deprecated) OAuth2 error endpoint URL.
- `oauth2_exclude_not_before_claim` (Boolean) Exclude the `nbf` (not before) claim from access tokens.
- `oauth2_grant_jwt_iat_optional` (Boolean) Make the `iat` claim optional in JWT assertion grants (RFC 7523).
- `oauth2_grant_jwt_jti_optional` (Boolean) Make the `jti` claim optional in JWT assertion grants (RFC 7523).
- `oauth2_grant_jwt_max_ttl` (String) Maximum TTL for JWT assertions in grant flows (e.g. '720h').
- `oauth2_grant_refresh_token_rotation_grace_period` (String) Grace period for refresh token rotation (e.g. '5s'). Set to '0s' to disable.
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
- `oauth2_ttl_access_token` (String) OAuth2 access token lifespan (e.g., '1h', '30m'). Requires Hydra service.
- `oauth2_ttl_auth_code` (String) OAuth2 authorization code lifespan (e.g., '30m'). Requires Hydra service.
- `oauth2_ttl_id_token` (String) OAuth2 ID token lifespan (e.g., '1h'). Requires Hydra service.
- `oauth2_ttl_login_consent_request` (String) OAuth2 login/consent request lifespan (e.g., '30m'). Requires Hydra service.
- `oauth2_ttl_refresh_token` (String) OAuth2 refresh token lifespan (e.g., '720h' for 30 days). Requires Hydra service.
- `oauth2_urls_consent` (String) OAuth2 consent endpoint URL.
- `oauth2_urls_error` (String) OAuth2 error endpoint URL.
- `oauth2_urls_login` (String) OAuth2 login endpoint URL.
- `oauth2_urls_logout` (String) OAuth2 logout endpoint URL.
- `oauth2_urls_post_logout_redirect` (String) Default redirect URL after OAuth2 logout.
- `oauth2_urls_registration` (String) Registration endpoint URL for the OAuth2 login and consent flow.
- `oauth2_urls_self_issuer` (String) OAuth2 issuer URL. Overrides the default project URL used as the OAuth2/OIDC issuer.
- `oauth2_webfinger_jwks_broadcast_keys` (List of String) JWK set IDs to broadcast via OIDC discovery.
- `oauth2_webfinger_oidc_discovery_auth_url` (String) Override the OAuth2 authorization URL in OIDC discovery.
- `oauth2_webfinger_oidc_discovery_client_registration_url` (String) Override the dynamic client registration URL in OIDC discovery.
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
- `selfservice_flows_login_after_passkey_default_browser_return_url` (String) Return URL after login via passkey.
- `selfservice_flows_login_after_password_default_browser_return_url` (String) Return URL after login via password.
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
- `selfservice_flows_registration_after_passkey_default_browser_return_url` (String) Return URL after registration via passkey.
- `selfservice_flows_registration_after_password_default_browser_return_url` (String) Return URL after registration via password.
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
- `selfservice_methods_captcha_config_allowed_domains` (List of String) Domains allowed for CAPTCHA verification.
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
- `selfservice_methods_link_config_base_url` (String) Base URL for recovery, verification, and login links. Leave empty for automatic detection.
- `selfservice_methods_link_config_lifespan` (String) Lifespan of magic links (e.g. '1h').
- `selfservice_methods_link_enabled` (Boolean) Enable the magic link authentication method.
- `selfservice_methods_lookup_secret_enabled` (Boolean) Enable backup/recovery codes.
- `selfservice_methods_oidc_config_base_redirect_uri` (String) Base redirect URI for OIDC/social sign-in callbacks.
- `selfservice_methods_oidc_enable_auto_link_policy` (Boolean) Enable the OIDC auto-link policy. When true, social sign-in providers with auto_link enabled (on ory_social_provider) can automatically link to existing identities that share the same identifier (e.g., email).
- `selfservice_methods_oidc_enabled` (Boolean) Enable OIDC (OpenID Connect) social sign-in. Must be enabled for social providers (e.g. Google, GitHub) to work.
- `selfservice_methods_passkey_config_rp_display_name` (String) Passkey relying party display name.
- `selfservice_methods_passkey_config_rp_id` (String) Passkey relying party ID (typically your domain).
- `selfservice_methods_passkey_config_rp_origins` (List of String) Allowed origins for passkey relying party verification.
- `selfservice_methods_passkey_enabled` (Boolean) Enable Passkey authentication.
- `selfservice_methods_password_config_haveibeenpwned_enabled` (Boolean) Check passwords against HaveIBeenPwned.
- `selfservice_methods_password_config_identifier_similarity_check_enabled` (Boolean) Check password similarity to identifier.
- `selfservice_methods_password_config_ignore_network_errors` (Boolean) Ignore HaveIBeenPwned network errors during password validation.
- `selfservice_methods_password_config_max_breaches` (Number) Maximum allowed breaches in HaveIBeenPwned.
- `selfservice_methods_password_config_min_password_length` (Number) Minimum password length.
- `selfservice_methods_password_enabled` (Boolean) Enable password authentication.
- `selfservice_methods_profile_enabled` (Boolean) Enable the profile authentication method. When enabled, users can update their identity traits (e.g., name, address) via the settings flow.
- `selfservice_methods_saml_enabled` (Boolean) Enable SAML login method.
- `selfservice_methods_totp_config_issuer` (String) TOTP issuer name shown in authenticator apps.
- `selfservice_methods_totp_enabled` (Boolean) Enable TOTP (Time-based One-Time Password).
- `selfservice_methods_webauthn_config_passwordless` (Boolean) Enable passwordless WebAuthn authentication.
- `selfservice_methods_webauthn_config_rp_display_name` (String) WebAuthn Relying Party display name.
- `selfservice_methods_webauthn_config_rp_icon` (String) Deprecated. WebAuthn relying party icon URL (ignored for security reasons).
- `selfservice_methods_webauthn_config_rp_id` (String) WebAuthn Relying Party ID (typically your domain).
- `selfservice_methods_webauthn_enabled` (Boolean) Enable WebAuthn (hardware keys).
- `session_cookie_persistent` (Boolean) Enable persistent session cookies (survive browser close).
- `session_cookie_same_site` (String) SameSite cookie attribute (Lax, Strict, None).
- `session_lifespan` (String) Session duration (e.g., '24h0m0s').
- `session_tokenizer_templates` (Attributes Map) JWT tokenizer templates for the /sessions/whoami endpoint. Each key is a template name, and the value configures how JWTs are generated. (see [below for nested schema](#nestedatt--session_tokenizer_templates))
- `session_whoami_required_aal` (String) Required AAL for session whoami endpoint: 'aal1', 'aal2', or 'highest_available'.
- `settings_lifespan` (String, Deprecated) Lifespan of the settings flow (e.g., '30m0s'). Controls how long a settings flow session remains valid.
- `settings_privileged_session_max_age` (String, Deprecated) Maximum age of a privileged session for the settings flow (e.g., '15m0s'). After this duration, the user must re-authenticate to make privileged changes like password updates.
- `settings_ui_url` (String, Deprecated) URL for the account settings UI.
- `smtp_connection_uri` (String, Sensitive) SMTP connection URI for sending emails (e.g., smtps://user:pass@host:port).
- `smtp_from_address` (String, Deprecated) Email address to send from.
- `smtp_from_name` (String, Deprecated) Name to display as sender.
- `smtp_headers` (Map of String) Custom SMTP headers for outbound emails.
- `totp_issuer` (String, Deprecated) TOTP issuer name shown in authenticator apps.
- `verification_lifespan` (String, Deprecated) Lifespan of the verification flow (e.g., '30m0s'). Controls how long a verification flow session remains valid.
- `verification_notify_unknown_recipients` (Boolean, Deprecated) When enabled, verification emails are sent even if the email address is not associated with any known identity.
- `verification_ui_url` (String, Deprecated) URL for the verification UI.
- `verification_use` (String, Deprecated) Verification method to use: 'code' (one-time code) or 'link' (magic link).
- `webauthn_passwordless` (Boolean, Deprecated) Enable passwordless WebAuthn authentication.
- `webauthn_rp_display_name` (String, Deprecated) WebAuthn Relying Party display name.
- `webauthn_rp_id` (String, Deprecated) WebAuthn Relying Party ID (typically your domain).
- `webauthn_rp_origins` (List of String) Allowed origins for WebAuthn relying party.

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



<a id="nestedatt--session_tokenizer_templates"></a>
### Nested Schema for `session_tokenizer_templates`

Required:

- `jwks_url` (String, Sensitive) JWKS URL for signing tokens. Must use base64:// scheme (e.g., 'base64://eyJrZXlzIjpbXX0=').

Optional:

- `claims_mapper_url` (String) Jsonnet claims mapper URL. Supports base64:// and https:// schemes.
- `subject_source` (String) Subject source for the JWT: 'id' (default) or 'external_id'.
- `ttl` (String) Token time-to-live duration (e.g., '1h', '30m'). Default: '1m'.
