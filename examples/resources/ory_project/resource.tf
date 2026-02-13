# Create a production project in a specific region
resource "ory_project" "production" {
  name        = "My Application - Production"
  environment = "prod"
  home_region = "eu-central"
}

# Create a staging project
resource "ory_project" "staging" {
  name        = "My Application - Staging"
  environment = "stage"
  home_region = "us-west"
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
