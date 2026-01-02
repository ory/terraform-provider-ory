#!/bin/bash
# run-acceptance-tests.sh - Creates a shared test project, runs all acceptance tests, and cleans up
#
# Usage:
#   ./scripts/run-acceptance-tests.sh [go test flags...]
#
# Required environment variables:
#   ORY_WORKSPACE_API_KEY - Workspace API key
#   ORY_WORKSPACE_ID      - Workspace ID
#
# Optional environment variables:
#   ORY_CONSOLE_API_URL   - Console API URL (default: https://api.console.ory.sh)
#   ORY_PROJECT_API_URL   - Project API URL pattern (default: https://%s.projects.oryapis.com)
#   ORY_KETO_TESTS_ENABLED           - Enable Keto/relationship tests
#   ORY_B2B_ENABLED                  - Enable B2B/organization tests
#   ORY_SOCIAL_PROVIDER_TESTS_ENABLED - Enable social provider tests
#   ORY_SCHEMA_TESTS_ENABLED         - Enable identity schema tests
#   ORY_PROJECT_TESTS_ENABLED        - Enable project create/delete tests

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

parse_curl_response() {
    local response="$1"
    local http_code
    local body

    # Get last line (HTTP code)
    http_code=$(echo "$response" | tail -n1)

    # Get all lines except the last (body) - works on both BSD and GNU
    body=$(echo "$response" | sed '$d')

    echo "$body"
    echo "$http_code"
}

# Configuration
CONSOLE_API_URL="${ORY_CONSOLE_API_URL:-https://api.console.ory.sh}"
PROJECT_API_URL="${ORY_PROJECT_API_URL:-https://%s.projects.oryapis.com}"
PROJECT_NAME="tf-acc-test-$(date +%s)"

# Validate required environment variables
if [[ -z "${ORY_WORKSPACE_API_KEY:-}" ]]; then
    echo -e "${RED}Error: ORY_WORKSPACE_API_KEY is required${NC}"
    exit 1
fi

if [[ -z "${ORY_WORKSPACE_ID:-}" ]]; then
    echo -e "${RED}Error: ORY_WORKSPACE_ID is required${NC}"
    exit 1
fi

# Cleanup function - always runs on exit
cleanup() {
    local exit_code=$?
    if [[ -n "${PROJECT_ID:-}" ]]; then
        echo -e "\n${YELLOW}Cleaning up test project: ${PROJECT_ID}${NC}"

        # Delete the project
        local delete_response
        delete_response=$(curl -s -w "\n%{http_code}" -X DELETE \
            "${CONSOLE_API_URL}/projects/${PROJECT_ID}" \
            -H "Authorization: Bearer ${ORY_WORKSPACE_API_KEY}" \
            -H "Content-Type: application/json" 2>&1) || true

        local http_code
        http_code=$(echo "$delete_response" | tail -n1)

        if [[ "$http_code" == "204" ]] || [[ "$http_code" == "200" ]]; then
            echo -e "${GREEN}Successfully deleted test project${NC}"
        else
            echo -e "${YELLOW}Warning: Failed to delete test project (HTTP ${http_code})${NC}"
            echo "$delete_response" | sed '$d'
        fi
    fi
    exit $exit_code
}

trap cleanup EXIT

echo -e "${GREEN}Creating shared test project: ${PROJECT_NAME}${NC}"

# Create the project
create_response=$(curl -s -w "\n%{http_code}" -X POST \
    "${CONSOLE_API_URL}/projects" \
    -H "Authorization: Bearer ${ORY_WORKSPACE_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"${PROJECT_NAME}\",
        \"environment\": \"prod\",
        \"workspace_id\": \"${ORY_WORKSPACE_ID}\"
    }")

http_code=$(echo "$create_response" | tail -n1)
response_body=$(echo "$create_response" | sed '$d')

if [[ "$http_code" != "201" ]] && [[ "$http_code" != "200" ]]; then
    echo -e "${RED}Error: Failed to create project (HTTP ${http_code})${NC}"
    echo "$response_body"
    exit 1
