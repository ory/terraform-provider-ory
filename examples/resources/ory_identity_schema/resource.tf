# Customer identity schema with email and name
resource "ory_identity_schema" "customer" {
  schema_id   = "customer_v1"
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
            title  = "Email Address"
            "ory.sh/kratos" = {
              credentials = {
                password = { identifier = true }
                code     = { identifier = true, via = "email" }
              }
              verification = { via = "email" }
              recovery     = { via = "email" }
            }
          }
          name = {
            type = "object"
            properties = {
              first = {
                type  = "string"
                title = "First Name"
              }
              last = {
                type  = "string"
                title = "Last Name"
              }
            }
          }
          phone = {
            type  = "string"
            title = "Phone Number"
            "ory.sh/kratos" = {
              credentials = {
                code = { identifier = true, via = "sms" }
              }
            }
          }
        }
        required             = ["email"]
        additionalProperties = false
      }
    }
  })
}

output "schema_id" {
  value = ory_identity_schema.customer.id
}
