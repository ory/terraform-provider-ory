# Invite a person to the workspace from the provider configuration.
resource "ory_workspace_member" "carol" {
  email = "carol@example.com"
  role  = "developer"
}

# Invite a workspace owner.
resource "ory_workspace_member" "dave" {
  email = "dave@example.com"
  role  = "owner"
}
