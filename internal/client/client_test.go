package client

import (
	"fmt"
	"testing"

	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestNewOryClient_DefaultURLs(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   DefaultConsoleAPIURL,
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.consoleClient == nil {
		t.Error("console client should be initialized with workspace API key")
	}

	// Verify console client servers are configured
	servers := client.consoleClient.GetConfig().Servers
	if len(servers) == 0 {
		t.Error("console client should have servers configured")
	}
	if servers[0].URL != DefaultConsoleAPIURL {
		t.Errorf("expected console URL '%s', got '%s'", DefaultConsoleAPIURL, servers[0].URL)
	}
}

func TestNewOryClient_CustomConsoleURL(t *testing.T) {
	// Using example.com to demonstrate custom URL configuration
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   testutil.ExampleConsoleAPIURL,
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers := client.consoleClient.GetConfig().Servers
	if servers[0].URL != testutil.ExampleConsoleAPIURL {
		t.Errorf("expected custom console URL, got '%s'", servers[0].URL)
	}

	// Verify operation servers are also configured with custom URL
	opServers := client.consoleClient.GetConfig().OperationServers
	if createProjectServers, ok := opServers["ProjectAPIService.CreateProject"]; ok {
		if createProjectServers[0].URL != testutil.ExampleConsoleAPIURL {
			t.Errorf("expected operation server URL to be custom, got '%s'", createProjectServers[0].URL)
		}
	} else {
		t.Error("CreateProject operation server should be configured")
	}
}

func TestNewOryClient_CustomProjectURL(t *testing.T) {
	// Using example.com to demonstrate custom URL configuration
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		ProjectAPIURL: testutil.ExampleProjectAPIURL,
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.projectClient == nil {
		t.Error("project client should be initialized with project API key and slug")
	}

	servers := client.projectClient.GetConfig().Servers
	expectedURL := fmt.Sprintf(testutil.ExampleProjectAPIURL, testutil.TestProjectSlug)
	if servers[0].URL != expectedURL {
		t.Errorf("expected project URL '%s', got '%s'", expectedURL, servers[0].URL)
	}
}

func TestNewOryClient_DefaultProjectURL(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		// ProjectAPIURL is empty, should use default
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	servers := client.projectClient.GetConfig().Servers
	expectedURL := fmt.Sprintf(DefaultProjectAPIURL, testutil.TestProjectSlug)
	if servers[0].URL != expectedURL {
		t.Errorf("expected default project URL '%s', got '%s'", expectedURL, servers[0].URL)
	}
}

func TestNewOryClient_NoProjectClientWithoutSlug(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		// ProjectSlug is empty
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.projectClient != nil {
		t.Error("project client should not be initialized without project slug")
	}
}

func TestNewOryClient_NoConsoleClientWithoutWorkspaceKey(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.consoleClient != nil {
		t.Error("console client should not be initialized without workspace API key")
	}
}

func TestOryClient_Config(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ProjectAPIKey:   testutil.TestProjectAPIKey,
		ProjectID:       testutil.TestProjectID,
		ProjectSlug:     testutil.TestProjectSlug,
		WorkspaceID:     testutil.TestWorkspaceID,
		ConsoleAPIURL:   testutil.ExampleConsoleAPIURL,
		ProjectAPIURL:   testutil.ExampleProjectAPIURL,
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify config is accessible
	retrievedCfg := client.Config()
	if retrievedCfg.WorkspaceAPIKey != cfg.WorkspaceAPIKey {
		t.Error("WorkspaceAPIKey mismatch")
	}
	if retrievedCfg.ConsoleAPIURL != cfg.ConsoleAPIURL {
		t.Error("ConsoleAPIURL mismatch")
	}
	if retrievedCfg.ProjectAPIURL != cfg.ProjectAPIURL {
		t.Error("ProjectAPIURL mismatch")
	}
}

func TestOryClient_ProjectID(t *testing.T) {
	cfg := OryClientConfig{
		ProjectID: "test-project-id",
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.ProjectID() != "test-project-id" {
		t.Errorf("expected 'test-project-id', got '%s'", client.ProjectID())
	}
}

func TestOryClient_WorkspaceID(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceID: "test-workspace-id",
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.WorkspaceID() != "test-workspace-id" {
		t.Errorf("expected 'test-workspace-id', got '%s'", client.WorkspaceID())
	}
}

func TestExtractDebugInfo_NilError(t *testing.T) {
	info := extractDebugInfo(nil)
	if info.ErrorType != "<nil>" {
		t.Errorf("expected '<nil>' error type, got '%s'", info.ErrorType)
	}
}

func TestOryErrorDebugInfo_String(t *testing.T) {
	info := OryErrorDebugInfo{
		ErrorType:    "TestError",
		StatusCode:   400,
		ErrorID:      "err-123",
		ErrorMessage: "Bad Request",
		ErrorReason:  "Invalid input",
		RequestID:    "req-456",
		Feature:      "test-feature",
		RawBody:      `{"error": "test"}`,
	}

	str := info.String()
	if str == "" {
		t.Error("String() should return non-empty debug info")
	}

	// Check key information is present
	checks := []string{"TestError", "400", "err-123", "Bad Request", "Invalid input", "req-456", "test-feature"}
	for _, check := range checks {
		if !contains(str, check) {
			t.Errorf("String() should contain '%s'", check)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewOryClient_InvalidConsoleURL(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   "not-a-valid-url",
	}

	_, err := NewOryClient(cfg)
	if err == nil {
		t.Error("expected error for invalid console URL")
	}
	if !contains(err.Error(), "invalid console API URL") {
		t.Errorf("expected error message to contain 'invalid console API URL', got: %s", err.Error())
	}
}

func TestNewOryClient_InvalidProjectURL(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		ProjectAPIURL: "://invalid-url-template",
	}

	_, err := NewOryClient(cfg)
	if err == nil {
		t.Error("expected error for invalid project URL")
	}
	if !contains(err.Error(), "invalid project API URL") {
		t.Errorf("expected error message to contain 'invalid project API URL', got: %s", err.Error())
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "429 status code",
			err:      fmt.Errorf("request failed with status 429"),
			expected: true,
		},
		{
			name:     "Too Many Requests message",
			err:      fmt.Errorf("Too Many Requests"),
			expected: true,
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
		{
			name:     "500 error (not rate limit)",
			err:      fmt.Errorf("Internal Server Error 500"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRateLimitError(tt.err)
			if result != tt.expected {
				t.Errorf("isRateLimitError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "500 Internal Server Error",
			err:      fmt.Errorf("request failed with status 500"),
			expected: true,
		},
		{
			name:     "Internal Server Error message",
			err:      fmt.Errorf("Internal Server Error"),
			expected: true,
		},
		{
			name:     "502 Bad Gateway",
			err:      fmt.Errorf("request failed with status 502"),
			expected: true,
		},
		{
			name:     "Bad Gateway message",
			err:      fmt.Errorf("Bad Gateway"),
			expected: true,
		},
		{
			name:     "503 Service Unavailable",
			err:      fmt.Errorf("request failed with status 503"),
			expected: true,
		},
		{
			name:     "Service Unavailable message",
			err:      fmt.Errorf("Service Unavailable"),
			expected: true,
		},
		{
			name:     "504 Gateway Timeout",
			err:      fmt.Errorf("request failed with status 504"),
			expected: true,
		},
		{
			name:     "Gateway Timeout message",
			err:      fmt.Errorf("Gateway Timeout"),
			expected: true,
		},
		{
			name:     "404 Not Found (not retryable)",
			err:      fmt.Errorf("request failed with status 404"),
			expected: false,
		},
		{
			name:     "400 Bad Request (not retryable)",
			err:      fmt.Errorf("Bad Request"),
			expected: false,
		},
		{
			name:     "429 Rate Limit (not retryable by this function)",
			err:      fmt.Errorf("request failed with status 429"),
			expected: false,
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
