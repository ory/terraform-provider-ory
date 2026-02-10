# Create a production project
resource "ory_project" "production" {
  name        = "My Application - Production"
  environment = "prod"
}

# Create a staging project
resource "ory_project" "staging" {
  name        = "My Application - Staging"
  environment = "stage"
}

# Create a development project (note: no B2B Organizations support)
resource "ory_project" "dev" {
  name        = "My Application - Development"
  environment = "dev"
}

# Output the project details
output "production_project_id" {
  value = ory_project.production.id
}

output "production_project_slug" {
  description = "Use this for ORY_PROJECT_SLUG"
  value       = ory_project.production.slug
}
