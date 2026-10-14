# The organization the identity provider provisions users into. SCIM clients
# need a B2B plan and a project in the prod or stage environment.
resource "ory_organization" "acme" {
  label   = "Acme Corporation"
  domains = ["acme.example.com"]
}

# A SCIM client for Okta. The identity provider calls
# https://<project-slug>.projects.oryapis.com/scim/okta-acme/v2 with
# "Authorization: Bearer <authorization_header_secret>".
resource "ory_scim_client" "okta" {
  organization_id             = ory_organization.acme.id
  client_id                   = "okta-acme"
  label                       = "Okta SCIM"
  mapper_url                  = "base64://${base64encode(file("${path.module}/scim-mapper.jsonnet"))}"
  authorization_header_secret = var.okta_scim_secret
}

# A SCIM client whose secret never enters Terraform state. Change the version
# whenever the secret in Vault rotates.
ephemeral "vault_kv_secret_v2" "entra_scim" {
  mount = "secret"
  name  = "ory/entra-scim"
}

resource "ory_scim_client" "entra" {
  organization_id                        = ory_organization.acme.id
  client_id                              = "entra-acme"
  label                                  = "Microsoft Entra SCIM"
  mapper_url                             = "base64://${base64encode(file("${path.module}/scim-mapper.jsonnet"))}"
  authorization_header_secret_wo         = ephemeral.vault_kv_secret_v2.entra_scim.data["secret"]
  authorization_header_secret_wo_version = "1"
}

# A disabled client keeps its configuration but answers HTTP 404 until it is
# enabled again.
resource "ory_scim_client" "staging" {
  organization_id             = ory_organization.acme.id
  client_id                   = "okta-acme-staging"
  label                       = "Okta SCIM (staging tenant)"
  mapper_url                  = "base64://${base64encode(file("${path.module}/scim-mapper.jsonnet"))}"
  authorization_header_secret = var.okta_staging_scim_secret
  state                       = "disabled"
}

variable "okta_scim_secret" {
  description = "Bearer token Okta sends with every SCIM request"
  type        = string
  sensitive   = true
}

variable "okta_staging_scim_secret" {
  description = "Bearer token the Okta staging tenant sends with every SCIM request"
  type        = string
  sensitive   = true
}
