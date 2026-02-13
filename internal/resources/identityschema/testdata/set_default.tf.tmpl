resource "ory_identity_schema" "test" {
  schema_id   = "[[ .SchemaID ]]"
  set_default = true
  schema      = jsonencode({
    "$id": "[[ .AppURL ]]/[[ .SchemaID ]].json",
    "$schema": "http://json-schema.org/draft-07/schema#",
    "title": "Test Schema [[ .SchemaID ]]",
    "type": "object",
    "properties": {
      "traits": {
        "type": "object",
        "properties": {
          "email": {
            "type": "string",
            "format": "email",
            "title": "Email",
            "ory.sh/kratos": {
              "credentials": {
                "password": {"identifier": true}
              },
              "verification": {"via": "email"},
              "recovery": {"via": "email"}
            }
          }
        },
        "required": ["email"]
      }
    }
  })
}
