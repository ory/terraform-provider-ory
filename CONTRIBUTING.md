# Contributing to the Ory Terraform Provider

Thank you for your interest in contributing to the Ory Terraform Provider!

## Development Setup

### Prerequisites

- [Go](https://golang.org/doc/install) >= 1.21
- [Terraform](https://www.terraform.io/downloads) >= 1.0
- An [Ory Network](https://console.ory.sh/) account for testing

### Building

```bash
# Clone the repository
git clone https://github.com/ory/terraform-provider-orynetwork.git
cd terraform-provider-orynetwork

# Install development tools and set up git hooks
make tools
make hooks

# Build
make build

# Install locally
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
make test           # Run all unit tests
make test-short     # Run unit tests in short mode
```

### Acceptance Tests

Acceptance tests are **self-contained** - they automatically create a temporary Ory project, run tests against it, and clean up when done.

#### Required Environment Variables

```bash
export ORY_WORKSPACE_API_KEY="ory_wak_..."  # Workspace API key
export ORY_WORKSPACE_ID="..."               # Workspace ID
```

#### Running Acceptance Tests

```bash
# Standard acceptance tests
make test-acc

# With debug logging
make test-acc-verbose

# Run all tests with all features enabled
make test-acc-all

# Run specific resource tests
ORY_KETO_TESTS_ENABLED=true make test-acc-keto
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

#### API URL Overrides (for local development)

```bash
export ORY_CONSOLE_API_URL="https://api.console.ory.sh"      # Console API
export ORY_PROJECT_API_URL="https://%s.projects.oryapis.com" # Project API template
```

### Writing Acceptance Tests

Follow these guidelines when writing acceptance tests:

#### 1. Use the Test Utilities

```go
//go:build acceptance

package myresource_test

import (
    "testing"
    "github.com/hashicorp/terraform-plugin-testing/helper/resource"
    "github.com/ory/terraform-provider-orynetwork/internal/acctest"
)

func TestAccMyResource_basic(t *testing.T) {
    acctest.RunTest(t, resource.TestCase{
        PreCheck:                 func() { acctest.AccPreCheck(t) },
        ProtoV6ProviderFactories: acctest.TestAccProtoV6ProviderFactories(),
        Steps: []resource.TestStep{
            {
                Config: testAccMyResourceConfig(),
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

#### 2. Test Configuration Best Practices

- Use `fmt.Sprintf()` for variable injection in HCL configs
- Include the `provider "ory" {}` declaration in each config
- Test create, read, update, import, and delete operations

```go
func testAccMyResourceConfig(name string) string {
    return fmt.Sprintf(`
provider "ory" {}

resource "ory_myresource" "test" {
  name = %[1]q
}
`, name)
}
```

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
    "ory/terraform-provider-orynetwork" = "/path/to/terraform-provider-orynetwork"
  }
  direct {}
}
```

## Making Changes

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
4. Add documentation in `docs/resources/`
5. Add examples in `examples/resources/`
6. Write acceptance tests

### Resource Contribution Checklist

- [ ] Resource implements all CRUD operations
- [ ] Resource supports import via `ImportState()`
- [ ] Acceptance tests cover create, read, update, import, delete
- [ ] Tests use `acctest.RunTest()` for consistent test execution
- [ ] Documentation added to `docs/resources/`
- [ ] Examples added to `examples/resources/`
- [ ] Code passes `make lint` and `make fmt`

### Code Style

- Run `make fmt` before committing
- Run `make lint` to check for issues
- Follow existing patterns in the codebase
- Add meaningful comments for complex logic

### Commit Messages

Use clear, descriptive commit messages:

```
Add ory_foo resource for managing foos

- Implement CRUD operations
- Add acceptance tests
- Add documentation
```

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Submit a pull request

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