fi

# Extract project details
PROJECT_ID=$(echo "$response_body" | jq -r '.id')
PROJECT_SLUG=$(echo "$response_body" | jq -r '.slug')
PROJECT_ENV=$(echo "$response_body" | jq -r '.environment')

if [[ -z "$PROJECT_ID" ]] || [[ "$PROJECT_ID" == "null" ]]; then
    echo -e "${RED}Error: Failed to extract project ID from response${NC}"
    echo "$response_body"
    exit 1
fi

echo -e "${GREEN}Created project: ${PROJECT_ID} (slug: ${PROJECT_SLUG}, environment: ${PROJECT_ENV})${NC}"

# Create API key for the project
echo -e "${GREEN}Creating API key for project...${NC}"

apikey_response=$(curl -s -w "\n%{http_code}" -X POST \
    "${CONSOLE_API_URL}/projects/${PROJECT_ID}/tokens" \
    -H "Authorization: Bearer ${ORY_WORKSPACE_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"name": "tf-acc-test-key"}')

http_code=$(echo "$apikey_response" | tail -n1)
response_body=$(echo "$apikey_response" | sed '$d')

if [[ "$http_code" != "201" ]] && [[ "$http_code" != "200" ]]; then
    echo -e "${RED}Error: Failed to create API key (HTTP ${http_code})${NC}"
    echo "$response_body"
    exit 1
fi

PROJECT_API_KEY=$(echo "$response_body" | jq -r '.value')

if [[ -z "$PROJECT_API_KEY" ]] || [[ "$PROJECT_API_KEY" == "null" ]]; then
    echo -e "${RED}Error: Failed to extract API key from response${NC}"
    echo "$response_body"
    exit 1
fi

echo -e "${GREEN}Created API key for project${NC}"

# Configure Keto namespaces for relationship tests
echo -e "${GREEN}Configuring Keto namespaces...${NC}"

patch_response=$(curl -s -w "\n%{http_code}" -X PATCH \
    "${CONSOLE_API_URL}/projects/${PROJECT_ID}" \
    -H "Authorization: Bearer ${ORY_WORKSPACE_API_KEY}" \
    -H "Content-Type: application/json" \
    -d '[{
        "op": "add",
        "path": "/services/permission/config/namespaces",
        "value": [
            {"name": "documents", "id": 1},
            {"name": "folders", "id": 2},
            {"name": "groups", "id": 3},
            {"name": "users", "id": 4}
        ]
    }]')

http_code=$(echo "$patch_response" | tail -n1)
if [[ "$http_code" == "200" ]] || [[ "$http_code" == "204" ]]; then
    echo -e "${GREEN}Configured Keto namespaces${NC}"
else
    echo -e "${YELLOW}Warning: Failed to configure Keto namespaces (HTTP ${http_code}) - relationship tests may fail${NC}"
fi

# Export environment variables for tests
export TF_ACC=1
export ORY_PROJECT_ID="$PROJECT_ID"
export ORY_PROJECT_SLUG="$PROJECT_SLUG"
export ORY_PROJECT_API_KEY="$PROJECT_API_KEY"
export ORY_PROJECT_ENVIRONMENT="$PROJECT_ENV"
export ORY_TEST_PROJECT_PRECREATED=1

echo ""
echo -e "${GREEN}Running acceptance tests...${NC}"
echo "  Project ID:   ${PROJECT_ID}"
echo "  Project Slug: ${PROJECT_SLUG}"
echo "  Environment:  ${PROJECT_ENV}"
echo ""

# Default test flags if none provided
TEST_FLAGS=("$@")
if [[ ${#TEST_FLAGS[@]} -eq 0 ]]; then
    TEST_FLAGS=(-p 1 -v -timeout 30m ./...)
fi

# Run the tests with acceptance build tag
go test -tags acceptance "${TEST_FLAGS[@]}"
