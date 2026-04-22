# SAML identity provider (Ory Polis)
resource "ory_saml_provider" "corporate" {
  provider_id          = "corporate"
  label                = "Sign in with Corporate SSO"
  raw_idp_metadata_xml = "base64://${base64encode(file("${path.module}/corporate-idp-metadata.xml"))}"
}

# SAML provider with a hosted metadata URL
resource "ory_saml_provider" "hosted" {
  provider_id          = "okta"
  label                = "Okta SSO"
  raw_idp_metadata_xml = "https://example.okta.com/app/metadata"
}

# SAML provider scoped to an organization (B2B)
resource "ory_saml_provider" "acme" {
  provider_id          = "acme"
  label                = "Acme SSO"
  raw_idp_metadata_xml = "base64://${base64encode(var.acme_idp_metadata)}"
  organization_id      = ory_organization.acme.id
}

# SAML provider with a custom mapper and audience override
resource "ory_saml_provider" "custom" {
  provider_id                  = "custom"
  label                        = "Custom Workplace"
  raw_idp_metadata_xml         = "base64://${base64encode(var.custom_idp_metadata)}"
  mapper_url                   = "base64://${base64encode(file("${path.module}/saml-mapper.jsonnet"))}"
  audience_override_base_url   = "https://iam.example.com"
  proxy_saml_audience_override = "https://sp.example.com/saml"
}

resource "ory_organization" "acme" {
  label   = "Acme"
  domains = ["acme.example.com"]
}

variable "acme_idp_metadata" {
  description = "SAML IDP metadata XML for the Acme organization"
  type        = string
  sensitive   = false
}

variable "custom_idp_metadata" {
  description = "SAML IDP metadata XML for the Custom workplace"
  type        = string
  sensitive   = false
}
