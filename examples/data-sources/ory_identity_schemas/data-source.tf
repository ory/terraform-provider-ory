# List all identity schemas
data "ory_identity_schemas" "all" {}

output "schemas" {
  value = data.ory_identity_schemas.all.schemas
}

# List all workspace schemas during project bootstrap.
# When only a workspace API key is configured (no project_slug/project_api_key),
# set project_id to discover all workspace-scoped schemas for a new project.
resource "ory_project" "new" {
  name        = "my-new-project"
  environment = "dev"
}

data "ory_identity_schemas" "for_new_project" {
  project_id = ory_project.new.id
}
