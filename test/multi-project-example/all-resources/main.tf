# =============================================================================
# Multi-Project for_each — Project API resources with resource-level creds
# =============================================================================
# Demonstrates that Project API resources (JWK, identity, OIDC dynamic client)
# can use for_each across multiple projects in a single apply, thanks to
# resource-level project_slug + project_api_key attributes.
#
# Note: ory_relationship is also supported but omitted here because it requires
# Ory Keto namespaces to be configured on the project first.
# =============================================================================

terraform {
  required_providers {
    ory = {
      source = "ory/ory"
    }
  }
}

provider "ory" {}

locals {
  projects = {
    "project-a" = { name = "tf-all-resources-a", environment = "dev" }
    "project-b" = { name = "tf-all-resources-b", environment = "dev" }
  }
}

resource "ory_project" "this" {
  for_each    = local.projects
  name        = each.value.name
  environment = each.value.environment
}

resource "ory_project_api_key" "this" {
  for_each   = ory_project.this
  project_id = each.value.id
  name       = "terraform-managed"
}

# JWK — for_each with resource-level credentials
resource "ory_json_web_key_set" "this" {
  for_each        = ory_project.this
  project_slug    = each.value.slug
  project_api_key = ory_project_api_key.this[each.key].value

  set_id    = "signing-keys"
  key_id    = "sig-key-1"
  algorithm = "RS256"
  use       = "sig"
}

# Identity — for_each with resource-level credentials
resource "ory_identity" "admin" {
  for_each        = ory_project.this
  project_slug    = each.value.slug
  project_api_key = ory_project_api_key.this[each.key].value

  schema_id = "preset://email"
  traits = jsonencode({
    email = "admin-${each.key}@example.com"
  })
}

# OIDC Dynamic Client — for_each with resource-level credentials
resource "ory_oidc_dynamic_client" "app" {
  for_each        = ory_project.this
  project_slug    = each.value.slug
  project_api_key = ory_project_api_key.this[each.key].value

  client_name    = "tf-test-${each.key}"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid offline_access"
  redirect_uris  = ["https://${each.key}.example.com/callback"]
}

output "projects" {
  value = { for k, v in ory_project.this : k => { id = v.id, slug = v.slug } }
}

output "jwk_set_ids" {
  value = { for k, v in ory_json_web_key_set.this : k => v.id }
}

output "identity_ids" {
  value = { for k, v in ory_identity.admin : k => v.id }
}

output "oidc_client_ids" {
  value = { for k, v in ory_oidc_dynamic_client.app : k => v.client_id }
}
