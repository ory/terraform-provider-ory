---
page_title: "ory_oauth2_client Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Network OAuth2 client.
---

# ory_oauth2_client (Resource)

Manages an Ory Network OAuth2 client.

OAuth2 clients are used for machine-to-machine authentication or user-facing OAuth2/OIDC flows.

~> **Important:** The `client_secret` is only returned when the client is first created. Store it securely immediately after creation. It cannot be retrieved later, including after `terraform import`.

## Example Usage

```terraform
# Machine-to-machine client (Client Credentials flow)
resource "ory_oauth2_client" "api_service" {
  client_name                = "API Service"
  grant_types                = ["client_credentials"]
  token_endpoint_auth_method = "client_secret_post"
  scope                      = "read write admin"
}

# Web application (Authorization Code flow) with OIDC logout and metadata
resource "ory_oauth2_client" "web_app" {
  client_name    = "Web Application"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  redirect_uris = [
    "https://app.example.com/callback",
    "https://app.example.com/auth/callback"
  ]
  post_logout_redirect_uris  = ["https://app.example.com/logout"]
  token_endpoint_auth_method = "client_secret_basic"
  scope                      = "openid profile email offline_access"

  # Client metadata URIs
  client_uri = "https://app.example.com"
  logo_uri   = "https://app.example.com/logo.png"
  policy_uri = "https://app.example.com/privacy"
  tos_uri    = "https://app.example.com/terms"

  # OIDC logout
  frontchannel_logout_uri = "https://app.example.com/logout/frontchannel"
  backchannel_logout_uri  = "https://app.example.com/logout/backchannel"

  # Per-client CORS
  allowed_cors_origins = [
    "https://app.example.com",
    "https://admin.example.com"
  ]
}

# Single Page Application (Public client with PKCE)
resource "ory_oauth2_client" "spa" {
  client_name    = "Single Page App"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  redirect_uris = [
    "https://spa.example.com/callback",
    "http://localhost:3000/callback"
  ]
  token_endpoint_auth_method = "none"
  scope                      = "openid profile email"
}

output "api_service_client_id" {
  value = ory_oauth2_client.api_service.client_id
}

output "api_service_client_secret" {
  value     = ory_oauth2_client.api_service.client_secret
  sensitive = true
}
```

## Grant Types

| Grant Type | Description | Use Case |
|------------|-------------|----------|
| `authorization_code` | Authorization Code flow | Web apps, SPAs with PKCE |
| `client_credentials` | Client Credentials flow | Machine-to-machine / API access |
| `refresh_token` | Refresh Token grant | Long-lived sessions (pair with another grant) |
| `implicit` | Implicit flow (legacy) | Legacy SPAs (not recommended) |
| `urn:ietf:params:oauth:grant-type:device_code` | Device Authorization | IoT devices, CLIs |

## Response Types

| Response Type | Description |
|---------------|-------------|
| `code` | Authorization code (used with `authorization_code` grant) |
| `token` | Access token (used with `implicit` grant) |
| `id_token` | ID token (OpenID Connect) |

## Token Endpoint Auth Methods

| Method | Description |
|--------|-------------|
| `client_secret_post` | Secret sent in POST body (default) |
| `client_secret_basic` | Secret sent via HTTP Basic auth header |
| `private_key_jwt` | Client authenticates with a signed JWT |
| `none` | Public client (no secret, used for SPAs) |

## Access Token Strategy

The `access_token_strategy` attribute controls the format of issued access tokens:

| Strategy | Description |
|----------|-------------|
| `opaque` | Short, random string tokens (default) |
| `jwt` | Self-contained JSON Web Tokens |

## Consent Behavior

| Attribute | Description |
|-----------|-------------|
| `skip_consent` | When `true`, the user is never asked to grant consent for this client. Useful for first-party clients. |
| `skip_logout_consent` | When `true`, the user is not asked to confirm logout for this client. |

## Subject Type

The `subject_type` attribute controls how the `sub` claim is generated in ID tokens:

| Type | Description |
|------|-------------|
| `public` | Same `sub` value across all clients (default) |
| `pairwise` | Unique `sub` value per client (privacy-preserving) |

## OIDC Logout

The provider supports both OIDC front-channel and back-channel logout:

- `frontchannel_logout_uri` — The client's URL that the OP will redirect the user-agent to after logout. The OP sends the logout request via the user's browser.
- `backchannel_logout_uri` — The client's URL that the OP will call directly (server-to-server) to notify the client about a logout event.

## Import

OAuth2 clients can be imported using their client ID:

```shell
terraform import ory_oauth2_client.api <client-id>
```

~> **Note:** When importing, the `client_secret` will **not** be available. The secret is only returned at creation time and cannot be retrieved from the API afterward.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `client_name` (String) Human-readable name for the client.

### Optional

- `access_token_strategy` (String) Access token strategy: jwt or opaque.
- `allowed_cors_origins` (List of String) List of allowed CORS origins for this client.
- `audience` (List of String) List of allowed audiences for tokens.
- `backchannel_logout_uri` (String) OpenID Connect back-channel logout URI.
- `client_uri` (String) URL of the client's homepage.
- `contacts` (List of String) List of contact email addresses for the client maintainers.
- `frontchannel_logout_uri` (String) OpenID Connect front-channel logout URI.
- `grant_types` (List of String) OAuth2 grant types: authorization_code, implicit, client_credentials, refresh_token.
- `logo_uri` (String) URL of the client's logo.
- `metadata` (String) Custom metadata as JSON string.
- `policy_uri` (String) URL of the client's privacy policy.
- `post_logout_redirect_uris` (List of String) List of allowed post-logout redirect URIs for OpenID Connect logout.
- `redirect_uris` (List of String) List of allowed redirect URIs for authorization code flow.
- `response_types` (List of String) OAuth2 response types: code, token, id_token.
- `scope` (String) Space-separated list of OAuth2 scopes. If not specified, the API will set a default scope.
- `skip_consent` (Boolean) Skip the consent screen for this client. When true, the user is never asked to grant consent.
- `skip_logout_consent` (Boolean) Skip the logout consent screen for this client. When true, the user is not asked to confirm logout.
- `subject_type` (String) OpenID Connect subject type: public (same sub for all clients) or pairwise (unique sub per client).
- `token_endpoint_auth_method` (String) Token endpoint authentication method: client_secret_post, client_secret_basic, private_key_jwt, none.
- `tos_uri` (String) URL of the client's terms of service.

### Read-Only

- `client_id` (String) The OAuth2 client ID.
- `client_secret` (String, Sensitive) The OAuth2 client secret. Only returned on creation.
- `id` (String) Internal Terraform ID (same as client_id).
