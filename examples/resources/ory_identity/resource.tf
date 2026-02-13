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
