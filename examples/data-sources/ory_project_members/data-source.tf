# List the members of the project from the provider configuration.
data "ory_project_members" "current" {}

# People with a role on the project itself, not through the workspace.
output "direct_project_members" {
  value = [for m in data.ory_project_members.current.members : m.email if m.source == "project"]
}

# List the members of another project.
data "ory_project_members" "other" {
  project_id = "00000000-0000-0000-0000-000000000000"
}
