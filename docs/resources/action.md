---
page_title: "ory_action Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Action (webhook) for identity flows.
---

# ory_action (Resource)

Manages an Ory Action (webhook) for identity flows.

Actions allow you to trigger webhooks at specific points in identity flows (login, registration, recovery, settings, verification).

-> **Plan:** Available on all Ory Network plans.

## Example Usage

```terraform
# Post-registration webhook
resource "ory_action" "welcome_email" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/welcome"
  method      = "POST"
  body        = <<-JSONNET
    function(ctx) {
      email: ctx.identity.traits.email,
      name: ctx.identity.traits.name,
      created_at: ctx.identity.created_at
    }
  JSONNET
}

# Pre-login validation
resource "ory_action" "validate_login" {
  flow          = "login"
  timing        = "before"
  url           = "https://api.example.com/webhooks/validate"
  method        = "POST"
  can_interrupt = true # Allow webhook to block login
}

# Webhook with write-only (ephemeral) authentication secrets sourced from Vault.
# The *_wo secrets are never stored in Terraform state or plan (Terraform 1.11+).
# Bump the matching *_wo_version whenever a secret rotates so Terraform re-sends it.
ephemeral "vault_kv_secret_v2" "webhook_auth" {
  mount = "secret"
  name  = "ory/webhook-auth"
}

resource "ory_action" "secured_webhook" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/secured"
  method      = "POST"

  webhook_auth_type                           = "basic_auth"
  webhook_auth_basic_auth_user                = "webhook-user"
  webhook_auth_basic_auth_password_wo         = ephemeral.vault_kv_secret_v2.webhook_auth.data["password"]
  webhook_auth_basic_auth_password_wo_version = "1"
}

# Async audit log (fire and forget)
resource "ory_action" "audit_log" {
  flow            = "settings"
  timing          = "after"
  auth_method     = "password"
  url             = "https://api.example.com/webhooks/audit"
  method          = "POST"
  response_ignore = true
}

# Post-verification sync
# The verification (and recovery) flow is not scoped by authentication method,
# so auth_method is omitted here — the hook always runs after verification.
resource "ory_action" "sync_verified" {
  flow   = "verification"
  timing = "after"
  url    = "https://api.example.com/webhooks/user-verified"
  method = "POST"
}

# Post-registration enrichment (parse response to modify identity)
resource "ory_action" "enrich_identity" {
  flow           = "registration"
  timing         = "after"
  auth_method    = "password"
  url            = "https://api.example.com/webhooks/enrich"
  method         = "POST"
  response_parse = true # Parse the webhook response to update identity traits
  body           = <<-JSONNET
    function(ctx) {
      identity_id: ctx.identity.id,
      email: ctx.identity.traits.email
    }
  JSONNET
}

# Webhook with basic auth
resource "ory_action" "with_basic_auth" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/secured"
  method      = "POST"

  webhook_auth_type                = "basic_auth"
  webhook_auth_basic_auth_user     = var.webhook_user
  webhook_auth_basic_auth_password = var.webhook_password
}

# Webhook with API key auth (sent as header)
resource "ory_action" "with_api_key" {
  flow        = "login"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/login"
  method      = "POST"

  webhook_auth_type          = "api_key"
  webhook_auth_api_key_name  = "X-API-KEY"
  webhook_auth_api_key_value = var.api_key
  webhook_auth_api_key_in    = "header"
}
```

## Authentication Methods

The `auth_method` attribute specifies which authentication method triggers the webhook. It applies only to `timing = "after"` webhooks on the `login`, `registration`, and `settings` flows.

| Value | Description |
|-------|-------------|
| `password` | Password-based authentication (default) |
| `oidc` | Social/OIDC authentication (Google, GitHub, etc.) |
| `code` | One-time code (magic link, OTP) |
| `webauthn` | Hardware security keys |
| `passkey` | Passkey authentication |
| `totp` | Time-based one-time password |
| `lookup_secret` | Recovery/backup codes |

~> **Note:** `auth_method` only applies to `timing = "after"` webhooks on the `login`, `registration`, and `settings` flows. The `recovery` and `verification` flows are **not** scoped by authentication method — their after-hooks always run — so `auth_method` is ignored for them and should be omitted. For `timing = "before"` hooks, the webhook runs before any authentication method is invoked.

## Webhook Authentication

Webhooks can be configured with authentication to secure the endpoint. Two types are supported:

### Basic Auth

```hcl
resource "ory_action" "secured_webhook" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/welcome"
  method      = "POST"

  webhook_auth_type                = "basic_auth"
  webhook_auth_basic_auth_user     = var.webhook_user
  webhook_auth_basic_auth_password = var.webhook_password
}
```

### API Key

```hcl
resource "ory_action" "api_key_webhook" {
  flow        = "login"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/login"
  method      = "POST"

  webhook_auth_type          = "api_key"
  webhook_auth_api_key_name  = "X-API-KEY"
  webhook_auth_api_key_value = var.api_key
  webhook_auth_api_key_in    = "header"
}
```

| Attribute | Description |
|-----------|-------------|
| `webhook_auth_type` | Authentication type: `basic_auth` or `api_key` |
| `webhook_auth_basic_auth_user` | Username for basic auth |
| `webhook_auth_basic_auth_password` | Password for basic auth (sensitive) |
| `webhook_auth_api_key_name` | Header or cookie name for the API key |
| `webhook_auth_api_key_value` | The API key value (sensitive) |
| `webhook_auth_api_key_in` | Where to send the API key: `header` or `cookie` |

