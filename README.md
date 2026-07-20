# Terraform Provider for Ory Network

[![Go Reference](https://pkg.go.dev/badge/github.com/ory/terraform-provider-ory.svg)](https://pkg.go.dev/github.com/ory/terraform-provider-ory)
[![Go Report Card](https://goreportcard.com/badge/github.com/ory/terraform-provider-ory)](https://goreportcard.com/report/github.com/ory/terraform-provider-ory)

> **Special Thanks**
> Shoutout to [Jason Hernandez](https://github.com/jasonhernandez) and the [Materialize](https://materialize.com/) team for creating the initial version of this provider! Also see [NOTICE.md](./NOTICE.md)

A Terraform provider for managing [Ory Network](https://www.ory.sh/) resources using infrastructure-as-code.

> **Note**: This provider is for **Ory Network** (the managed SaaS offering) only. It does not support self-hosted Ory deployments.

# NOTICE - All remating deprecated attributes will be cleaned up on `v26.4.0`

## Migrating Deprecated `ory_project_config` Attributes

Many attributes in the `ory_project_config` resource have been renamed to follow the OpenAPI spec naming convention. The old names still work but will show deprecation warnings in Terraform output and will be removed in a future major version. Run `./scripts/migrate-deprecated-attrs.sh` to see the full list of renames.

**Examples of renamed attributes:**

| Old Name | New Name |
|----------|----------|
| `enable_password` | `selfservice_methods_password_enabled` |
| `login_ui_url` | `selfservice_flows_login_ui_url` |
| `oauth2_access_token_lifespan` | `oauth2_ttl_access_token` |
| `password_min_length` | `selfservice_methods_password_config_min_password_length` |
| `smtp_from_address` | `courier_smtp_from_address` |

To migrate your `.tf` files automatically, run the provided migration script:

```bash
./scripts/migrate-deprecated-attrs.sh /path/to/your/terraform/configs
```

The script creates `.bak` backups of each modified file. After migrating, run `terraform plan` to verify no changes are detected.

For the full list of renamed attributes, see the [project_config resource docs](docs/resources/project_config.md).

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://golang.org/doc/install) (see version in `go.mod`; for building from source)
- An [Ory Network](https://console.ory.sh/) account

## Installation

```hcl
terraform {
  required_providers {
    ory = {
      source = "ory/ory"
    }
  }
}
```

## Authentication

Ory Network uses two types of API keys:

| Key Type          | Prefix        | Purpose                                       |
| ----------------- | ------------- | --------------------------------------------- |
| Workspace API Key | `ory_wak_...` | Projects, organizations, workspace management |
| Project API Key   | `ory_pat_...` | Identities, OAuth2 clients, relationships     |

```bash
export ORY_WORKSPACE_API_KEY="ory_wak_..."
export ORY_PROJECT_API_KEY="ory_pat_..."
export ORY_PROJECT_ID="your-project-uuid"
export ORY_PROJECT_SLUG="your-project-slug"
```

Or configure directly in the provider block:

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
  session_lifespan     = "720h0m0s"  # 30 days
}

# Add Google social login
resource "ory_social_provider" "google" {
  provider_id   = "google"
  provider_type = "google"
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

For all available resources, data sources, and their attributes, see the [Terraform Registry documentation](https://registry.terraform.io/providers/ory/ory/latest/docs) or browse the `examples/` directory.

## Importing an Existing Project

Already have an Ory Network project configured through the Console? You can adopt Terraform without recreating anything:

```bash
export ORY_WORKSPACE_API_KEY=ory_wak_...
export ORY_PROJECT_API_KEY=ory_pat_...   # optional: covers OAuth2 clients, JWKS, ...
export ORY_PROJECT_ID=<project-uuid>

# Inventory the project and emit import blocks for everything importable
./.claude/skills/import-existing-project/scripts/generate-imports.sh > imports.tf

# Generate matching resource configuration, then refine until `terraform plan` is clean
terraform plan -generate-config-out=generated.tf
```

The full runbook — per-resource discovery endpoints, import ID formats, what cannot be imported (identity schemas, API key values, secrets), and how to refine the generated config — lives in [`.claude/skills/import-existing-project/SKILL.md`](.claude/skills/import-existing-project/SKILL.md). It is packaged as an agent skill, so coding agents pick it up automatically from `.claude/skills/`, and it doubles as a step-by-step guide for humans.

## Documentation

Documentation is auto-generated from templates in `templates/` using [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs). Do NOT edit files in `docs/` directly — they are overwritten by `make format`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and contribution guidelines.

## License

Apache License, Version 2.0. See [LICENSE](LICENSE).

## Related Links

- [Ory Network Documentation](https://www.ory.sh/docs/)
- [Ory API Reference](https://www.ory.sh/docs/reference/api)
- [Terraform Provider Development](https://developer.hashicorp.com/terraform/plugin)
