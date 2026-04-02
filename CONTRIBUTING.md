# Contributing to the Ory Terraform Provider

Thank you for your interest in contributing to the Ory Terraform Provider!

## Development Setup

### Prerequisites

- [Go](https://golang.org/doc/install) (see version in `go.mod`)
- [Terraform](https://www.terraform.io/downloads) >= 1.0
- An [Ory Network](https://console.ory.sh/) account for acceptance testing

### Building

```bash
# Clone the repository
git clone https://github.com/ory/terraform-provider-ory.git
cd terraform-provider-ory

# Install dependencies
make deps

# Set up pre-commit hooks
git config core.hooksPath .githooks

# Build
make build

# Install locally (to ~/.terraform.d/plugins/)
make install
```

## Testing

### Test Types

The provider has two types of tests:

1. **Unit Tests** - Fast, isolated tests that don't require API access
2. **Acceptance Tests** - Integration tests that create real resources in Ory Network

### Unit Tests

Unit tests can be run without any credentials:

```bash
make test           # Run all unit tests with coverage
make test-short     # Run unit tests in short mode (CI runs these with coverage enabled)
```

### Acceptance Tests

Acceptance tests run against a **pre-created Ory project**. The project must be configured with keto namespaces and dynamic client registration enabled (the CI project already has this).

#### Setup

1. Copy `.env.example` to `.env` and fill in your credentials:

```bash
cp .env.example .env
```

The `.env` file is gitignored and automatically loaded by `make` targets.

**Required** (validated by `make env-check`):

```bash
ORY_WORKSPACE_API_KEY=ory_wak_...
ORY_WORKSPACE_ID=...
```

**Recommended** (needed by most resource tests):

```bash
ORY_PROJECT_ID=...
ORY_PROJECT_SLUG=...
ORY_PROJECT_API_KEY=ory_pat_...
ORY_PROJECT_ENVIRONMENT=prod
```

When set, tests use this persistent project instead of creating ephemeral ones. The project must have keto namespaces and dynamic client registration configured. See `.env.example` for the full list of variables.

#### Running Acceptance Tests

```bash
make test-acc              # Standard acceptance tests
make test-acc-verbose      # With debug logging
make test-acc-all          # All tests with all features enabled
make test-acc-keto         # Run specific resource tests
```

#### Optional Feature Flags

Some tests require specific Ory plan features. Enable them with environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `ORY_KETO_TESTS_ENABLED=true` | Run relationship/Keto tests |
| `ORY_B2B_ENABLED=true` | Run B2B/organization tests (requires B2B plan) |
| `ORY_SOCIAL_PROVIDER_TESTS_ENABLED=true` | Run social provider tests |
| `ORY_SCHEMA_TESTS_ENABLED=true` | Run identity schema tests |
| `ORY_PROJECT_TESTS_ENABLED=true` | Run project creation/deletion tests |
| `ORY_EVENT_STREAM_TESTS_ENABLED=true` | Run event stream tests (requires Enterprise plan + AWS setup below) |

> **Note:** CI enables **all** feature flags, including `ORY_PROJECT_TESTS_ENABLED`, on pull requests. Locally, `make test-acc-all` enables all flags **except** `ORY_PROJECT_TESTS_ENABLED` by default (project creation/deletion tests are excluded because they are slow and potentially destructive). To run those locally, set `ORY_PROJECT_TESTS_ENABLED=true` explicitly.

#### Event Stream Tests

Event stream tests have additional requirements beyond a feature flag because they interact with real AWS infrastructure:

1. **AWS SNS topic** — a real SNS topic to receive events
2. **AWS IAM role** — with `sns:Publish` permission on the topic and a trust policy allowing Ory's AWS account to assume it
3. **IAM trust policy ExternalId** — must match `ORY_PROJECT_ID`. The Ory API validates that the trust policy includes a `StringEquals` condition on `sts:ExternalId` matching the project UUID, so the test project and IAM role are tightly coupled.

```bash
# Add to .env (in addition to the required vars above):
ORY_EVENT_STREAM_TESTS_ENABLED=true
ORY_EVENT_STREAM_TOPIC_ARN=arn:aws:sns:...
ORY_EVENT_STREAM_ROLE_ARN=arn:aws:iam::...:role/...
```

See the [Ory live events documentation](https://www.ory.com/docs/actions/live-events) for full AWS setup instructions.

### Writing Acceptance Tests

Follow these guidelines when writing acceptance tests:

#### 1. Use the Test Utilities

```go
//go:build acceptance

package myresource_test

import (
    "testing"
    "github.com/hashicorp/terraform-plugin-testing/helper/resource"
    "github.com/ory/terraform-provider-ory/internal/acctest"
)

func TestAccMyResource_basic(t *testing.T) {
    acctest.RunTest(t, resource.TestCase{
        PreCheck:                 func() { acctest.AccPreCheck(t) },
        ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
        Steps: []resource.TestStep{
            {
                Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
                    "Name": "Test Resource",
                }),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttrSet("ory_myresource.test", "id"),
                ),
            },
            {
                ResourceName:      "ory_myresource.test",
                ImportState:       true,
                ImportStateVerify: true,
            },
        },
    })
}
```

#### 2. Test Configuration with `testdata/` Templates

Store Terraform configurations in `testdata/` files, not inline strings. Use `acctest.LoadTestConfig()` to load and render them:

```go
// In your test function:
Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", map[string]string{
    "Name": "My Resource",
})
```

**Template files** use `[[ ]]` delimiters (to avoid conflicts with Terraform's `{{ }}`):

```hcl
# testdata/basic.tf.tmpl
resource "ory_myresource" "test" {
  name = "[[ .Name ]]"
}
```

The `provider "ory" {}` block is automatically prepended — don't include it in template files.

For configs with no variables, pass `nil`:

```go
Config: acctest.LoadTestConfig(t, "testdata/basic.tf.tmpl", nil)
```

**Guidelines:**
- One `.tf.tmpl` file per test scenario in each resource's `testdata/` directory
- Use descriptive filenames: `basic.tf.tmpl`, `updated.tf.tmpl`, `with_audience.tf.tmpl`
- Test create, read, update, import, and delete operations
- Pass dynamic values (URLs, names) via the template data map using `testutil` constants

#### 3. Feature-Gated Tests

For tests requiring specific Ory plan features:

```go
func TestAccOrganizationResource_basic(t *testing.T) {
    acctest.RequireB2BTests(t)  // Skips if ORY_B2B_ENABLED != "true"
    // ... test implementation
}

func TestAccRelationshipResource_basic(t *testing.T) {
    acctest.RequireKetoTests(t)  // Skips if ORY_KETO_TESTS_ENABLED != "true"
    // ... test implementation
}
```

### Using Local Provider

To use a locally built provider, create a `~/.terraformrc` file:

```hcl
provider_installation {
  dev_overrides {
    "ory/ory" = "/path/to/terraform-provider-ory"
  }
  direct {}
}
```

## Making Changes

### Project Config Codegen

The `ory_project_config` resource uses code generation to avoid maintaining schema, patch, and read mappings by hand. A codegen tool reads `internal/codegen/mappings.yaml` and generates three Go files.

#### How it works

```
mappings.yaml                    resource.go (hand-written)
    |                                |
    v                                v
make generate ──> schema_gen.go    custom schema attrs (CORS, tokenizer, courier, etc.)
                  patches_gen.go   custom patch logic (remove-on-empty, keto namespaces, etc.)
                  read_gen.go      custom read logic (CORS structs, sensitive fields, etc.)
                        |                |
                        v                v
                  Schema() merges generated + custom at runtime
                  buildPatches() iterates generated tables + custom
                  readProjectConfig() calls readSimpleFields() + custom
```

The generated `*_gen.go` files are **overwritten** on every `make generate`. Hand-written code in `resource.go` and `helpers.go` is **never touched** by the codegen tool.

#### The "governs" pattern

The published OpenAPI spec (`ory/client-go`) includes descriptions like:

```
This governs the "session.lifespan" setting.
```

The codegen tool uses this to automatically derive JSON Patch paths:
- `kratos_` prefix + `governs "session.lifespan"` → `/services/identity/config/session/lifespan`
- `hydra_` prefix + `governs "ttl.access_token"` → `/services/oauth2/config/ttl/access_token`

When the OpenAPI spec is present locally (`internal/codegen/openapi.yaml`), `make generate` automatically uses it to validate and derive paths. If the spec is not present, it falls back to the explicit `patch_path` in `mappings.yaml`.

#### Adding a new simple config field

1. Check if the field exists in the OpenAPI spec:

```bash
make download-spec   # Download latest spec
make discover        # Shows unmapped spec properties with YAML entries ready to copy
```

2. Add an entry to `internal/codegen/mappings.yaml`:

```yaml
# Preferred: link to OpenAPI property (patch_path auto-derived from governs)
- name: my_new_field
  go_field: MyNewField
  type: string
  openapi_property: kratos_my_new_field
  description: "What this field does."

# Fallback: explicit patch_path (when the spec property has no governs description)
- name: my_new_field
  go_field: MyNewField
  type: string
  patch_path: /services/identity/config/path/to/field
  description: "What this field does."
```

3. Add the struct field to `ProjectConfigResourceModel` in `resource.go`:

```go
MyNewField types.String `tfsdk:"my_new_field"`
```

4. Run `make generate && make format`.

That's it — the field appears in the Terraform schema, JSON Patch operations, and API response reading. No other Go code changes needed.

#### mappings.yaml fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Terraform attribute name (e.g., `session_lifespan`) |
| `go_field` | Yes | Go struct field name (e.g., `SessionLifespan`) |
| `type` | Yes | `string`, `bool`, `int64`, `list_string`, `map_string` |
| `patch_path` | * | JSON Patch path. Required unless `openapi_property` is set and the spec has a governs description |
| `openapi_property` | No | Maps to `normalizedProjectRevision` property name. The governs description is used to derive `patch_path` |
| `description` | Yes | Short description for Terraform docs (auto-generated by `tfplugindocs`) |
| `computed` | No | If `true`, attribute has a server-side default |
| `default_bool` | No | Static default for bool attributes |
| `default_int64` | No | Static default for int64 attributes |
| `sensitive` | No | If `true`, value is masked in Terraform output |
| `skip_empty_read` | No | If `true`, skip reading empty strings from API (used for account_experience fields) |
| `validators` | No | `one_of: [...]` or `regex: "..."` + `regex_message: "..."` |
| `deprecated_name` | No | Old terraform attribute name (shows deprecation warning in Terraform) |
| `deprecated_go_field` | No | Old Go struct field name for the deprecated attribute |

#### Makefile targets

| Target | Description |
|--------|-------------|
| `make generate` | Generate Go files from mappings.yaml. Auto-validates against the OpenAPI spec if `internal/codegen/openapi.yaml` exists |
| `make download-spec` | Download the latest OpenAPI spec from `ory/client-go` |
| `make discover` | Download spec and output YAML entries + Go struct fields for all unmapped properties |
| `make check-coverage` | Download spec and fail if any properties are unmapped (used in CI to detect drift) |

#### CI drift detection

A daily GitHub Actions workflow (`.github/workflows/regenerate-config.yml`) runs at 09:00 UTC and:
1. Downloads the latest OpenAPI spec from `ory/client-go`
2. Regenerates code and creates a PR if the generated files changed
3. Runs `--strict` mode to detect unmapped properties
4. If new properties are found, creates a GitHub issue with the `codegen-drift` label listing the new properties and instructions to add them

#### Renaming attributes (deprecated aliases)

When renaming an attribute, use `deprecated_name` / `deprecated_go_field` to keep the old name working with a deprecation warning:

```yaml
- name: selfservice_methods_password_enabled   # new spec-derived name
  go_field: SelfserviceMethodsPasswordEnabled
  type: bool
  patch_path: /services/identity/config/selfservice/methods/password/enabled
  description: "Enable password authentication."
  deprecated_name: enable_password              # old name (shows warning)
  deprecated_go_field: EnablePassword
```

Both names work. When a user uses the old name, Terraform shows:
```
Warning: Argument is deprecated
Use selfservice_methods_password_enabled instead. This attribute will be removed in a future major version.
```

The model struct needs BOTH fields (old and new). The codegen generates:
- Both schema attributes (new is normal, old has `DeprecationMessage`)
- Patch logic that checks the new field first, falls back to the deprecated
- Read logic that writes to whichever field is set in state

#### Migrating to spec-derived attribute names

53 attributes have been renamed from short user-friendly names to spec-derived names for consistency. The old names still work but show deprecation warnings. A migration script is provided:

```bash
# Preview changes (creates .tf.bak backups)
./scripts/migrate-deprecated-attrs.sh .

# Verify no infrastructure changes
terraform plan
```

The script renames all deprecated attributes in `.tf` files. After running, verify with `terraform plan` that no changes are detected.

#### Complex types (not codegen'd)

These require hand-written code in `resource.go` because they have non-trivial serialization:

- CORS settings (top-level project struct, not nested config map)
- Keto namespaces (list of `{name, id}` objects)
- Session tokenizer templates (nested map with sensitive fields)
- Courier channels (list of objects with discriminated auth config)
- Courier HTTP request config (nested object with auth sub-object)
- `default_return_url` / `allowed_return_urls` (remove-on-empty semantics)
- `smtp_connection_uri` (sensitive, write-only — not read from API)
- `mfa_enforcement` (maps string values to different API fields)

### Adding a New Resource

1. Create a new package in `internal/resources/`
2. Implement the resource with these methods:
   - `Metadata()` - Resource type name
   - `Schema()` - Resource schema definition
   - `Configure()` - Provider configuration
   - `Create()` - Create the resource
   - `Read()` - Read the resource state
   - `Update()` - Update the resource
   - `Delete()` - Delete the resource
   - `ImportState()` - Import existing resources
3. Register the resource in `internal/provider/provider.go`
4. Add a documentation **template** in `templates/resources/` (NOT `docs/resources/`)
5. Add examples in `examples/resources/`
6. Write acceptance tests
7. Run `make format` to regenerate docs

### Documentation

Documentation is auto-generated using [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs).

**Important:** Never edit files in `docs/` directly. They are overwritten on every `make format`.

Instead, edit the **templates** in `templates/`:
- `templates/resources/<name>.md.tmpl` — Resource documentation
- `templates/data-sources/<name>.md.tmpl` — Data source documentation
- `templates/index.md.tmpl` — Provider documentation

Templates use Go template syntax:
```
{{ .SchemaMarkdown | trimspace }}     → auto-generated schema table
{{ tffile "examples/resources/..." }} → embeds an example .tf file
{{ .Name }}, {{ .Type }}              → resource metadata
```

After editing templates, run `make format` to regenerate docs.

### Resource Contribution Checklist

- [ ] Resource implements all CRUD operations
- [ ] Resource supports import via `ImportState()`
- [ ] Acceptance tests cover create, read, update, import, delete
- [ ] Tests use `acctest.RunTest()` for consistent test execution
- [ ] Test configs stored in `testdata/` directory (not inline strings)
- [ ] Documentation template added to `templates/resources/`
- [ ] Examples added to `examples/resources/`
- [ ] Code passes `make format` (includes lint, fmt, and doc generation)

### Pre-Commit Checklist

Run these checks locally before committing. They mirror what CI runs on every pull request.

#### Required

```bash
make generate       # Regenerate code from mappings.yaml (if you changed project_config)
make build          # Verify the provider compiles
make format         # Format code, tidy modules, regenerate docs, fix lint issues
make test-short     # Run unit tests in short mode (matches CI)
```

`make format` runs several tools in sequence:
- `go fmt` + `gofmt -s` — Go formatting
- `terraform fmt` — HCL formatting for examples
- `go mod tidy` — dependency cleanup
- `tfplugindocs generate` — regenerate `docs/` from templates
- `golangci-lint --fix` — lint with auto-fix

#### Recommended

```bash
make sec            # Run security scans (govulncheck + gosec + gitleaks)
make sec-trivy      # Run trivy vulnerability scan (requires build first)
make licenses       # Check dependency licenses
```

You can also run security scans individually:

| Command | Tool | What it checks |
|---------|------|----------------|
| `make sec-vuln` | govulncheck | Known Go vulnerabilities |
| `make sec-gosec` | gosec | Go security patterns (injection, file traversal, etc.) |
| `make sec-gitleaks` | gitleaks | Hardcoded secrets and credentials |
| `make sec-trivy` | trivy | Vulnerability, secret, and misconfig scanning (not included in `make sec`) |

> **Note:** `make sec-trivy` is **not** included in `make sec` — it must be run separately and requires a prior `make build`.

#### Quick Reference

```bash
# Minimum before committing:
make generate && make build && make format && make test-short

# Full CI-equivalent check:
make generate && make build && make format && make test-short && make sec && make sec-trivy && make licenses
```

### Code Style

- Follow existing patterns in the codebase
- Add meaningful comments for complex logic
- Use `#nosec` annotations for false positives (with a justification comment)

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
feat: add ory_foo resource for managing foos
fix: handle nil pointer in OAuth2 client read
refactor: extract test configs into testdata templates
docs: add algorithm guidance to JWK docs
```

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Run checks: `make build && make format && make test-short`
5. Submit a pull request using the PR template

Please include:

- Description of the changes
- Link to any related issues
- Test results or screenshots if applicable

## Reporting Issues

When reporting issues, please include:

- Terraform version (`terraform version`)
- Provider version
- Relevant Terraform configuration (sanitized of secrets)
- Expected behavior
- Actual behavior
- Steps to reproduce

## Code of Conduct

Please be respectful and constructive in all interactions. We're all here to build something useful together.
