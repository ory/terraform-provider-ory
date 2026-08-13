---
page_title: "ory Provider"
description: |-
  The Ory provider enables Terraform to manage Ory Network resources.
---

# ory Provider

The Ory provider enables Terraform to manage [Ory Network](https://www.ory.sh/) resources.

## Authentication

Ory Network uses two types of API keys:

| API Key Type | Prefix | Used For |
|--------------|--------|----------|
| **Workspace API Key** | `ory_wak_...` | Projects, organizations, workspace management, project config, actions |
| **Project API Key** | `ory_pat_...` | Identities, OAuth2 clients, relationships |

## Configuration Options

There are two ways to configure the Ory provider:

### Option 1: Environment Variables (Recommended for CI/CD)

Set credentials as environment variables and use an empty provider block:

```bash
export ORY_WORKSPACE_API_KEY="ory_wak_..."
export ORY_WORKSPACE_ID="..."           # Required for creating new projects
export ORY_PROJECT_API_KEY="ory_pat_..."
export ORY_PROJECT_ID="..."
export ORY_PROJECT_SLUG="..."
```

```hcl
terraform {
  required_providers {
    ory = {
      source = "ory/ory"
    }
  }
}

provider "ory" {}  # Picks up from ORY_* environment variables
```

### Option 2: Terraform Variables (Recommended for tfvars)

Define variables and pass values via `terraform.tfvars` or `-var` flags:

```hcl
terraform {
  required_providers {
    ory = {
      source = "ory/ory"
    }
  }
}

provider "ory" {
  workspace_api_key = var.ory_workspace_api_key
  workspace_id      = var.ory_workspace_id
  project_api_key   = var.ory_project_api_key
  project_id        = var.ory_project_id
  project_slug      = var.ory_project_slug
}

variable "ory_workspace_api_key" {
  type        = string
  sensitive   = true
  description = "Ory Workspace API Key (ory_wak_...)"
}

variable "ory_workspace_id" {
  type        = string
  description = "Ory Workspace ID (UUID)"
}

variable "ory_project_api_key" {
  type        = string
  sensitive   = true
  description = "Ory Project API Key (ory_pat_...)"
}

variable "ory_project_id" {
  type        = string
  description = "Ory Project ID (UUID)"
}

variable "ory_project_slug" {
  type        = string
  description = "Ory Project Slug (e.g., vibrant-moore-abc123)"
}
```

Then create a `terraform.tfvars` file (do not commit this file):

```hcl
ory_workspace_api_key = "ory_wak_..."
ory_workspace_id      = "..."
ory_project_api_key   = "ory_pat_..."
ory_project_id        = "..."
ory_project_slug      = "..."
```

Alternatively, variables can be passed with `-var` parameter when running terraform commands:

```bash
terraform plan -var 'ory_workspace_api_key=ory_wak_...' -var 'ory_workspace_id=...' -var 'ory_project_api_key=ory_pat_...' -var 'ory_project_id=...' -var 'ory_project_slug=...'
```

Or use `TF_VAR_` environment variables:

```bash
export TF_VAR_ory_workspace_api_key="ory_wak_..."
export TF_VAR_ory_workspace_id="..."
export TF_VAR_ory_project_api_key="ory_pat_..."
export TF_VAR_ory_project_id="..."
export TF_VAR_ory_project_slug="..."

terraform plan
```

## Which Credentials Do You Need?

| Resource | Required Credentials |
|----------|---------------------|
| `ory_project`, `ory_workspace` | `workspace_api_key`, `workspace_id` |
| `ory_organization` | `workspace_api_key`, `project_id` |
| `ory_project_config`, `ory_action`, `ory_social_provider`, `ory_saml_provider`, `ory_email_template` | `workspace_api_key`, `project_id` |
| `ory_identity_schema`, `ory_project_api_key` | `workspace_api_key`, `project_id` |
| `ory_identity`, `ory_oauth2_client`, `ory_relationship` | `project_api_key`, `project_slug` |
| `ory_json_web_key_set` | `project_api_key`, `project_slug` |

## Why project configuration requires a workspace API key

Project configuration and project-level resources are managed through the Ory Console API, which requires a **workspace API key** (`ory_wak_...`). A **project API key** (`ory_pat_...`) authenticates project *data* operations such as identities, OAuth2 clients, and relationships, but is not authorized to read or change project *configuration*. This is a property of the Ory Network API, not the provider: the Console endpoints reject a project API key with `403 Forbidden`.

As a result, `ory_project_config`, `ory_action`, `ory_email_template`, `ory_social_provider`, `ory_saml_provider`, `ory_identity_schema`, `ory_custom_domain`, `ory_event_stream`, `ory_organization`, and `ory_project_api_key` all require a workspace API key. For more detail, see [Manage Ory Network projects through the API](https://www.ory.com/docs/guides/manage-project-via-api).

## Limiting blast radius with `allowed_project_ids`

A workspace API key can read and modify every project in the workspace, including production. To reduce the risk of an accidental change to the wrong project, set `allowed_project_ids` to the project IDs this configuration is permitted to touch. When set, the provider refuses any project operation whose target project ID is not in the list, before any request is sent to Ory.

```hcl
provider "ory" {
  workspace_api_key   = var.ory_workspace_api_key
  project_id          = var.ory_project_id
  allowed_project_ids = [var.ory_project_id] # only this project may be changed
}
```

The list can also be supplied as a comma-separated `ORY_ALLOWED_PROJECT_IDS` environment variable:

```bash
export ORY_ALLOWED_PROJECT_IDS="3fa274fe-910d-4766-b1a4-0c0dd3b41429,7bc1e0c2-..."
```

When `allowed_project_ids` is unset, no restriction is applied.

## Rate limits and retries

The Ory API answers `429 Too Many Requests` when a caller exceeds the request budget for a route. Terraform runs resource operations in parallel, ten at a time by default, so a bulk apply of many `ory_oauth2_client` or `ory_identity` resources can reach that budget.

The provider retries a rejected request instead of failing the apply. It follows [Ory's guidance for 429 responses](https://www.ory.com/docs/guides/rate-limits-new#how-to-handle-429-responses):

1. Back off exponentially, capped at 30 seconds.
2. Wait longer when the `x-ratelimit-reset` response header reports a longer window.
3. Add random jitter to every wait, so parallel workers do not retry together.
4. Pause before the budget runs out, based on the `x-ratelimit-remaining` header.

No configuration is needed. A large apply works at the default parallelism, and `terraform apply -parallelism=1` is no longer a workaround.

A `429` still reaches Terraform when the retries run out or when `max_retries` is `0`. The error then names the operation and the number of retries it used.

Set `max_retries` to change how many times a rejected request is retried. The default is 6. Its waits of 1, 2, 4, 8, 16 and 30 seconds come to 61 seconds, so the retry outlasts a full 60-second rate-limit window. That total is a floor: the jitter and a longer window reported by the server both raise it. The maximum is 20. Set it to `0` to turn the retry off.

```hcl
provider "ory" {
  workspace_api_key = var.ory_workspace_api_key
  project_id        = var.ory_project_id
  max_retries       = 8 # for a very large apply
}
```

The value can also be supplied as an `ORY_MAX_RETRIES` environment variable:

```bash
export ORY_MAX_RETRIES=8
```

To see each wait, run Terraform with `TF_LOG=DEBUG` and look for `Ory API rate limit reached`.

## Import Requirements

When importing existing resources, ensure you have the appropriate credentials configured **before** running `terraform import`.

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `allowed_project_ids` (List of String) Optional safety guardrail. When set, the provider refuses any project-configuration operation (`ory_project_config`, `ory_action`, `ory_email_template`, `ory_social_provider`, `ory_saml_provider`, `ory_identity_schema`, `ory_custom_domain`, `ory_event_stream`, `ory_organization`, `ory_project_api_key`, and `ory_project` create/delete) that targets a project ID not in this list. This bounds the blast radius of a workspace API key so a mis-pointed `project_id` (such as production) cannot be read or changed. When unset, no restriction is applied. Can also be set via the `ORY_ALLOWED_PROJECT_IDS` environment variable as a comma-separated list.
- `console_api_url` (String) Override the console API URL (default: `https://api.console.ory.sh`). Mainly for testing.
- `max_retries` (Number) How many times a request that the Ory API rejects with `429 Too Many Requests` is retried before the error reaches Terraform (default: `6`, maximum: `20`). The provider backs off exponentially, capped at 30 seconds, waits longer when the `x-ratelimit-reset` header reports a longer window, and adds random jitter so parallel workers do not retry together. At the default the waits come to 61 seconds, which is a floor: the jitter and a longer reported window both raise it. The `429` still reaches Terraform once the retries run out, or when this is set to `0`. Raise it for a very large apply. Can also be set via the `ORY_MAX_RETRIES` environment variable.
- `project_api_key` (String, Sensitive) Ory Project API Key (`ory_pat_...`). Used for identity and OAuth2 operations. Can also be set via `ORY_PROJECT_API_KEY` environment variable.
- `project_api_url` (String) Override the project API URL template (default: `https://%s.projects.oryapis.com`).
- `project_id` (String) Ory Project ID. Can also be set via `ORY_PROJECT_ID` environment variable.
- `project_slug` (String) Ory Project Slug (e.g., `vibrant-moore-abc123`). Required for identity and OAuth2 operations. Can also be set via `ORY_PROJECT_SLUG` environment variable.
- `workspace_api_key` (String, Sensitive) Ory Workspace API Key (`ory_wak_...`). Used for organization and project management. Can also be set via `ORY_WORKSPACE_API_KEY` environment variable.
- `workspace_id` (String) Ory Workspace ID. Can also be set via `ORY_WORKSPACE_ID` environment variable.
