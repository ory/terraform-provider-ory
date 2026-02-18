# Read the current workspace (from provider config)
data "ory_workspace" "current" {}

output "workspace_name" {
  value = data.ory_workspace.current.name
}

# Read a specific workspace by ID
data "ory_workspace" "other" {
  id = "other-workspace-uuid"
}
