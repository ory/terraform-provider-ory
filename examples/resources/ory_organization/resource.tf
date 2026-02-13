# Create an organization for multi-tenancy (B2B SaaS)
resource "ory_organization" "acme" {
  label   = "Acme Corporation"
  domains = ["acme.com", "acme.io"]
}

# Multiple tenant organizations
resource "ory_organization" "globex" {
  label   = "Globex Corporation"
  domains = ["globex.com"]
}

# Dynamic organizations from a variable map
variable "tenant_orgs" {
  type = map(object({
    label   = string
    domains = list(string)
  }))
  default = {}
}

resource "ory_organization" "tenants" {
  for_each = var.tenant_orgs
  label    = each.value.label
  domains  = each.value.domains
}

output "organization_id" {
  value = ory_organization.acme.id
}
