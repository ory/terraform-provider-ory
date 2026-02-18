# Look up an identity by ID
data "ory_identity" "user" {
  id = "identity-uuid"
}

output "identity_state" {
  value = data.ory_identity.user.state
}

output "identity_traits" {
  value = data.ory_identity.user.traits
}
