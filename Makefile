# Terraform Provider Ory Network
# ==============================
#
# Development and testing Makefile
#
# Required environment variables for acceptance tests:
#   ORY_WORKSPACE_API_KEY - Workspace API key
#   ORY_WORKSPACE_ID      - Workspace ID
#
# Optional environment variables:
#   ORY_CONSOLE_API_URL   - Console API URL (default: https://api.console.ory.sh)
#   ORY_PROJECT_API_URL   - Project API URL template (default: https://%s.projects.oryapis.com)

BINARY_NAME := terraform-provider-orynetwork
INSTALL_DIR := ~/.terraform.d/plugins/registry.terraform.io/ory/orynetwork/0.0.1/$(shell go env GOOS)_$(shell go env GOARCH)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==============================================================================
# DEPENDENCIES
# ==============================================================================

.PHONY: deps
deps: ## Install all dependencies (Go modules, tools)
	go mod download
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	@command -v jq >/dev/null 2>&1 || { echo "jq not found. Please install: brew install jq (macOS) or apt-get install jq (Linux)"; }

.PHONY: deps-ci
deps-ci: ## Install dependencies for CI environment
	go mod download
	@echo "Installing jq..."
	@if command -v apt-get >/dev/null 2>&1; then sudo apt-get update && sudo apt-get install -y jq; fi

# ==============================================================================
# BUILD
# ==============================================================================

.PHONY: build
build: ## Build the provider binary
	go build -o $(BINARY_NAME)

.PHONY: install
install: build ## Install provider to local Terraform plugins directory
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY_NAME)

# ==============================================================================
# CODE QUALITY
# ==============================================================================

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...
	gofmt -s -w .

.PHONY: fmt-check
fmt-check: ## Check Go code formatting (fails if not formatted)
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files are not formatted correctly:"; \
		gofmt -l .; \
		echo ""; \
		echo "Run 'make fmt' to fix."; \
		exit 1; \
	fi

.PHONY: fmt-tf
fmt-tf: ## Format Terraform example files
	terraform fmt -recursive examples/

.PHONY: fmt-tf-check
fmt-tf-check: ## Check Terraform formatting (fails if not formatted)
	@if ! terraform fmt -check -recursive examples/; then \
		echo ""; \
		echo "Terraform files in examples/ are not formatted correctly."; \
		echo "Run 'make fmt-tf' to fix."; \
		exit 1; \
	fi

.PHONY: fmt-all
fmt-all: fmt fmt-tf ## Format all code (Go and Terraform)

.PHONY: lint
lint: ## Run Go linter
	golangci-lint run ./...

.PHONY: lint-tf
lint-tf: fmt-tf-check ## Check Terraform formatting (alias for fmt-tf-check)

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: generate
generate: ## Generate documentation and code
	go generate ./...

.PHONY: docs
docs: ## Generate Terraform documentation
	@command -v tfplugindocs >/dev/null 2>&1 || { echo "Installing tfplugindocs..."; go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest; }
	tfplugindocs generate

.PHONY: docs-check
docs-check: docs ## Check if documentation is up to date
	@if ! git diff --exit-code docs/; then \
		echo "Documentation is out of date. Run 'make docs' and commit the changes."; \
		exit 1; \
	fi

.PHONY: mod-tidy
mod-tidy: ## Tidy go modules
	go mod tidy

.PHONY: mod-check
mod-check: mod-tidy ## Check if go.mod/go.sum are up to date
	@if ! git diff --exit-code go.mod go.sum; then \
		echo "go.mod or go.sum is out of date. Run 'go mod tidy' and commit the changes."; \
		exit 1; \
	fi

.PHONY: check
check: fmt-all vet lint ## Format and run all code quality checks

.PHONY: ci
ci: fmt-check fmt-tf-check lint mod-check docs-check ## Run all CI checks (matches GitHub Actions, fails on issues)

# ==============================================================================
# TESTING
# ==============================================================================

.PHONY: test
test: ## Run unit tests (no API calls)
	go test -v -cover ./...

.PHONY: test-short
test-short: ## Run unit tests in short mode
	go test -v -short ./...

