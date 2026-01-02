# Configure the Ory provider
terraform {
  required_providers {
    ory = {
      source = "ory/orynetwork"
    }
  }
}

# Basic configuration using environment variables
provider "ory" {
  # Workspace API key for project/organization management
  workspace_api_key = var.ory_workspace_api_key # or set ORY_WORKSPACE_API_KEY env var

  # Project API key for identity/OAuth2 operations
  project_api_key = var.ory_project_api_key # or set ORY_PROJECT_API_KEY env var

  # Project identifiers
  project_id   = var.ory_project_id   # or set ORY_PROJECT_ID env var
  project_slug = var.ory_project_slug # or set ORY_PROJECT_SLUG env var
}

# Configuration with custom API URLs (for staging/enterprise environments)
# provider "ory" {
#   workspace_api_key = var.ory_workspace_api_key
#   workspace_id      = var.ory_workspace_id
#
#   # Custom API URLs (defaults shown)
#   console_api_url = "https://api.console.ory.sh"           # or set ORY_CONSOLE_API_URL env var
#   project_api_url = "https://%s.projects.oryapis.com"      # or set ORY_PROJECT_API_URL env var
# }

variable "ory_workspace_api_key" {
  type        = string
  sensitive   = true
  description = "Ory Workspace API Key (ory_wak_...)"
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
