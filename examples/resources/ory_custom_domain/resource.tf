# Basic custom domain
resource "ory_custom_domain" "auth" {
  hostname      = "auth.example.com"
  cookie_domain = "example.com"
}

# Custom domain with CORS and custom UI
resource "ory_custom_domain" "full" {
  hostname      = "auth.example.com"
  cookie_domain = "example.com"

  cors_enabled         = true
  cors_allowed_origins = ["https://app.example.com", "https://admin.example.com"]
  custom_ui_base_url   = "https://app.example.com/auth"
}
