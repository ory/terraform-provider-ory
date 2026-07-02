---
name: import-existing-project
description: Generate Terraform configuration for an existing Ory Network project. Inventories the project via the Ory Console and Admin APIs, builds terraform import blocks with the correct per-resource import IDs, generates configuration with terraform plan -generate-config-out, and refines it until terraform plan converges to no changes. Use when adopting Terraform for an Ory Network project that was configured through the Console UI, CLI, or API, or when asked to reverse-engineer .tf files from a live project.
---

# Import an existing Ory Network project into Terraform

Onboard a live Ory Network project into Terraform so that `terraform plan`
reports no changes. Terraform's built-in `import` blocks and
`terraform plan -generate-config-out` do the mechanical part; this skill fills
in the Ory-specific knowledge: where each resource is discovered, how each
import ID is built, which attributes to prune from generated config, and which
secrets can never be read back.

**Safety first:** every step below is read-only against Ory until the final
`terraform apply`, and that apply only records imports (`0 to add, 0 to change,
0 to destroy`). Never run `terraform destroy` in the working directory — the
imported resources are real. To undo a mistaken import, use
`terraform state rm <address>`; it removes the resource from state without
touching the live project.

## Prerequisites

- Terraform >= 1.5 (`import` blocks and `-generate-config-out`).
- `curl` and `jq` (used by the inventory script).
- A **workspace API key** (`ory_wak_...`, create under Workspace Settings > API
  keys) and, to cover OAuth2 clients / JWKS / trusted issuers, a **project API
  key** (`ory_pat_...`).
- The project UUID. List a workspace's projects with the workspace API key:

  ```bash
  curl -sH "Authorization: Bearer $ORY_WORKSPACE_API_KEY" \
    "${ORY_CONSOLE_API_URL:-https://api.console.ory.sh}/workspaces/$ORY_WORKSPACE_ID/projects" \
    | jq -r '.projects[] | "\(.id)  \(.slug)  \(.name)"'
  ```

  Use this workspace-scoped endpoint, not `GET /projects` (that path needs a
  browser session token and returns 403 for an API key). The `ory list
  projects` CLI also works, but only against Ory's **production** console with
  an interactive `ory auth` session — it ignores `ORY_CONSOLE_API_URL`, so it
  cannot reach other environments, and it rejects `--workspace` when
  `ORY_WORKSPACE_API_KEY` is set. The curl call above is environment-agnostic.

Export the credentials as environment variables; the provider and the
inventory script both read them:

```bash
export ORY_WORKSPACE_API_KEY=ory_wak_...
export ORY_PROJECT_API_KEY=ory_pat_...     # optional but recommended
export ORY_PROJECT_ID=<project-uuid>
export ORY_PROJECT_SLUG=<project-slug>
```

Non-default environments (e.g. staging) also need endpoint overrides, both for
the script and the provider block: `ORY_CONSOLE_API_URL` (default
`https://api.console.ory.sh`) and `ORY_PROJECT_API_URL` (a printf template,
default `https://%s.projects.oryapis.com`).

## Workflow

### 1. Inventory the project and build import blocks

Run the bundled script from an empty working directory:

```bash
<path-to-skill>/scripts/generate-imports.sh > imports.tf
```

It dumps the project revision (`GET /projects/{id}`), walks every list
endpoint, and emits one `import` block per importable resource with the
correct ID format (see the reference table below). Data-plane objects
(identities, relationship tuples) and unrecoverable ones (project API key
values) come out as explanatory comments instead of import blocks.

Review `imports.tf` and delete blocks for resources you do not want Terraform
to manage. Everything Terraform imports it will also plan to change or destroy
later, so import deliberately.

### 2. Scaffold the working directory

```hcl
# versions.tf
terraform {
  required_providers {
    ory = { source = "ory/ory" }
  }
}

# provider.tf — keys come from the ORY_* environment variables
provider "ory" {
  project_id   = "<project-uuid>"
  project_slug = "<project-slug>"
}
```

Setting `project_id`/`project_slug` on the provider matters: several import IDs
(social/SAML providers, email templates, single-segment forms) resolve the
project from provider configuration.

### 3. Generate configuration

```bash
terraform init
terraform plan -generate-config-out=generated.tf
```

Expect the "Config generation is experimental" warning; that is fine. If a
resource fails to generate (e.g. conflicting or unknown attributes), remove its
import block, note it, and hand-write that resource later.

### 4. Refine the generated config

`generated.tf` is a starting point, not the final code. Apply these rules:

- **Drop `null` attributes and empty strings/lists** — they are noise and some
  (write-only arguments) are invalid to set explicitly.
