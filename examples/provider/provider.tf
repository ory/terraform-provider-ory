# =============================================================================
# OPTION 1: Environment Variables (Recommended for CI/CD)
# =============================================================================
# Set these environment variables:
#   export ORY_WORKSPACE_API_KEY="ory_wak_..."
#   export ORY_WORKSPACE_ID="..."
#   export ORY_PROJECT_API_KEY="ory_pat_..."
#   export ORY_PROJECT_ID="..."
#   export ORY_PROJECT_SLUG="..."
#
# Then use an empty provider block:
#
# terraform {
#   required_providers {
#     ory = {
#       source = "ory/orynetwork"
#     }
#   }
# }
#
# provider "ory" {}
#
# =============================================================================
# OPTION 2: Terraform Variables (Recommended for tfvars)
# =============================================================================
# Define variables and pass values via terraform.tfvars or -var flags

terraform {
  required_providers {
    ory = {
      source = "ory/orynetwork"
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

# -----------------------------------------------------------------------------
# Variables
# -----------------------------------------------------------------------------

variable "ory_workspace_api_key" {
  type        = string
  sensitive   = true
  description = "Ory Workspace API Key (ory_wak_...)"
  default     = null
}

variable "ory_workspace_id" {
  type        = string
  description = "Ory Workspace ID (UUID)"
  default     = null
}

variable "ory_project_api_key" {
  type        = string
  sensitive   = true
  description = "Ory Project API Key (ory_pat_...)"
  default     = null
}

variable "ory_project_id" {
  type        = string
  description = "Ory Project ID (UUID)"
  default     = null
}

variable "ory_project_slug" {
  type        = string
  description = "Ory Project Slug (e.g., vibrant-moore-abc123)"
  default     = null
}

# -----------------------------------------------------------------------------
# Example terraform.tfvars (DO NOT COMMIT - add to .gitignore)
# -----------------------------------------------------------------------------
# ory_workspace_api_key = "ory_wak_..."
# ory_workspace_id      = "..."
# ory_project_api_key   = "ory_pat_..."
# ory_project_id        = "..."
# ory_project_slug      = "..."
