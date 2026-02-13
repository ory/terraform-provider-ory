---
page_title: "ory_email_template Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Network email template.
---

# ory_email_template (Resource)

Manages an Ory Network email template.

Email templates use [Go template syntax](https://pkg.go.dev/text/template) for variable substitution. HTML bodies use `html/template` (auto-escaping) and plaintext bodies use `text/template`. [Sprig template functions](http://masterminds.github.io/sprig/) are available, except date, random, OS, and network functions.

## Template Types

| Template Type | UI Name | Description |
|---------------|---------|-------------|
| `registration_code_valid` | Registration via Code | Sent when user registers with a valid code |
| `registration_code_invalid` | - | Sent when registration code is invalid/expired |
| `login_code_valid` | Login via Code | Sent when user logs in with a valid code |
| `login_code_invalid` | - | Sent when login code is invalid/expired |
| `verification_code_valid` | Verification via Code (Valid) | Sent for email verification with valid code |
| `verification_code_invalid` | - | Sent when verification code is invalid/expired |
| `recovery_code_valid` | Recovery via Code (Valid) | Sent for account recovery with valid code |
| `recovery_code_invalid` | - | Sent when recovery code is invalid/expired |
| `verification_valid` | - | Legacy verification email (link-based) |
| `verification_invalid` | - | Legacy verification invalid |
| `recovery_valid` | - | Legacy recovery email (link-based) |
| `recovery_invalid` | - | Legacy recovery invalid |

**Note:** The "_invalid" templates are sent when a code has expired or is incorrect. The non-code variants (recovery_valid, verification_valid) are for legacy link-based flows.

## Template Variables

Each template type has access to different variables:

| Template | Available Variables |
|----------|-------------------|
| `recovery_code_valid` | `.To`, `.RecoveryCode`, `.Identity`, `.ExpiresInMinutes` |
| `recovery_code_invalid` | `.To` |
| `verification_code_valid` | `.To`, `.VerificationCode`, `.VerificationURL`, `.Identity`, `.ExpiresInMinutes` |
| `verification_code_invalid` | `.To` |
| `login_code_valid` | `.To`, `.LoginCode`, `.Identity`, `.ExpiresInMinutes` |
| `login_code_invalid` | `.To` |
| `registration_code_valid` | `.To`, `.RegistrationCode`, `.Traits`, `.ExpiresInMinutes` |
| `registration_code_invalid` | `.To` |
| `recovery_valid` | `.To`, `.RecoveryURL`, `.Identity`, `.ExpiresInMinutes` |
| `verification_valid` | `.To`, `.VerificationURL`, `.Identity`, `.ExpiresInMinutes` |

The `.Identity` object provides access to `.Identity.traits` and `.Identity.metadata_public`.

## Important Behaviors

- **Destroying resets to defaults:** Deleting this resource resets the template to Ory's built-in default template. It does not leave a blank template.
- **Base64 encoding is automatic:** You provide raw template content; the provider handles base64 encoding internally.
- **Subject is optional:** If not specified, Ory uses a default subject line.

## Example Usage

```terraform
# Recovery code email
resource "ory_email_template" "recovery_code" {
  template_type = "recovery_code_valid"
  subject       = "Reset your password"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <head>
      <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .code { font-size: 32px; font-weight: bold; color: #0066cc; letter-spacing: 4px; }
      </style>
    </head>
    <body>
      <div class="container">
        <h1>Password Reset</h1>
        <p>Hi {{ .Identity.traits.name.first }},</p>
        <p>Your recovery code is:</p>
        <p class="code">{{ .RecoveryCode }}</p>
        <p>This code expires in 1 hour.</p>
        <p>If you didn't request this, please ignore this email.</p>
      </div>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Password Reset

    Hi {{ .Identity.traits.name.first }},

    Your recovery code is: {{ .RecoveryCode }}

    This code expires in 1 hour.

    If you didn't request this, please ignore this email.
  TEXT
}

# Verification code email
resource "ory_email_template" "verification_code" {
  template_type = "verification_code_valid"
  subject       = "Verify your email address"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <body>
      <h1>Email Verification</h1>
      <p>Welcome! Please verify your email address.</p>
      <p>Your verification code is: <strong>{{ .VerificationCode }}</strong></p>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Email Verification

    Welcome! Please verify your email address.

    Your verification code is: {{ .VerificationCode }}
  TEXT
}

# Login code email
resource "ory_email_template" "login_code" {
  template_type = "login_code_valid"
  subject       = "Your login code"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <body>
      <h1>Login Code</h1>
      <p>Your login code is: <strong>{{ .LoginCode }}</strong></p>
      <p>This code expires in 15 minutes.</p>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Login Code

    Your login code is: {{ .LoginCode }}

    This code expires in 15 minutes.
  TEXT
}

# Registration code email
resource "ory_email_template" "registration_code" {
  template_type = "registration_code_valid"
  subject       = "Complete your registration"

  body_html = <<-HTML
    <!DOCTYPE html>
    <html>
    <body>
      <h1>Welcome!</h1>
      <p>Your registration code is: <strong>{{ .RegistrationCode }}</strong></p>
      <p>Enter this code to complete your account setup.</p>
    </body>
    </html>
  HTML

  body_plaintext = <<-TEXT
    Welcome!

    Your registration code is: {{ .RegistrationCode }}

    Enter this code to complete your account setup.
  TEXT
}
```

## Import

```shell
terraform import ory_email_template.welcome registration_code_valid
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `body_html` (String) HTML body template (Go template syntax).
- `body_plaintext` (String) Plaintext body template (Go template syntax).
- `template_type` (String) The email template type. See the Template Types table above for valid values and their UI equivalents. Common values: `registration_code_valid`, `login_code_valid`, `verification_code_valid`, `recovery_code_valid`.

### Optional

- `project_id` (String) Project ID. If not set, uses provider's project_id.
- `subject` (String) Email subject template (Go template syntax).

### Read-Only

- `id` (String) Resource ID (same as template_type).