.PHONY: test-acc
test-acc: env-check ## Run acceptance tests (creates single shared project)
	@echo "Running acceptance tests with shared project..."
	./scripts/run-acceptance-tests.sh -p 1 -v -timeout 30m ./...

.PHONY: test-acc-verbose
test-acc-verbose: env-check ## Run acceptance tests with debug logging
	@echo "Running acceptance tests with debug logging..."
	TF_LOG=DEBUG ./scripts/run-acceptance-tests.sh -p 1 -v -timeout 30m ./...

.PHONY: test-acc-keto
test-acc-keto: env-check ## Run only keto/relationship tests
	@echo "Running relationship/keto tests..."
	ORY_KETO_TESTS_ENABLED=true ./scripts/run-acceptance-tests.sh -p 1 -v -timeout 30m ./internal/resources/relationship/...

.PHONY: test-acc-all
test-acc-all: env-check ## Run all acceptance tests including optional ones
	@echo "Running all acceptance tests with all features enabled..."
	ORY_KETO_TESTS_ENABLED=true \
		ORY_B2B_ENABLED=true \
		ORY_SOCIAL_PROVIDER_TESTS_ENABLED=true \
		ORY_SCHEMA_TESTS_ENABLED=true \
		./scripts/run-acceptance-tests.sh -p 1 -v -timeout 30m ./...

# ==============================================================================
# ENVIRONMENT HELPERS
# ==============================================================================

.PHONY: env-check
env-check: ## Check required environment variables
	@echo "Required environment variables:"
	@if [ -z "$$ORY_WORKSPACE_API_KEY" ]; then echo "  ORY_WORKSPACE_API_KEY: NOT SET (required)"; exit 1; else echo "  ORY_WORKSPACE_API_KEY: set"; fi
	@if [ -z "$$ORY_WORKSPACE_ID" ]; then echo "  ORY_WORKSPACE_ID: NOT SET (required)"; exit 1; else echo "  ORY_WORKSPACE_ID: $$ORY_WORKSPACE_ID"; fi
	@echo ""
	@echo "Optional API URL overrides:"
	@if [ -z "$$ORY_CONSOLE_API_URL" ]; then echo "  ORY_CONSOLE_API_URL: (using default)"; else echo "  ORY_CONSOLE_API_URL: $$ORY_CONSOLE_API_URL"; fi
	@if [ -z "$$ORY_PROJECT_API_URL" ]; then echo "  ORY_PROJECT_API_URL: (using default)"; else echo "  ORY_PROJECT_API_URL: $$ORY_PROJECT_API_URL"; fi
	@echo ""
	@echo "Optional test feature flags (set to 'true' to enable):"
	@if [ "$$ORY_KETO_TESTS_ENABLED" = "true" ]; then echo "  ORY_KETO_TESTS_ENABLED: true"; else echo "  ORY_KETO_TESTS_ENABLED: (not set - relationship tests will be skipped)"; fi
	@if [ "$$ORY_B2B_ENABLED" = "true" ]; then echo "  ORY_B2B_ENABLED: true"; else echo "  ORY_B2B_ENABLED: (not set - organization tests will be skipped)"; fi
	@if [ "$$ORY_SOCIAL_PROVIDER_TESTS_ENABLED" = "true" ]; then echo "  ORY_SOCIAL_PROVIDER_TESTS_ENABLED: true"; else echo "  ORY_SOCIAL_PROVIDER_TESTS_ENABLED: (not set)"; fi
	@if [ "$$ORY_SCHEMA_TESTS_ENABLED" = "true" ]; then echo "  ORY_SCHEMA_TESTS_ENABLED: true"; else echo "  ORY_SCHEMA_TESTS_ENABLED: (not set - schema tests will be skipped)"; fi
	@if [ "$$ORY_PROJECT_TESTS_ENABLED" = "true" ]; then echo "  ORY_PROJECT_TESTS_ENABLED: true"; else echo "  ORY_PROJECT_TESTS_ENABLED: (not set - project resource tests will be skipped)"; fi
