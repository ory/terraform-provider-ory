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
  audience                   = ["https://api.example.com"]
  access_token_strategy      = "jwt"
  contacts                   = ["api-team@example.com"]

  # Custom token lifespans for this client
  client_credentials_grant_access_token_lifespan = "30m"
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

  # First-party app: skip consent and logout consent screens
  skip_consent        = true
  skip_logout_consent = true
  subject_type        = "pairwise"
  contacts            = ["web-team@example.com"]

  # Client metadata URIs
  client_uri = "https://app.example.com"
  logo_uri   = "https://app.example.com/logo.png"
  policy_uri = "https://app.example.com/privacy"
  tos_uri    = "https://app.example.com/terms"

  # OIDC logout with session notifications
  frontchannel_logout_uri              = "https://app.example.com/logout/frontchannel"
  frontchannel_logout_session_required = true
  backchannel_logout_uri               = "https://app.example.com/logout/backchannel"
  backchannel_logout_session_required  = true

  # Per-client CORS
  allowed_cors_origins = [
    "https://app.example.com",
    "https://admin.example.com"
  ]

  # Per-grant token lifespans
  authorization_code_grant_access_token_lifespan  = "1h"
  authorization_code_grant_id_token_lifespan      = "1h"
  authorization_code_grant_refresh_token_lifespan = "720h"
  refresh_token_grant_access_token_lifespan       = "1h"
  refresh_token_grant_id_token_lifespan           = "1h"
  refresh_token_grant_refresh_token_lifespan      = "720h"

  # Custom metadata
  metadata = jsonencode({
    department = "engineering"
    tier       = "internal"
  })
}

