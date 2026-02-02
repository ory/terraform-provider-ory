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

SHELL := /bin/bash -o pipefail
BINARY_NAME := terraform-provider-orynetwork
INSTALL_DIR := ~/.terraform.d/plugins/registry.terraform.io/ory/orynetwork/0.0.1/$(shell go env GOOS)_$(shell go env GOARCH)

# Platform detection for tool downloads
OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH := amd64
endif
ifeq ($(ARCH),aarch64)
	ARCH := arm64
endif

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

# Ory CLI for dependency management
.bin/ory:
	@mkdir -p .bin
	@curl --retry 7 --retry-connrefused -sSfL https://raw.githubusercontent.com/ory/meta/master/install.sh | bash -s -- -d -b .bin ory v0.3.4
	@touch -a -m .bin/ory

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

# Code quality tool binaries
.bin/golangci-lint: .deps/golangci-lint.yaml .bin/ory
	@VERSION=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/golangci-lint.yaml); \
	echo "Installing golangci-lint $${VERSION}..."; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b .bin $${VERSION}

.bin/tfplugindocs: .deps/tfplugindocs.yaml .bin/ory
	@mkdir -p .bin
	@URL=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/tfplugindocs.yaml); \
	echo "Downloading tfplugindocs from $${URL}..."; \
	curl -sSfL "$${URL}" -o /tmp/tfplugindocs.zip; \
	unzip -q -o /tmp/tfplugindocs.zip -d .bin tfplugindocs; \
	rm /tmp/tfplugindocs.zip; \
	chmod +x .bin/tfplugindocs

.bin/go-licenses: .deps/go-licenses.yaml .bin/ory
	@VERSION=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/go-licenses.yaml); \
	echo "Installing go-licenses $${VERSION}..."; \
	GOBIN=$(PWD)/.bin go install github.com/google/go-licenses@$${VERSION}

.PHONY: format
format: .bin/tfplugindocs .bin/golangci-lint ## Format all code (Go, Terraform, modules, docs, lint fixes)
	go fmt ./...
	gofmt -s -w .
	terraform fmt -recursive examples/
	go mod tidy
	.bin/tfplugindocs generate --provider-name ory
	.bin/golangci-lint run --fix ./...

.PHONY: lint
lint: .bin/golangci-lint ## Run Go linter (without fixes)
	.bin/golangci-lint run ./...

.PHONY: licenses
licenses: .bin/go-licenses ## Check dependency licenses
	@# go-licenses has known issues with Go 1.25+ stdlib packages (github.com/google/go-licenses/issues/128)
	@# Suppress stderr noise about stdlib, but fail if disallowed licenses are found (exit code 3)
	@.bin/go-licenses check ./... --disallowed_types=forbidden,restricted 2>/dev/null; \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -eq 3 ]; then \
		echo "ERROR: Disallowed licenses found!"; \
		.bin/go-licenses check ./... --disallowed_types=forbidden,restricted; \
		exit 1; \
	fi; \
	echo "License check passed"

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
# SECURITY SCANNING
# ==============================================================================

.PHONY: sec
sec: sec-vuln sec-gosec sec-gitleaks ## Run all security scans

# Security tool binaries
.bin/govulncheck: .deps/govulncheck.yaml .bin/ory
	@VERSION=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/govulncheck.yaml); \
	echo "Installing govulncheck $${VERSION}..."; \
	GOBIN=$(PWD)/.bin go install golang.org/x/vuln/cmd/govulncheck@$${VERSION}

.bin/gosec: .deps/gosec.yaml .bin/ory
	@mkdir -p .bin
	@URL=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/gosec.yaml); \
	echo "Downloading gosec from $${URL}..."; \
	curl -sSfL "$${URL}" | tar -xz -C .bin gosec; \
	chmod +x .bin/gosec

.bin/gitleaks: .deps/gitleaks.yaml .bin/ory
	@mkdir -p .bin
	@URL=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/gitleaks.yaml); \
	echo "Downloading gitleaks from $${URL}..."; \
	curl -sSfL "$${URL}" | tar -xz -C .bin gitleaks; \
	chmod +x .bin/gitleaks

.bin/trivy: .deps/trivy.yaml .bin/ory
	@mkdir -p .bin
	@URL=$$(.bin/ory dev ci deps url -o $(OS) -a $(ARCH) -c .deps/trivy.yaml); \
	echo "Downloading trivy from $${URL}..."; \
	curl -sSfL "$${URL}" | tar -xz -C .bin trivy; \
	chmod +x .bin/trivy

.PHONY: sec-vuln
sec-vuln: .bin/govulncheck ## Run govulncheck for Go vulnerability scanning
	.bin/govulncheck ./...

.PHONY: sec-gosec
sec-gosec: .bin/gosec ## Run gosec for Go security analysis
	.bin/gosec ./...

.PHONY: sec-gitleaks
sec-gitleaks: .bin/gitleaks ## Run gitleaks for secret detection
	.bin/gitleaks detect --source . --verbose

.PHONY: sec-trivy
sec-trivy: .bin/trivy build ## Run trivy vulnerability scan on built binary
	.bin/trivy fs --scanners vuln,secret,misconfig --severity CRITICAL,HIGH .

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
