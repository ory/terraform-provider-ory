# Read the current project (from provider config)
data "ory_project" "current" {}

output "project_name" {
  value = data.ory_project.current.name
}

output "project_slug" {
  value = data.ory_project.current.slug
}

output "project_state" {
  value = data.ory_project.current.state
}

output "project_environment" {
  value = data.ory_project.current.environment
}

output "project_home_region" {
  value = data.ory_project.current.home_region
}

# Read a specific project by ID
data "ory_project" "other" {
  id = "other-project-uuid"
}
