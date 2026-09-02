---
page_title: "ory_scim_client Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Network SCIM client that lets an external identity provider provision identities into an organization.
---

# ory_scim_client (Resource)

Manages an Ory Network SCIM client. A SCIM client is the credential and the mapping an external identity provider such as Okta or Microsoft Entra uses to provision identities into one organization.

SCIM clients are part of the project configuration and have no Console API endpoint of their own. The provider writes them through the normalized project revision and reads them back from it.

-> **Plan:** SCIM provisioning needs an Ory Network plan with B2B features and a project in the `prod` or `stage` environment, the same requirements as `ory_organization`.

## How SCIM provisioning works

1. You create an organization and a SCIM client that points at it.
2. You give the identity provider the SCIM base URL, `https://<project-slug>.projects.oryapis.com/scim/<client_id>/v2`, and the `authorization_header_secret`.
3. The identity provider sends SCIM requests with the header `Authorization: Bearer <authorization_header_secret>`.
4. Ory applies the `mapper_url` Jsonnet to each SCIM user and creates or updates the identity inside the organization.

The URL path takes the string `client_id`, not an internal UUID. A request with the wrong secret gets HTTP 401. A request for a disabled client, or for a `client_id` that does not exist, gets HTTP 404.

## Example Usage

```terraform
# The organization the identity provider provisions users into. SCIM clients
# need a B2B plan and a project in the prod or stage environment.
resource "ory_organization" "acme" {
  label   = "Acme Corporation"
  domains = ["acme.example.com"]
}

# A SCIM client for Okta. The identity provider calls
# https://<project-slug>.projects.oryapis.com/scim/okta-acme/v2 with
# "Authorization: Bearer <authorization_header_secret>".
resource "ory_scim_client" "okta" {
  organization_id             = ory_organization.acme.id
  client_id                   = "okta-acme"
  label                       = "Okta SCIM"
  mapper_url                  = "base64://${base64encode(file("${path.module}/scim-mapper.jsonnet"))}"
  authorization_header_secret = var.okta_scim_secret
}

# A SCIM client whose secret never enters Terraform state. Change the version
# whenever the secret in Vault rotates.
ephemeral "vault_kv_secret_v2" "entra_scim" {
  mount = "secret"
  name  = "ory/entra-scim"
}

resource "ory_scim_client" "entra" {
  organization_id                        = ory_organization.acme.id
  client_id                              = "entra-acme"
  label                                  = "Microsoft Entra SCIM"
  mapper_url                             = "base64://${base64encode(file("${path.module}/scim-mapper.jsonnet"))}"
  authorization_header_secret_wo         = ephemeral.vault_kv_secret_v2.entra_scim.data["secret"]
  authorization_header_secret_wo_version = "1"
}

# A disabled client keeps its configuration but answers HTTP 404 until it is
# enabled again.
resource "ory_scim_client" "staging" {
  organization_id             = ory_organization.acme.id
  client_id                   = "okta-acme-staging"
  label                       = "Okta SCIM (staging tenant)"
  mapper_url                  = "base64://${base64encode(file("${path.module}/scim-mapper.jsonnet"))}"
  authorization_header_secret = var.okta_staging_scim_secret
  state                       = "disabled"
}

variable "okta_scim_secret" {
  description = "Bearer token Okta sends with every SCIM request"
  type        = string
  sensitive   = true
}

variable "okta_staging_scim_secret" {
  description = "Bearer token the Okta staging tenant sends with every SCIM request"
  type        = string
  sensitive   = true
}
```

## Authorization Secret

The API never returns `authorization_header_secret`. Every read reports it as an empty string. The provider therefore keeps the configured value in state and does not detect a change made in the Console. Changing the value in configuration rotates the secret. The old secret stops working as soon as the apply finishes.

For a value that never enters Terraform state, use the write-only argument instead. Write-only arguments need Terraform 1.11 or later.

| Write-only attribute | Replaces | Version trigger |
|----------------------|----------|-----------------|
| `authorization_header_secret_wo` | `authorization_header_secret` | `authorization_header_secret_wo_version` |

The two attributes are mutually exclusive, and exactly one of them must be set. Terraform cannot diff a write-only value, so change `authorization_header_secret_wo_version` whenever the secret rotates. That change makes Terraform send the new value to Ory.

## Mapper URL

