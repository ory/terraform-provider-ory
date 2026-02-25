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

### Resources

- **Project & Workspace Management**: Create and manage Ory Network projects and workspaces
- **Project Configuration**: CORS, session settings, password policies, MFA, OAuth2/Hydra settings
- **Identity Management**: Create and manage user identities with custom schemas
- **Identity Schemas**: Define custom identity schemas for your project
- **Authentication Flows**: Configure social providers (Google, GitHub, Microsoft, Apple, OIDC)
- **Webhooks/Actions**: Trigger webhooks on identity flow events
- **Email Templates**: Customize verification, recovery, and login code emails
- **OAuth2 Clients**: Manage OAuth2/OIDC client applications and dynamic client registration (RFC 7591)
- **JSON Web Key Sets**: Manage JWK sets for token signing and verification
- **JWT Grant Trust**: Trust external identity providers for RFC 7523 JWT Bearer grants
- **Event Streams**: Publish Ory events to external systems like AWS SNS (Enterprise)
- **Organizations**: Multi-tenancy support for B2B applications
- **Permissions (Keto)**: Manage relationship tuples for fine-grained authorization
- **API Key Management**: Manage project API keys

### Data Sources

- **Project** / **Workspace** — Look up existing projects and workspaces
- **Identity** — Query user identities
- **Identity Schemas** — List available identity schemas
- **OAuth2 Client** — Look up OAuth2 client details
- **Organization** — Look up organization details

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
make build
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
  scope         = ["email", "profile"]
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

## Development

### Setup

```bash
git clone https://github.com/ory/terraform-provider-ory.git
cd terraform-provider-ory

# Install dependencies and development tools (linters, doc generators, security scanners)
make deps

# Set up git hooks (conventional commit validation, pre-push checks)
git config core.hooksPath .githooks

# Build the provider
make build

# Install to local Terraform plugins directory
make install
```

### Testing

#### Unit Tests

Unit tests run without any credentials:

```bash
make test           # Run all unit tests
make test-short     # Run unit tests in short mode
```

#### Acceptance Tests

Acceptance tests run against a **pre-created Ory project**. Copy `.env.example` to `.env` and fill in your credentials:

```bash
cp .env.example .env
```

At minimum you need:

```bash
# Workspace credentials
ORY_WORKSPACE_API_KEY=ory_wak_...
ORY_WORKSPACE_ID=...

# Pre-created test project
ORY_PROJECT_ID=...
ORY_PROJECT_SLUG=...
ORY_PROJECT_API_KEY=ory_pat_...
ORY_PROJECT_ENVIRONMENT=prod
```

The `.env` file is gitignored and automatically loaded by `make` targets.

```bash
make test-acc              # Standard acceptance tests
make test-acc-verbose      # With debug logging
make test-acc-keto         # Run only Keto/relationship tests
make test-acc-all          # All tests with all features enabled
```

Or run directly with `go test`:

