resource "ory_identity" "test" {
  schema_id = "preset://username"

  traits = jsonencode({
    username = "[[ .Username ]]"
  })

  state = "active"

  metadata_public = jsonencode({
    role        = "admin"
    created_by  = "terraform"
  })
}