# Client with custom token lifespans
resource "ory_oauth2_client" "api_gateway" {
  client_name = "API Gateway"
  grant_types = ["client_credentials"]
  scope       = "api:read api:write"

  # Short-lived access tokens for M2M
  client_credentials_grant_access_token_lifespan = "15m"

  # Logout session tracking
  backchannel_logout_uri              = "https://gateway.example.com/logout"
  backchannel_logout_session_required = true
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

# Device Authorization flow (CLI tools, IoT devices)
resource "ory_oauth2_client" "cli_tool" {
  client_name                = "CLI Tool"
  grant_types                = ["urn:ietf:params:oauth:grant-type:device_code", "refresh_token"]
  response_types             = ["code"]
  token_endpoint_auth_method = "none"
  scope                      = "openid offline_access"
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

## Per-Grant Token Lifespans

Override default token lifespans on a per-client, per-grant basis using Go duration strings (e.g., `1h`, `30m`, `720h`):

| Grant Type | Access Token | ID Token | Refresh Token |
|------------|-------------|----------|---------------|
| Authorization Code | `authorization_code_grant_access_token_lifespan` | `authorization_code_grant_id_token_lifespan` | `authorization_code_grant_refresh_token_lifespan` |
| Client Credentials | `client_credentials_grant_access_token_lifespan` | — | — |
| Device Authorization | `device_authorization_grant_access_token_lifespan` | `device_authorization_grant_id_token_lifespan` | `device_authorization_grant_refresh_token_lifespan` |
| Implicit | `implicit_grant_access_token_lifespan` | `implicit_grant_id_token_lifespan` | — |
| JWT Bearer | `jwt_bearer_grant_access_token_lifespan` | — | — |
| Refresh Token | `refresh_token_grant_access_token_lifespan` | `refresh_token_grant_id_token_lifespan` | `refresh_token_grant_refresh_token_lifespan` |

If not set, the project-level defaults apply.

## OIDC Configuration

| Attribute | Description |
|-----------|-------------|
| `jwks_uri` | URL of the client's JSON Web Key Set, used with `private_key_jwt` authentication |
| `userinfo_signed_response_alg` | JWS algorithm for signing UserInfo responses (e.g., `RS256`) |
| `request_object_signing_alg` | JWS algorithm for signing request objects (e.g., `RS256`) |

## OIDC Logout

The provider supports both OIDC front-channel and back-channel logout:

- `frontchannel_logout_uri` — The client's URL that the OP will redirect the user-agent to after logout. The OP sends the logout request via the user's browser.
- `backchannel_logout_uri` — The client's URL that the OP will call directly (server-to-server) to notify the client about a logout event.
- `frontchannel_logout_session_required` — Whether the client requires a session identifier (`sid`) in front-channel logout notifications.
- `backchannel_logout_session_required` — Whether the client requires a session identifier (`sid`) in back-channel logout notifications.

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
- `authorization_code_grant_access_token_lifespan` (String) Access token lifespan for authorization code grant (e.g., '1h', '30m').
- `authorization_code_grant_id_token_lifespan` (String) ID token lifespan for authorization code grant (e.g., '1h', '30m').
- `authorization_code_grant_refresh_token_lifespan` (String) Refresh token lifespan for authorization code grant (e.g., '720h').
- `backchannel_logout_session_required` (Boolean) Whether the client requires a session identifier in back-channel logout notifications.
- `backchannel_logout_uri` (String) OpenID Connect back-channel logout URI.
- `client_credentials_grant_access_token_lifespan` (String) Access token lifespan for client credentials grant (e.g., '1h', '30m').
- `client_uri` (String) URL of the client's homepage.
- `contacts` (List of String) List of contact email addresses for the client maintainers.
- `device_authorization_grant_access_token_lifespan` (String) Access token lifespan for device authorization grant (e.g., '1h').
- `device_authorization_grant_id_token_lifespan` (String) ID token lifespan for device authorization grant (e.g., '1h').
- `device_authorization_grant_refresh_token_lifespan` (String) Refresh token lifespan for device authorization grant (e.g., '720h').
- `frontchannel_logout_session_required` (Boolean) Whether the client requires a session identifier in front-channel logout notifications.
- `frontchannel_logout_uri` (String) OpenID Connect front-channel logout URI.
- `grant_types` (List of String) OAuth2 grant types: authorization_code, implicit, client_credentials, refresh_token.
- `implicit_grant_access_token_lifespan` (String) Access token lifespan for implicit grant (e.g., '1h', '30m').
- `implicit_grant_id_token_lifespan` (String) ID token lifespan for implicit grant (e.g., '1h', '30m').
- `jwks_uri` (String) URL of the client's JSON Web Key Set for private_key_jwt authentication.
- `jwt_bearer_grant_access_token_lifespan` (String) Access token lifespan for JWT bearer grant (e.g., '1h', '30m').
- `logo_uri` (String) URL of the client's logo.
- `metadata` (String) Custom metadata as JSON string.
- `policy_uri` (String) URL of the client's privacy policy.
- `post_logout_redirect_uris` (List of String) List of allowed post-logout redirect URIs for OpenID Connect logout.
- `redirect_uris` (List of String) List of allowed redirect URIs for authorization code flow.
- `refresh_token_grant_access_token_lifespan` (String) Access token lifespan for refresh token grant (e.g., '1h', '30m').
- `refresh_token_grant_id_token_lifespan` (String) ID token lifespan for refresh token grant (e.g., '1h', '30m').
- `refresh_token_grant_refresh_token_lifespan` (String) Refresh token lifespan for refresh token grant (e.g., '720h').
- `request_object_signing_alg` (String) JWS algorithm for signing request objects (e.g., 'RS256', 'ES256').
- `response_types` (List of String) OAuth2 response types: code, token, id_token.
- `scope` (String) Space-separated list of OAuth2 scopes. If not specified, the API will set a default scope.
- `skip_consent` (Boolean) Skip the consent screen for this client. When true, the user is never asked to grant consent.
- `skip_logout_consent` (Boolean) Skip the logout consent screen for this client. When true, the user is not asked to confirm logout.
- `subject_type` (String) OpenID Connect subject type: public (same sub for all clients) or pairwise (unique sub per client).
- `token_endpoint_auth_method` (String) Token endpoint authentication method: client_secret_post, client_secret_basic, private_key_jwt, none.
- `tos_uri` (String) URL of the client's terms of service.
- `userinfo_signed_response_alg` (String) JWS algorithm for signing UserInfo responses (e.g., 'RS256', 'ES256').

### Read-Only

- `client_id` (String) The OAuth2 client ID.
- `client_secret` (String, Sensitive) The OAuth2 client secret. Only returned on creation.
- `id` (String) Internal Terraform ID (same as client_id).
