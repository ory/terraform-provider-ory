# API key for backend service
resource "ory_project_api_key" "backend" {
  name = "Backend Service"
}

# API key with expiration
resource "ory_project_api_key" "temporary" {
  name       = "Temporary Access"
  expires_at = "2026-12-31T23:59:59Z"
}

# Multiple keys for different environments
resource "ory_project_api_key" "development" {
  name = "Development"
}

resource "ory_project_api_key" "staging" {
  name = "Staging"
}

resource "ory_project_api_key" "production" {
  name = "Production"
}

output "backend_api_key" {
  value     = ory_project_api_key.backend.value
  sensitive = true
}
