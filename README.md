# Terraform Provider for Ory Network

[![Go Reference](https://pkg.go.dev/badge/github.com/ory/terraform-provider-ory.svg)](https://pkg.go.dev/github.com/ory/terraform-provider-ory)
[![Go Report Card](https://goreportcard.com/badge/github.com/ory/terraform-provider-ory)](https://goreportcard.com/report/github.com/ory/terraform-provider-ory)

> **Special Thanks**
> Shoutout to [Jason Hernandez](https://github.com/jasonhernandez) and the [Materialize](https://materialize.com/) team for creating the initial version of this provider! Also see [NOTICE.md](./NOTICE.md)

## License

This project is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.

A Terraform provider for managing [Ory Network](https://www.ory.sh/) resources using infrastructure-as-code.

> **Note**: This provider is for **Ory Network** (the managed SaaS offering) only. It does not support self-hosted Ory deployments.

## Features

- **Identity Management**: Create and manage user identities with custom schemas
- **Authentication Flows**: Configure social providers (Google, GitHub, Microsoft, Apple, OIDC)
- **Project Configuration**: CORS, session settings, password policies, MFA
- **Webhooks/Actions**: Trigger webhooks on identity flow events
- **Email Templates**: Customize verification, recovery, and login code emails
- **OAuth2 Clients**: Manage OAuth2/OIDC client applications
- **Organizations**: Multi-tenancy support for B2B applications
- **Permissions (Keto)**: Manage relationship tuples for fine-grained authorization

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for building from source)
- An [Ory Network](https://console.ory.sh/) account

## Installation

### From Terraform Registry (Recommended)

```hcl
terraform {
  required_providers {
    ory = {
      source = "ory/ory"
    }
  }
}
```

### From Source

```bash
git clone https://github.com/ory/terraform-provider-ory.git
cd terraform-provider-ory
go build -o terraform-provider-ory
```

Then configure Terraform to use the local provider:

```hcl
# ~/.terraformrc
provider_installation {
  dev_overrides {
    "ory/ory" = "/path/to/terraform-provider-ory"
  }
  direct {}
}
```

## Authentication

Ory Network uses two types of API keys:

| Key Type          | Prefix        | Purpose                                       |
| ----------------- | ------------- | --------------------------------------------- |
| Workspace API Key | `ory_wak_...` | Projects, organizations, workspace management |
| Project API Key   | `ory_pat_...` | Identities, OAuth2 clients, relationships     |

### Environment Variables (Recommended)

```bash
export ORY_WORKSPACE_API_KEY="ory_wak_..."
export ORY_PROJECT_API_KEY="ory_pat_..."
export ORY_PROJECT_ID="your-project-uuid"
export ORY_PROJECT_SLUG="your-project-slug"  # e.g., "vibrant-moore-abc123"
```

### Provider Configuration

```hcl
provider "ory" {
  workspace_api_key = var.ory_workspace_key  # or ORY_WORKSPACE_API_KEY env var
  project_api_key   = var.ory_project_key    # or ORY_PROJECT_API_KEY env var
  project_id        = var.ory_project_id     # or ORY_PROJECT_ID env var
  project_slug      = var.ory_project_slug   # or ORY_PROJECT_SLUG env var
}
```

## Quick Start

```hcl
terraform {
  required_providers {
    ory = {
      source = "ory/ory"
    }
  }
}

provider "ory" {}

# Configure project settings
resource "ory_project_config" "main" {
  cors_enabled         = true
  cors_origins         = ["https://app.example.com"]
  password_min_length  = 10
  session_lifespan     = "720h"  # 30 days
}

# Add Google social login
resource "ory_social_provider" "google" {
  provider_id   = "google"
  client_id     = var.google_client_id
  client_secret = var.google_client_secret
  scopes        = ["email", "profile"]
}

# Create a webhook for new registrations
resource "ory_action" "welcome_email" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/welcome"
  method      = "POST"
}
```

## Resources

| Resource                                                     | Description                          |
| ------------------------------------------------------------ | ------------------------------------ |
| [`ory_project`](docs/resources/project.md)                   | Ory Network projects                 |
| [`ory_workspace`](docs/resources/workspace.md)               | Ory workspaces                       |
| [`ory_organization`](docs/resources/organization.md)         | Organizations for multi-tenancy      |
| [`ory_identity`](docs/resources/identity.md)                 | User identities                      |
| [`ory_identity_schema`](docs/resources/identity_schema.md)   | Custom identity schemas              |
| [`ory_oauth2_client`](docs/resources/oauth2_client.md)       | OAuth2/OIDC client applications      |
| [`ory_project_config`](docs/resources/project_config.md)     | Project configuration settings       |
| [`ory_action`](docs/resources/action.md)                     | Webhooks for identity flows          |
| [`ory_social_provider`](docs/resources/social_provider.md)   | Social sign-in providers             |
| [`ory_email_template`](docs/resources/email_template.md)     | Email template customization         |
| [`ory_project_api_key`](docs/resources/project_api_key.md)   | Project API keys                     |
| [`ory_json_web_key_set`](docs/resources/json_web_key_set.md) | JSON Web Key Sets for signing        |
| [`ory_relationship`](docs/resources/relationship.md)         | Ory Permissions (Keto) relationships |

## Data Sources

| Data Source                                   | Description              |
| --------------------------------------------- | ------------------------ |
| [`ory_project`](docs/data-sources/project.md) | Read project information |

## Examples

### Multi-Tenant B2B Setup

```hcl
# Create organizations for each tenant
resource "ory_organization" "acme" {
  label   = "Acme Corporation"
  domains = ["acme.com"]
}

resource "ory_organization" "globex" {
  label   = "Globex Inc"
  domains = ["globex.com"]
}

# Identity schema with organization support
resource "ory_identity_schema" "customer" {
  name = "customer_v1"
  schema = jsonencode({
    "$id"     = "https://example.com/customer.schema.json"
    "$schema" = "http://json-schema.org/draft-07/schema#"
    title     = "Customer"
    type      = "object"
    properties = {
      traits = {
        type = "object"
        properties = {
          email = {
            type   = "string"
            format = "email"
            "ory.sh/kratos" = {
              credentials = { password = { identifier = true } }
              verification = { via = "email" }
              recovery     = { via = "email" }
            }
          }
          name = {
            type = "object"
            properties = {
              first = { type = "string" }
              last  = { type = "string" }
            }
          }
        }
        required = ["email"]
      }
    }
  })
}
```

### OAuth2 Client for Machine-to-Machine

```hcl
resource "ory_oauth2_client" "api_service" {
  client_name   = "API Service"
  grant_types   = ["client_credentials"]
  token_endpoint_auth_method = "client_secret_post"
  scope         = "read write"
}

output "client_id" {
  value = ory_oauth2_client.api_service.client_id
}

output "client_secret" {
  value     = ory_oauth2_client.api_service.client_secret
  sensitive = true
}
```

### MFA Configuration

```hcl
resource "ory_project_config" "secure" {
  # Enable TOTP (authenticator apps)
  enable_totp = true
  totp_issuer = "MyApp"

  # Enable WebAuthn (security keys, passkeys)
  enable_webauthn           = true
  webauthn_rp_display_name  = "MyApp"
  webauthn_rp_id            = "app.example.com"
  webauthn_rp_origins       = ["https://app.example.com"]
  webauthn_passwordless     = true

  # Require MFA for all users
  required_aal = "aal2"
}
```

### Custom Email Templates

```hcl
resource "ory_email_template" "recovery" {
  template_type = "recovery_code_valid"
  subject       = "Reset your password"
  body_html     = <<-HTML
    <h1>Password Reset</h1>
    <p>Hi {{ .Identity.traits.name.first }},</p>
    <p>Your recovery code is: <strong>{{ .RecoveryCode }}</strong></p>
  HTML
  body_plaintext = <<-TEXT
    Password Reset

    Hi {{ .Identity.traits.name.first }},

    Your recovery code is: {{ .RecoveryCode }}
  TEXT
}
```

## Known Limitations

| Resource              | Limitation                                                                          |
| --------------------- | ----------------------------------------------------------------------------------- |
| `ory_organization`    | Requires B2B features AND project environment must be `prod` or `stage` (not `dev`) |
| `ory_identity_schema` | Immutable - content cannot be updated after creation                                |
| `ory_identity_schema` | Delete not supported by Ory API (resource removed from state only)                  |
| `ory_workspace`       | Delete not supported by Ory API                                                     |
| `ory_oauth2_client`   | `client_secret` only returned on create                                             |
| `ory_email_template`  | Delete resets to Ory defaults                                                       |
| `ory_relationship`    | Requires Ory Permissions (Keto) to be enabled                                       |

## Development

### Building

```bash
go build -o terraform-provider-ory
```

### Testing

Acceptance tests are **self-contained** - they automatically create a temporary Ory project, run tests against it, and clean up when done.

#### Required Environment Variables

```bash
# Required for acceptance tests
export ORY_WORKSPACE_API_KEY="ory_wak_..."  # Workspace API key
export ORY_WORKSPACE_ID="..."                # Workspace ID
```

#### Running Tests

```bash
# Unit tests only (no credentials needed)
make test

# All acceptance tests (creates temp project, runs tests, cleans up)
make test-acc

# Acceptance tests with debug logging
make test-acc-verbose

# Only Keto/relationship tests
make test-acc-keto

# All tests with all features enabled
make test-acc-all
```

Or run directly with go test:

```bash
# Unit tests
go test -short ./...

# Acceptance tests
TF_ACC=1 go test -p 1 -v -timeout 30m ./...

# Specific resource tests
TF_ACC=1 go test -p 1 -v ./internal/resources/identity/...
TF_ACC=1 go test -p 1 -v ./internal/resources/oauth2client/...
```

#### Optional Test Feature Flags

Some tests require additional feature flags or specific Ory plan features:

| Environment Variable                     | Purpose                                             | Default  |
| ---------------------------------------- | --------------------------------------------------- | -------- |
| `TF_ACC=1`                               | Enable acceptance tests                             | Required |
| `ORY_KETO_TESTS_ENABLED=true`            | Run Relationship tests (requires Keto)              | Skipped  |
| `ORY_B2B_ENABLED=true`                   | Run Organization tests (requires B2B plan)          | Skipped  |
| `ORY_SOCIAL_PROVIDER_TESTS_ENABLED=true` | Run social provider tests                           | Skipped  |
| `ORY_SCHEMA_TESTS_ENABLED=true`          | Run IdentitySchema tests (schemas can't be deleted) | Skipped  |
| `ORY_PROJECT_TESTS_ENABLED=true`         | Run Project create/delete tests                     | Skipped  |

#### Test Coverage by Plan

| Test Suite          | Free Plan | Growth Plan | Enterprise |
| ------------------- | --------- | ----------- | ---------- |
| Identity            | ✅        | ✅          | ✅         |
| OAuth2 Client       | ✅        | ✅          | ✅         |
| Project Config      | ✅        | ✅          | ✅         |
| Action (webhooks)   | ✅        | ✅          | ✅         |
| Email Template      | ✅        | ✅          | ✅         |
| Social Provider     | ✅        | ✅          | ✅         |
| JWK                 | ✅        | ✅          | ✅         |
| Organization        | ❌        | ✅\*        | ✅         |
| Relationship (Keto) | ❌        | ✅          | ✅         |

\*Organizations require B2B features to be enabled on your plan.

### Generating Documentation

```bash
go generate ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Related Links

- [Ory Network Documentation](https://www.ory.sh/docs/)
- [Ory API Reference](https://www.ory.sh/docs/reference/api)
- [Terraform Provider Development](https://developer.hashicorp.com/terraform/plugin)
