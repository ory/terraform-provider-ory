# Create a production project in Europe (default region)
resource "ory_project" "production_eu" {
  name        = "My Application - Production EU"
  environment = "prod"
  home_region = "eu-central"
}

# Create a production project in US East
resource "ory_project" "production_us_east" {
  name        = "My Application - Production US East"
  environment = "prod"
  home_region = "us-east"
}

# Create a production project in US West
resource "ory_project" "production_us_west" {
  name        = "My Application - Production US West"
  environment = "prod"
  home_region = "us-west"
}

# Create a production project in Asia Pacific (Tokyo)
resource "ory_project" "production_asia" {
  name        = "My Application - Production Asia"
  environment = "prod"
  home_region = "asia-northeast"
}

# Create a global multi-region project
resource "ory_project" "production_global" {
  name        = "My Application - Production Global"
  environment = "prod"
  home_region = "global"
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
  value = ory_project.production_eu.id
}

output "production_project_slug" {
  description = "Use this for ORY_PROJECT_SLUG"
  value       = ory_project.production_eu.slug
}
