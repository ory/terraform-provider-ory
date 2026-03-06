# Look up an identity schema by its API-assigned ID
data "ory_identity_schema" "customer" {
  id = "abc123def456..."
}

output "schema_content" {
  value = data.ory_identity_schema.customer.schema
}

# Or reference the ID from a resource
resource "ory_identity_schema" "employee" {
  schema_id = "employee"
  schema = jsonencode({
    "$id"     = "https://example.com/employee.schema.json"
    "$schema" = "http://json-schema.org/draft-07/schema#"
    title     = "Employee"
    type      = "object"
    properties = {
      traits = {
        type = "object"
        properties = {
          email = {
            type   = "string"
            format = "email"
            "ory.sh/kratos" = {
              credentials  = { password = { identifier = true } }
              verification = { via = "email" }
              recovery     = { via = "email" }
            }
          }
        }
        required = ["email"]
      }
    }
  })
}

data "ory_identity_schema" "employee" {
  id = ory_identity_schema.employee.id
}
