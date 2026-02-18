# List all identity schemas
data "ory_identity_schemas" "all" {}

output "schemas" {
  value = data.ory_identity_schemas.all.schemas
}