`mapper_url` is the Jsonnet snippet that maps the SCIM user resource to identity traits. It accepts:

- A base64-encoded Jsonnet snippet prefixed with `base64://`.
- An `http://` or `https://` URL that Ory downloads at write time.

```jsonnet
local claims = std.extVar('claims');
{
  identity: {
    traits: {
      email: claims.email,
    },
  },
}
```

~> **Note:** Ory stores the mapper at a content-addressed object-storage URL and reports that URL on every read, for example `https://storage.googleapis.com/.../<hash>.jsonnet`. The provider keeps the value you configured in state and reads the stored URL only on import, so `mapper_url` never shows a spurious diff.

## Organizations

`organization_id` must reference an organization in the same project. The API reports a missing organization as an HTTP 500 foreign key violation, and the provider adds the attribute name to that error.

Deleting an organization deletes its SCIM clients server-side. When that happens outside Terraform, the next refresh removes both resources from state and the next apply recreates them. Reference the organization by resource attribute, `organization_id = ory_organization.acme.id`, so Terraform orders the create and the destroy correctly.

## Important Behaviors

- **`client_id` cannot be changed** after creation. Changing it forces a new resource.
- **`state` defaults to `enabled`.** A disabled client keeps its configuration and answers HTTP 404 until it is enabled again.
- **An existing `client_id` is taken over.** When a client with the configured `client_id` already exists in the project, for example one created in the Console, the first apply rewrites it with the configured values and rotates its secret. This matches `ory_social_provider` and `ory_saml_provider`.
- **Concurrent creates are safe.** Every write is a compare-and-swap on the project revision. The provider serializes the SCIM clients of one apply and rebuilds the write after a conflict with another writer.

## Import

Import through the project ID and the `client_id`:

```shell
terraform import ory_scim_client.okta <project-id>/okta-acme
```

The `client_id` alone also works when the provider block sets `project_id`:

```shell
terraform import ory_scim_client.okta okta-acme
```

After the import, `mapper_url` holds the stored object-storage URL and `authorization_header_secret` is empty. Set both in configuration. The first apply then uploads your mapper and rotates the secret to the configured value. Both changes are updates in place.

Keep `state` in the configuration when the imported client is disabled. The attribute defaults to `enabled`, so a configuration without `state = "disabled"` enables the client on the first apply.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `client_id` (String) Unique identifier of the SCIM client within the project. It is the path segment of the SCIM base URL, https://<project-slug>.projects.oryapis.com/scim/<client_id>/v2, and must match ^[a-z0-9_-]+$. Changing it forces a new resource.
- `label` (String) Human-readable name of the SCIM client, shown in the Ory Console.
- `mapper_url` (String) Jsonnet mapper that maps the SCIM user payload to identity traits. Accepts a base64-encoded Jsonnet snippet prefixed with base64://, or an http or https URL that Ory downloads at write time. Ory stores the payload at a content-addressed object-storage URL and reports that URL on read. The provider keeps the configured value in state and reads the stored URL only on import, so the value never shows a spurious diff.
- `organization_id` (String) ID of the organization the SCIM client provisions identities into. It must be the UUID of an organization in the same project. Deleting the organization deletes the SCIM client server-side, and the next plan then recreates both.

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `authorization_header_secret` (String, Sensitive) Bearer token the identity provider sends in the Authorization header of every SCIM request. Stored in Terraform state. The API never returns it, so a change made outside Terraform is not detected. Changing the value rotates the secret. Exactly one of authorization_header_secret and authorization_header_secret_wo must be set.
- `authorization_header_secret_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only equivalent of authorization_header_secret for Terraform 1.11 and later: the value is sent to Ory but never stored in Terraform state or plan. Use it to source the secret from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own. Change authorization_header_secret_wo_version to rotate it. Mutually exclusive with authorization_header_secret.
- `authorization_header_secret_wo_version` (String) Version trigger for authorization_header_secret_wo. Change this value whenever the write-only secret changes so Terraform sends the new value to Ory. Has no effect unless authorization_header_secret_wo is set.
- `project_id` (String) Project ID. If not set, uses the provider's project_id.
- `state` (String) State of the SCIM client, enabled or disabled. Only an enabled client serves SCIM requests. A disabled client answers HTTP 404 on its SCIM endpoint. Defaults to enabled.

### Read-Only

- `id` (String) Resource ID. The same value as client_id.
