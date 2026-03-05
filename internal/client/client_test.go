package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestOryClient_EnsureProjectClient_NilWithoutCredentials(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   DefaultConsoleAPIURL,
		// No ProjectAPIKey or ProjectSlug
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.projectClient != nil {
		t.Error("project client should be nil without credentials")
	}

	err = client.ensureProjectClient()
	if err == nil {
		t.Fatal("expected error from ensureProjectClient without credentials")
	}
	if !contains(err.Error(), "project_slug and project_api_key are required") {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestOryClient_EnsureProjectClient_LazyInit(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Project client should already be initialized since credentials were provided at creation
	if client.projectClient == nil {
		t.Error("project client should be initialized when credentials provided at creation")
	}

	// ensureProjectClient should succeed (client already initialized)
	if err := client.ensureProjectClient(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOryClient_WithProjectCredentials(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   DefaultConsoleAPIURL,
		// No project credentials initially
	}

	parent, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a child client with project credentials
	child := parent.WithProjectCredentials(testutil.TestProjectSlug, testutil.TestProjectAPIKey)

	// Parent should still have no project credentials
	if parent.config.ProjectSlug != "" {
		t.Error("parent config should not be modified")
	}

	// Child should have the new credentials
	if child.config.ProjectSlug != testutil.TestProjectSlug {
		t.Errorf("expected slug '%s', got '%s'", testutil.TestProjectSlug, child.config.ProjectSlug)
	}
	if child.config.ProjectAPIKey != testutil.TestProjectAPIKey {
		t.Errorf("expected API key '%s', got '%s'", testutil.TestProjectAPIKey, child.config.ProjectAPIKey)
	}

	// Child should share the parent's console client
	if child.consoleClient != parent.consoleClient {
		t.Error("child should share the parent's console client")
	}

	// Child's project client should be lazily initialized
	if child.projectClient != nil {
		t.Error("child project client should be nil before first use")
	}

	// ensureProjectClient should initialize the child's project client
	if err := child.ensureProjectClient(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child.projectClient == nil {
		t.Error("child project client should be initialized after ensureProjectClient")
	}

	// Verify the child's project client URL
	servers := child.projectClient.GetConfig().Servers
	expectedURL := fmt.Sprintf(DefaultProjectAPIURL, testutil.TestProjectSlug)
	if servers[0].URL != expectedURL {
		t.Errorf("expected project URL '%s', got '%s'", expectedURL, servers[0].URL)
	}
}

func TestOryClient_WithProjectCredentials_Isolation(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
	}

	parent, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create two children with different credentials
	child1 := parent.WithProjectCredentials("slug-one", "key-one")
	child2 := parent.WithProjectCredentials("slug-two", "key-two")

	// Initialize both
	if err := child1.ensureProjectClient(); err != nil {
		t.Fatalf("unexpected error for child1: %v", err)
	}
	if err := child2.ensureProjectClient(); err != nil {
		t.Fatalf("unexpected error for child2: %v", err)
	}

	// Verify each child has its own project client with correct URL
	servers1 := child1.projectClient.GetConfig().Servers
	expectedURL1 := fmt.Sprintf(DefaultProjectAPIURL, "slug-one")
	if servers1[0].URL != expectedURL1 {
		t.Errorf("child1: expected project URL '%s', got '%s'", expectedURL1, servers1[0].URL)
	}

	servers2 := child2.projectClient.GetConfig().Servers
	expectedURL2 := fmt.Sprintf(DefaultProjectAPIURL, "slug-two")
	if servers2[0].URL != expectedURL2 {
		t.Errorf("child2: expected project URL '%s', got '%s'", expectedURL2, servers2[0].URL)
	}

	// Parent should still have its original project client
	parentServers := parent.projectClient.GetConfig().Servers
	expectedParent := fmt.Sprintf(DefaultProjectAPIURL, testutil.TestProjectSlug)
	if parentServers[0].URL != expectedParent {
		t.Errorf("parent: expected project URL '%s', got '%s'", expectedParent, parentServers[0].URL)
	}
}

func TestOryClient_EnsureProjectClient_MissingSlugOnly(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		// ProjectSlug is empty
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = client.ensureProjectClient()
	if err == nil {
		t.Fatal("expected error when slug is missing")
	}
	if !contains(err.Error(), "project_slug and project_api_key are required") {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestOryClient_EnsureProjectClient_MissingKeyOnly(t *testing.T) {
	cfg := OryClientConfig{
		ProjectSlug: testutil.TestProjectSlug,
		// ProjectAPIKey is empty
	}

	client, err := NewOryClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = client.ensureProjectClient()
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
	if !contains(err.Error(), "project_slug and project_api_key are required") {
		t.Errorf("unexpected error message: %s", err.Error())
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

func TestProviderUserAgent(t *testing.T) {
	expectedUserAgent := "Terraform/1.5.0 (+https://www.terraform.io) terraform-provider-ory/1.0.0"
	userAgentChan := make(chan string, 1)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgentChan <- r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	mockConfig := OryClientConfig{
		ConsoleAPIURL:   mockServer.URL + "/?%s",
		ProjectAPIURL:   mockServer.URL + "/?%s",
		ProjectSlug:     "dummy-project-slug",
		ProjectAPIKey:   "dummy-project-api-key",
		WorkspaceAPIKey: "dummy-workspace-api-key",
		UserAgent:       expectedUserAgent,
	}

	c, err := NewOryClient(mockConfig)
	if err != nil {
		t.Fatalf("Could not set up client: %v", err)
	}

	t.Run("ProjectAPI", func(t *testing.T) {
		_, resp, err := c.ProjectAPI().ProjectAPI.GetProject(context.Background(), "dummy-project-id").Execute()
		if resp != nil && resp.Body != nil {
			err = errors.Join(resp.Body.Close(), err)
		}
		if err != nil {
			t.Logf("Expected error from mock response: %v", err)
		}

		capturedUserAgent := <-userAgentChan
		if capturedUserAgent != expectedUserAgent {
			t.Errorf("Expected User-Agent %q, but got %q", expectedUserAgent, capturedUserAgent)
		}
	})

	t.Run("ConsoleAPI", func(t *testing.T) {
		_, resp, err := c.ConsoleAPI().ProjectAPI.GetProject(context.Background(), "dummy-project-id").Execute()
		if resp != nil && resp.Body != nil {
			err = errors.Join(resp.Body.Close(), err)
		}
		if err != nil {
			t.Logf("Expected error from mock response: %v", err)
		}

		capturedUserAgent := <-userAgentChan
		if capturedUserAgent != expectedUserAgent {
			t.Errorf("Expected User-Agent %q, but got %q", expectedUserAgent, capturedUserAgent)
		}
	})
}
