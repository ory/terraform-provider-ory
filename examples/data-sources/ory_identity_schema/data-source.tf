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

# Look up a schema during project bootstrap (no project_slug/project_api_key needed)
data "ory_identity_schema" "bootstrap" {
  id         = "preset://username"
  project_id = "your-project-uuid"
}

# Bootstrap pattern: create a new project and set a custom schema as default.
# Provide the schema content directly — the API deduplicates automatically
# if the same content already exists in the workspace.
resource "ory_project" "new" {
  name = "my-new-project"
}

resource "ory_identity_schema" "default" {
  schema_id   = "customer"
  project_id  = ory_project.new.id
  set_default = true
  schema = jsonencode({
    "$id"     = "https://example.com/customer.schema.json"
    "$schema" = "http://json-schema.org/draft-07/schema#"
    title     = "Customer"
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

# After the schema is added, you can reference it via data source
data "ory_identity_schema" "default" {
  id         = ory_identity_schema.default.id
  project_id = ory_project.new.id
}
