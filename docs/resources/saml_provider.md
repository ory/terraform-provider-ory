---
page_title: "ory_saml_provider Resource - ory"
subcategory: ""
description: |-
  Manages an Ory Network SAML identity provider (Ory Polis).
---

# ory_saml_provider (Resource)

Manages an Ory Network SAML identity provider. Ory Network uses [Ory Polis](https://www.ory.sh/polis) as its SAML backend, which lets you connect enterprise SSO identity providers (Okta, Azure AD, OneLogin, JumpCloud, etc.) to your project.

SAML providers are configured as part of the project's SAML authentication method. Each provider is identified by a unique `provider_id` that is used in the ACS callback URL.

-> **Plan:** SAML single sign-on requires a plan with B2B or Enterprise features. Check your Ory Network plan before using this resource.

## How SAML sign-in works

When a user initiates a SAML login:

1. The user is redirected to your Ory project's SAML endpoint.
2. Ory (via Polis) redirects the user to the configured IDP (based on `provider_id`).
3. After successful authentication, the IDP returns a signed SAML assertion to Ory's ACS URL.
4. Ory validates the assertion against the certificate in `raw_idp_metadata_xml`, applies the `mapper_url` Jsonnet mapping, and creates or updates the identity.

The IDP metadata you configure here is what Ory uses to validate assertions — it must contain an `<IDPSSODescriptor>` with at least one signing `<KeyDescriptor>`.

## Example Usage

```terraform
# SAML identity provider (Ory Polis)
resource "ory_saml_provider" "corporate" {
  provider_id          = "corporate"
  label                = "Sign in with Corporate SSO"
  raw_idp_metadata_xml = "base64://${base64encode(file("${path.module}/corporate-idp-metadata.xml"))}"
}

# SAML provider with a hosted metadata URL
resource "ory_saml_provider" "hosted" {
  provider_id          = "okta"
  label                = "Okta SSO"
  raw_idp_metadata_xml = "https://example.okta.com/app/metadata"
}

# SAML provider scoped to an organization (B2B)
resource "ory_saml_provider" "acme" {
  provider_id          = "acme"
  label                = "Acme SSO"
  raw_idp_metadata_xml = "base64://${base64encode(var.acme_idp_metadata)}"
  organization_id      = ory_organization.acme.id
}

# SAML provider with a custom mapper and audience override
resource "ory_saml_provider" "custom" {
  provider_id                  = "custom"
  label                        = "Custom Workplace"
  raw_idp_metadata_xml         = "base64://${base64encode(var.custom_idp_metadata)}"
  mapper_url                   = "base64://${base64encode(file("${path.module}/saml-mapper.jsonnet"))}"
  audience_override_base_url   = "https://iam.example.com"
  proxy_saml_audience_override = "https://sp.example.com/saml"
}

resource "ory_organization" "acme" {
  label   = "Acme"
  domains = ["acme.example.com"]
}

variable "acme_idp_metadata" {
  description = "SAML IDP metadata XML for the Acme organization"
  type        = string
  sensitive   = false
}

variable "custom_idp_metadata" {
  description = "SAML IDP metadata XML for the Custom workplace"
  type        = string
  sensitive   = false
}
```

## IDP Metadata

The `raw_idp_metadata_xml` attribute accepts the IDP's SAML 2.0 metadata XML. Ory requires one of:

- **`base64://<base64-encoded XML>`** — for inline metadata (typical when the IDP exports a static XML file).
- **`https://...`** or **`http://...`** — for metadata hosted by the IDP (some providers expose a metadata URL).

The metadata must include at least one signing `<KeyDescriptor>` and one `<SingleSignOnService>` binding, or Ory will reject the configuration with a validation error.

~> **Note:** The API transforms inline metadata to a hosted GCS URL on read. The provider preserves the original `base64://` value in state when you configured it that way, so Terraform won't show spurious drift.

## Mapper URL

The `mapper_url` attribute controls how SAML attributes are mapped to Ory identity traits. It accepts:

- A URL pointing to a hosted Jsonnet file
- A base64-encoded Jsonnet template prefixed with `base64://`

If not set, the provider uses a default mapper that extracts the email claim.

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

## Organization Scoping (B2B)

Set `organization_id` to associate a SAML provider with a specific organization. This is how Ory routes a user's login based on their email domain to the correct IDP. When combined with an `ory_organization` resource that owns the user's domain, Ory automatically starts a SAML flow against this provider.

```hcl
resource "ory_organization" "acme" {
  label   = "Acme"
  domains = ["acme.example.com"]
}

resource "ory_saml_provider" "acme_sso" {
  provider_id          = "acme"
  label                = "Acme SSO"
  raw_idp_metadata_xml = "base64://${base64encode(var.acme_idp_metadata)}"
  organization_id      = ory_organization.acme.id
}
```

## Audience Overrides

- **`audience_override_base_url`** — Overrides the base URL used when computing the SAML SP audience (EntityID). Use when Ory is reached through a custom domain and you need the EntityID to match that domain instead of the default `*.projects.oryapis.com` hostname.
- **`proxy_saml_audience_override`** — Replaces the SP audience (EntityID) entirely. Use when the IDP trust was established against a pre-existing audience that you cannot change on the IDP side.

## Important Behaviors

- **`provider_id` cannot be changed** after creation. Changing it forces a new resource.
- **`raw_idp_metadata_xml` is required** and is validated by Ory on every update. An invalid cert or missing binding will reject the patch.
- **Deleting the last provider** resets the entire SAML configuration (`enabled = false`, empty providers). This mirrors the behavior of the `ory_social_provider` resource.

## Import

Import using the provider ID:

```shell
terraform import ory_saml_provider.corporate corporate
```

The `provider_id` is the unique identifier you chose when creating the provider. After import, the `raw_idp_metadata_xml` attribute will be populated with the hosted URL form (the API's transformed value). If you originally configured metadata inline with `base64://...`, update your configuration to use the new hosted URL, or re-apply with your original `base64://` value to re-upload the inline XML.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `provider_id` (String) Unique identifier for the SAML provider (used in ACS callback URLs).
- `raw_idp_metadata_xml` (String) SAML IDP metadata XML. Accepts a URL (http/https) or a base64-encoded XML prefixed with `base64://`. This is required by the SAML identity provider to establish trust.

### Optional

- `audience_override_base_url` (String) Override the base URL used when computing the SAML SP audience (EntityID). Useful when running Ory behind a custom domain.
- `label` (String) Human-readable label for the provider, displayed on the login button (e.g., "Sign in with Corporate SSO").
- `mapper_url` (String) Jsonnet mapper URL for mapping SAML attributes to identity traits. Accepts a URL (http/https) or a base64-encoded Jsonnet template prefixed with `base64://`. If not set, a default mapper that extracts email from claims will be used.
- `organization_id` (String) Organization ID to associate this SAML provider with (for B2B SSO).
- `project_id` (String) Project ID. If not set, uses provider's project_id.
- `proxy_saml_audience_override` (String) Customer-controlled override of the SAML Audience (EntityID) sent to the identity provider.

### Read-Only

- `id` (String) Resource ID (same as provider_id).
