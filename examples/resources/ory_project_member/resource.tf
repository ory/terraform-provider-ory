# Invite each person to the project from the provider configuration.
variable "project_team" {
  type = map(string)
  default = {
    "alice@example.com" = "developer"
    "bob@example.com"   = "viewer"
  }
}

resource "ory_project_member" "team" {
  for_each = var.project_team

  email = each.key
  role  = each.value
}

# Invite a person to another project.
resource "ory_project_member" "auditor" {
  project_id = "00000000-0000-0000-0000-000000000000"
  email      = "auditor@example.com"
  role       = "viewer"
}

output "pending_invites" {
  value = [for m in ory_project_member.team : m.email if m.status == "pending"]
}