- **Drop computed/read-only attributes** the API owns (`id`, timestamps,
  revision IDs, `state`, verification statuses). If Terraform errors with
  "Invalid or unknown key" or "value must be configured only by the provider",
  delete that attribute.
- **Re-supply secrets.** The API never returns SMTP connection URIs, social
  provider client secrets, or OAuth2 client secrets (values come back masked or
  absent). Wire them to `variable` blocks marked `sensitive`, or use the
  write-only variants (`client_secret_wo`, `smtp_connection_uri_wo`, ...) with
  `*_wo_version` to keep them out of state entirely.
- **Populate `ory_project_config` yourself — it generates as an empty shell.**
  The provider intentionally refreshes only attributes already tracked in
  state (so unmanaged settings never drift), and a fresh import tracks
  nothing. `-generate-config-out` therefore emits an all-null block for this
  resource. Delete the null lines and add the attributes you want Terraform
  to own, copying current values from the revision dump (`GET /projects/{id}`
  under `.services.identity.config`, `.services.oauth2.config`,
  `.services.permission.config`). The first plan shows those attributes as
  additions (`+`) even when the values match the server; the first apply just
  records them in state (the PATCH is idempotent) and subsequent plans are
  clean. Prefer the spec-derived attribute names over deprecated aliases (run
  `scripts/migrate-deprecated-attrs.sh` from the provider repo if needed).
- **Replace literal IDs with references** where resources relate, e.g.
  `project_id = ory_project.main.id` instead of the hardcoded UUID.
- **Split into files** (`project.tf`, `oauth2.tf`, `social.tf`, ...) once the
  plan converges.

### 5. Converge

```bash
terraform plan
```

Iterate on the config until the plan reports **only imports**:
`Plan: N to import, 0 to add, 0 to change, 0 to destroy.` Then:

```bash
terraform apply   # records the imports in state; changes nothing remotely
terraform plan    # must now report: No changes.
```

If the second plan still shows diffs, fix the config (not the project) and
re-plan. Typical causes are listed under Troubleshooting.

### 6. Aftercare

- Keep the `import` blocks in the repo (they are idempotent and document
  provenance) or delete them after the apply — either works.
- Add state hygiene: the state now contains project config and possibly
  private JWKS keys; store it in an encrypted backend, not in git.
- Commit the refined `.tf` files; secrets stay in variables / `*_wo` arguments.

## Resource reference

Console API = `https://api.console.ory.sh` with the workspace API key.
Project API = `https://{slug}.projects.oryapis.com` with the project API key.

| Resource | Discover via | Import ID |
|---|---|---|
| `ory_workspace` | `GET /workspaces` (console) | `{workspace_id}` |
| `ory_project` | `GET /workspaces/{ws}/projects` (console) | `{project_id}` |
| `ory_project_config` | `GET /projects/{id}` (console) | `{project_id}` |
| `ory_custom_domain` | `GET /projects/{id}/cname` (console) | `{project_id}/{domain_id}` |
| `ory_event_stream` | `GET /projects/{id}/eventstreams` (console) | `{project_id}/{stream_id}` |
| `ory_organization` | `GET /projects/{id}/organizations` (console) | `{project_id}/{org_id}` |
| `ory_project_api_key` | `GET /projects/{id}/tokens` (console) | `{project_id}/{key_id}` — value unrecoverable |
| `ory_social_provider` | revision: `.services.identity.config.selfservice.methods.oidc.config.providers[]` | `{provider_id}` (e.g. `google`) |
| `ory_saml_provider` | revision: `...methods.saml.config.providers[]` | `{provider_id}` |
| `ory_action` | revision: `...selfservice.flows.<flow>.<timing>...hooks[]` where `hook == "web_hook"` | after: `{project_id}:{flow}:after:{auth_method}:{METHOD}:{url}`; before: `{project_id}:{flow}:before:{METHOD}:{url}` |
| `ory_email_template` | revision: `...courier.templates.<base>.<valid\|invalid>` with non-empty subject/body | `{base}_{valid\|invalid}` (e.g. `recovery_code_valid`) |
| `ory_oauth2_client` | `GET /admin/clients` (project) | `{client_id}` |
| `ory_oidc_dynamic_client` | same list; clients created via DCR | `{client_id}` |
| `ory_json_web_key_set` | `GET /admin/keys/{set}` (project); no list-all endpoint, set IDs must be known | `{project_id}/{set_id}` |
| `ory_trusted_oauth2_jwt_grant_issuer` | `GET /admin/trust/grants/jwt-bearer/issuers` (project) | `{grant_id}` |
| `ory_identity` | `GET /admin/identities` (project) | `{identity_id}` |
| `ory_relationship` | `GET /namespaces` + `GET /relation-tuples?namespace=` (project) | `namespace:object#relation@subject_id`, or a subject set `namespace:object#relation@subject_ns:subject_obj#subject_rel` |
| `ory_identity_schema` | revision: `...identity.schemas[]` | **not importable** (immutable by design) |