```bash
# Acceptance tests
TF_ACC=1 go test -tags acceptance -p 1 -v -timeout 30m ./...

# Specific resource tests
TF_ACC=1 go test -tags acceptance -p 1 -v ./internal/resources/identity/...
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
| `ORY_EVENT_STREAM_TESTS_ENABLED=true`    | Run Event Stream tests (requires Enterprise plan)   | Skipped  |

#### Test Coverage by Plan

| Test Suite                      | Free Plan | Growth Plan | Enterprise |
| ------------------------------- | --------- | ----------- | ---------- |
| Identity                        | ✅        | ✅          | ✅         |
| OAuth2 Client                   | ✅        | ✅          | ✅         |
| OIDC Dynamic Client             | ✅        | ✅          | ✅         |
| Project Config                  | ✅        | ✅          | ✅         |
| Action (webhooks)               | ✅        | ✅          | ✅         |
| Email Template                  | ✅        | ✅          | ✅         |
| Social Provider                 | ✅        | ✅          | ✅         |
| JWK                             | ✅        | ✅          | ✅         |
| Trusted JWT Grant Issuer        | ✅        | ✅          | ✅         |
| Organization                    | ❌        | ✅\*        | ✅         |
| Relationship (Keto)             | ❌        | ✅          | ✅         |
| Event Stream                    | ❌        | ❌          | ✅         |

\*Organizations require B2B features to be enabled on your plan.

### Duration Format

Time-based attributes (e.g., `session_lifespan`, `oauth2_access_token_lifespan`) use Go duration strings. The Ory API normalizes durations on write, so use the full normalized format to avoid perpetual diffs in `terraform plan`:

| Write | API Returns | Use in Config |
|-------|-------------|---------------|
| `1h` | `1h0m0s` | `1h0m0s` |
| `30m` | `30m0s` | `30m0s` |
| `720h` | `720h0m0s` | `720h0m0s` |

### Known Limitations

- `ory_identity_schema`: Content is immutable; changes require resource replacement. Delete not supported by Ory API (resource removed from state only).
- `ory_workspace`: Delete not supported by Ory API.
- `ory_oauth2_client`: `client_secret` only returned on initial creation.
- `ory_email_template`: Delete resets to Ory defaults rather than removing.
- `ory_relationship`: Requires Ory Permissions (Keto) to be enabled on the project.
- `ory_project_config`: Cannot be deleted — it always exists for a project. Only attributes present in your Terraform configuration are tracked for drift.

### Documentation

Documentation is auto-generated from **templates** using [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs). Do NOT edit files in `docs/` directly — they are overwritten on every build.

**To update documentation:**

1. Edit the templates in `templates/` (e.g., `templates/resources/oauth2_client.md.tmpl`)
2. Edit example files in `examples/` (e.g., `examples/resources/ory_oauth2_client/resource.tf`)
3. Run `make format` to regenerate `docs/` from the templates

Templates use Go template syntax with these variables:
- `{{ .SchemaMarkdown | trimspace }}` — auto-generated schema table from Go code
- `{{ tffile "examples/resources/ory_foo/resource.tf" }}` — embed example files
- `{{ .Name }}`, `{{ .Type }}` — resource name and type

```
templates/
├── index.md.tmpl                                  # Provider-level docs
├── resources/                                     # 16 resource templates
│   ├── action.md.tmpl
│   ├── email_template.md.tmpl
│   ├── event_stream.md.tmpl
│   ├── identity.md.tmpl
│   ├── identity_schema.md.tmpl
│   ├── json_web_key_set.md.tmpl
│   ├── oauth2_client.md.tmpl
│   ├── oidc_dynamic_client.md.tmpl
│   ├── organization.md.tmpl
│   ├── project.md.tmpl
│   ├── project_api_key.md.tmpl
│   ├── project_config.md.tmpl
│   ├── relationship.md.tmpl
│   ├── social_provider.md.tmpl
│   ├── trusted_oauth2_jwt_grant_issuer.md.tmpl
│   └── workspace.md.tmpl
└── data-sources/                                  # 6 data source templates
    ├── identity.md.tmpl
    ├── identity_schemas.md.tmpl
    ├── oauth2_client.md.tmpl
    ├── organization.md.tmpl
    ├── project.md.tmpl
    └── workspace.md.tmpl
```

### Pre-Commit Checklist

Run these checks locally before committing. They mirror what CI runs on every push.

```bash
# Minimum before committing:
make build && make format && make test

# Full CI-equivalent check:
make build && make format && make test && make sec && make licenses
```

`make format` runs several tools in sequence: `go fmt`, `gofmt -s`, `terraform fmt`, `go mod tidy`, `tfplugindocs generate`, and `golangci-lint --fix`.

### Security Scanning

```bash
make sec            # Run all security scans
make sec-vuln       # govulncheck — known Go vulnerabilities
make sec-gosec      # gosec — Go security patterns
make sec-gitleaks   # gitleaks — hardcoded secrets
make sec-trivy      # trivy — vulnerability and misconfig scanning
```

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines on development setup, testing, writing acceptance tests, and the contribution checklist.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Run checks: `make build && make format && make test`
4. Commit using [Conventional Commits](https://www.conventionalcommits.org/) format
5. Open a Pull Request

## Related Links

- [Ory Network Documentation](https://www.ory.sh/docs/)
- [Ory API Reference](https://www.ory.sh/docs/reference/api)
- [Terraform Provider Development](https://developer.hashicorp.com/terraform/plugin)
