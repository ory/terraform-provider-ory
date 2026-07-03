# Identity using the preset email schema
resource "ory_identity" "basic_user" {
  schema_id = "preset://email"
  traits = jsonencode({
    email = "user@example.com"
  })
}

# Identity with password
resource "ory_identity" "user_with_password" {
  schema_id = "preset://email"
  traits = jsonencode({
    email = "secure-user@example.com"
  })
  password = var.user_password
  state    = "active"
}

# Identity with a write-only (ephemeral) password sourced from Vault.
# password_wo is never stored in Terraform state or plan (Terraform 1.11+).
# Bump password_wo_version whenever the password rotates so Terraform re-sends it.
ephemeral "vault_kv_secret_v2" "user_password" {
  mount = "secret"
  name  = "ory/user-password"
}

resource "ory_identity" "user_with_write_only_password" {
  schema_id = "preset://email"
  traits = jsonencode({
    email = "ephemeral-user@example.com"
  })
  password_wo         = ephemeral.vault_kv_secret_v2.user_password.data["password"]
  password_wo_version = "1"
  state               = "active"
}

# Identity with custom schema and metadata
resource "ory_identity" "customer" {
  schema_id = ory_identity_schema.customer.schema_id
  traits = jsonencode({
    email = "customer@example.com"
    name = {
      first = "John"
      last  = "Doe"
    }
  })
  metadata_public = jsonencode({
    tier = "premium"
  })
  metadata_admin = jsonencode({
    internal_id = "cust-12345"
    sales_rep   = "jane@company.com"
  })
}

variable "user_password" {
  type      = string
  sensitive = true
}
