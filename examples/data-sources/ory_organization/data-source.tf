# Look up an organization by ID
data "ory_organization" "acme" {
  id = "organization-uuid"
}

output "org_label" {
  value = data.ory_organization.acme.label
}

output "org_domains" {
  value = data.ory_organization.acme.domains
}