Single-segment forms (`{domain_id}`, `{org_id}`, `{key_id}`, `{stream_id}`,
`{set_id}`) also work when the provider block sets `project_id`; prefer the
explicit `{project_id}/...` form in generated files.

## Caveats

- **Identity schemas cannot be imported.** They are immutable; leave existing
  schemas unmanaged (readable via the `ory_identity_schema` data source) and
  manage only newly created schemas.
- **Secrets never round-trip.** SMTP connection URI, social/SAML client
  secrets, OAuth2 client secrets, tokenizer template keys: re-supply via
  variables or write-only `*_wo` arguments. Until you do, some of these show a
  perpetual diff or import as empty.
- **`ory_project_api_key` values** exist only at creation time. Importing one
  yields a resource whose `value` is null; rotating it through Terraform means
  destroy + create (a brand-new key).
- **`ory_trusted_oauth2_jwt_grant_issuer.jwk` forces a one-time replace on
  import.** The public-key `jwk` is required, forces replacement when changed,
  and is never returned by the read endpoint, so import leaves it empty. Supply
  the issuer's original public JWK in config; the first apply then destroys and
  re-creates the trust once to store it (a functional no-op re-registration —
  the issuer/subject/scope are unchanged). Plans after that first apply are
  clean. If a transient re-registration is unacceptable, leave the issuer
  unmanaged.
- **Default `hydra.*` JWKS sets** (`hydra.openid.id-token`,
  `hydra.jwt.access-token`) are system-managed — do not import them. Importing
  any JWKS puts private key material into state.
- **Identities and relationship tuples are data, not configuration.** Import
  only the handful that are genuinely config-like (service accounts, seed
  tuples).
- **`ory_social_provider.auto_link`** is write-only in the API; an import will
  not populate it. Re-add it to config manually where used (enterprise
  feature).
- **Organizations require a B2B plan; event streams require an enterprise
  plan.** On other plans those endpoints return empty lists or
  `feature_not_available` errors — skip the resources.
- **Flow-level `after` hooks** (a flat `hooks` array directly under
  `flows.<flow>.after`) only occur on the recovery/verification flows in
  provider-managed configs. For those, the auth-method segment of the import
  ID is a placeholder (`_`) and is ignored by the provider.
- **Attributes with static schema defaults** (e.g. `cors_enabled`) plan a
  one-time `+ <default>` change right after import even when unconfigured,
  because import leaves them null and the default then materializes. When the
  server already holds the default value the apply is a remote no-op; set the
  attribute explicitly if you want the plan to say so.

## Troubleshooting

- **`project_id forces replacement` after import** — the `project_id` in the
  resource/provider differs from the imported one. Align them; nothing needs
  to be recreated.
- **Perpetual diff on a secret attribute** — the API returns the value masked
  (or not at all). Move it to the write-only `*_wo` variant or set the config
  value to match what you originally provisioned.
- **`403 feature_not_available`** — the attribute or resource is gated by a
  higher plan (e.g. `use_auto_link`, event streams). Remove it from config or
  have the feature enabled for the project.
- **Import of a `before`-timing action fails with `Invalid Import ID`** — the
  URL's own colons broke the import ID parser in provider versions before the
  fix for issue #280. Upgrade the provider; both documented before formats
  then work.
- **`Cannot import non-existent remote object`** — the inventory went stale:
  the resource was deleted between running the script and the plan. Remove
  that import block, or re-run the script to refresh the inventory.
- **Generated attribute rejected on plan** — delete it; it is computed-only.
  `-generate-config-out` emits every attribute it saw in state, including ones
  that are not valid to configure.
- **`Missing Required Attribute` / `Missing Configuration for Required
  Attribute` on plan (config generation succeeded)** — the attribute is
  required but the API never returns it, so `-generate-config-out` left it
  null. This hits `ory_social_provider.client_secret` (per provider type),
  `ory_trusted_oauth2_jwt_grant_issuer.jwk`, and similar. Supply the value in
  config (a `variable`, a `*_wo` argument, or the literal you originally
  provisioned). See the `jwk` caveat above for the one-time replace it causes.