## HTTP Method

The `method` attribute specifies the HTTP method used when calling the webhook:

| Value | Description |
|-------|-------------|
| `POST` | HTTP POST request (default) |
| `GET` | HTTP GET request |
| `PUT` | HTTP PUT request |
| `PATCH` | HTTP PATCH request |
| `DELETE` | HTTP DELETE request |

## Import

Actions must be imported with the HTTP method included in the import ID.

**For "after" timing (post-hooks):**
```shell
terraform import ory_action.example "project_id:flow:after:auth_method:method:url"
```

**For "before" timing (pre-hooks):**
```shell
terraform import ory_action.example "project_id:flow:before:method:url"
```

### Examples

```shell
# Import a POST webhook for post-registration password flow
terraform import ory_action.welcome \
  "550e8400-e29b-41d4-a716-446655440000:registration:after:password:POST:https://api.example.com/webhooks/welcome"

# Import a PATCH webhook for post-login social (OIDC) flow
terraform import ory_action.social_login \
  "550e8400-e29b-41d4-a716-446655440000:login:after:oidc:PATCH:https://api.example.com/webhooks/social"

# Import a POST pre-login validation webhook
terraform import ory_action.validate \
  "550e8400-e29b-41d4-a716-446655440000:login:before:POST:https://api.example.com/webhooks/validate"
```

### Finding Import Values from Ory Console

1. **project_id**: Settings → General → Project ID
2. **flow**: The flow type (login, registration, recovery, settings, verification)
3. **timing**: "before" or "after"
4. **auth_method**: for `login`/`registration`/`settings` "after" hooks, one of password, oidc, code, webauthn, passkey, totp, lookup_secret. For `recovery`/`verification` "after" hooks the value is ignored, but the import ID format still requires this segment — use `password` to match the provider default and avoid a post-import diff (keep `auth_method` omitted from the resource configuration itself).
5. **method**: The HTTP method (POST, GET, PUT, PATCH, DELETE)
6. **url**: The exact webhook URL - must match exactly including protocol and trailing slashes

### Troubleshooting Import Errors

If you see "Cannot import non-existent remote object", the import will show a warning listing webhooks found at that location. This helps you find the correct URL, method, and auth_method.

Common issues:
- **URL mismatch**: URLs must match exactly, including `https://` and any trailing `/`
- **Wrong method**: Check the HTTP method configured for the webhook (POST, PATCH, etc.)
- **Wrong auth_method**: Check which authentication method the webhook is configured for
- **Wrong timing**: Check if the webhook is a pre-hook (before) or post-hook (after)

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `flow` (String) Identity flow to hook into (login, registration, recovery, settings, verification).
- `timing` (String) When to trigger: 'before' (pre-hook) or 'after' (post-hook).
- `url` (String) Webhook URL to call.

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `auth_method` (String) Authentication method that triggers the webhook. In the Ory Console UI, this is the "Method" selector. Valid values: `password` (default), `oidc` (social login), `code` (magic link/OTP), `webauthn`, `passkey`, `totp`, `lookup_secret`. Only applies to `timing = "after"` webhooks on the `login`, `registration`, and `settings` flows; it is ignored for the `recovery` and `verification` flows.
- `body` (String) Jsonnet template for the request body.
- `can_interrupt` (Boolean) Allow webhook to interrupt/block the flow (default: false).
- `method` (String) HTTP method (default: POST).
- `project_id` (String) Project ID. If not set, uses provider's project_id.
- `response_ignore` (Boolean) Run webhook async without waiting (default: false).
- `response_parse` (Boolean) Parse response to modify identity (default: false).
- `webhook_auth_api_key_in` (String) Where to send the API key: 'header' or 'cookie'.
- `webhook_auth_api_key_name` (String) Header or cookie name for API key webhook authentication.
- `webhook_auth_api_key_value` (String, Sensitive) API key value for API key webhook authentication. Stored in Terraform state; for an ephemeral alternative that is never persisted, use webhook_auth_api_key_value_wo.
- `webhook_auth_api_key_value_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of webhook_auth_api_key_value (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the API key from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change webhook_auth_api_key_value_wo_version to rotate it. Mutually exclusive with webhook_auth_api_key_value.
- `webhook_auth_api_key_value_wo_version` (String) Version trigger for webhook_auth_api_key_value_wo. Change this value whenever the write-only API key changes so Terraform sends the new value to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless webhook_auth_api_key_value_wo is set.
- `webhook_auth_basic_auth_password` (String, Sensitive) Password for basic auth webhook authentication. Stored in Terraform state; for an ephemeral alternative that is never persisted, use webhook_auth_basic_auth_password_wo.
- `webhook_auth_basic_auth_password_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of webhook_auth_basic_auth_password (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the password from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change webhook_auth_basic_auth_password_wo_version to rotate it. Mutually exclusive with webhook_auth_basic_auth_password.
- `webhook_auth_basic_auth_password_wo_version` (String) Version trigger for webhook_auth_basic_auth_password_wo. Change this value whenever the write-only password changes so Terraform sends the new value to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless webhook_auth_basic_auth_password_wo is set.
- `webhook_auth_basic_auth_user` (String) Username for basic auth webhook authentication.
- `webhook_auth_type` (String) Webhook authentication type: 'basic_auth' or 'api_key'.

### Read-Only

- `id` (String) Resource ID.
