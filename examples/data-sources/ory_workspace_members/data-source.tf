# List the members of the workspace from the provider configuration.
data "ory_workspace_members" "current" {}

output "workspace_owners" {
  value = [for m in data.ory_workspace_members.current.members : m.email if m.role == "owner"]
}
