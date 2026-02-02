// Package testutil provides shared test utilities and constants.
package testutil

// Test URL constants - using example.com as per RFC 2606
const (
	// ExampleConsoleAPIURL is a test console API URL using example.com domain.
	ExampleConsoleAPIURL = "https://api.console.example.com"

	// ExampleProjectAPIURL is a test project API URL template using example.com domain.
	ExampleProjectAPIURL = "https://%s.projects.example.com"

	// ExampleWebhookURL is a test webhook URL for action resources.
	ExampleWebhookURL = "https://webhook.example.com"

	// ExampleAppURL is a test application URL for OAuth2 redirect URIs.
	ExampleAppURL = "https://app.example.com"

	// ExampleAPIURL is a test API URL for OAuth2 audience.
	ExampleAPIURL = "https://api.example.com"

	// ExampleEmailDomain is a test email domain using example.com as per RFC 2606.
	ExampleEmailDomain = "example.com"
)

// Test API key constants - fake keys for unit tests
// #nosec G101
const (
	// TestWorkspaceAPIKey is a fake workspace API key for tests.
	TestWorkspaceAPIKey = "ory_wak_test" //nolint:gosec // G101 false positive - this is a fake test constant

	// TestProjectAPIKey is a fake project API key for tests.
	TestProjectAPIKey = "ory_pat_test" //nolint:gosec // G101 false positive - this is a fake test constant

	// TestProjectSlug is a test project slug.
	TestProjectSlug = "test-project-slug"

	// TestProjectID is a test project ID (UUID format).
	TestProjectID = "00000000-0000-0000-0000-000000000001"

	// TestWorkspaceID is a test workspace ID (UUID format).
	TestWorkspaceID = "00000000-0000-0000-0000-000000000002"
)
