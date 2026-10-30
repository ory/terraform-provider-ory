# Custom text for the English hosted login. A key that is not in the map keeps
# the built-in Ory Elements text.
resource "ory_account_experience_translation" "en" {
  locale = "en"
  messages = {
    "login.title"        = "Sign in to Example"
    "login.subtitle"     = "Sign in with {parts}"
    "recovery.title"     = "Forgot your password?"
    "registration.title" = "Create your Example account"
  }
}

# Custom text for the German hosted login, kept in a JSON file with the same
# flat format as the Ory Elements locale files. German is one of the locales
# that the hosted login serves when account_experience_enabled_locales is not
# set on ory_project_config.
resource "ory_account_experience_translation" "de" {
  locale   = "de"
  messages = jsondecode(file("${path.module}/translations/de.json"))
}
