package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/testutil"
)

func TestNewOryClient_DefaultURLs(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   DefaultConsoleAPIURL,
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	assert.NotNil(t, client.consoleClient, "console client should be initialized with workspace API key")

	// Verify console client servers are configured
	servers := client.consoleClient.GetConfig().Servers
	require.NotEmpty(t, servers, "console client should have servers configured")
	assert.Equal(t, DefaultConsoleAPIURL, servers[0].URL)
}

func TestNewOryClient_CustomConsoleURL(t *testing.T) {
	// Using example.com to demonstrate custom URL configuration
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   testutil.ExampleConsoleAPIURL,
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	servers := client.consoleClient.GetConfig().Servers
	assert.Equal(t, testutil.ExampleConsoleAPIURL, servers[0].URL)

	// Verify operation servers are also configured with custom URL
	opServers := client.consoleClient.GetConfig().OperationServers
	createProjectServers, ok := opServers["ProjectAPIService.CreateProject"]
	require.True(t, ok, "CreateProject operation server should be configured")
	assert.Equal(t, testutil.ExampleConsoleAPIURL, createProjectServers[0].URL)
}

func TestNewOryClient_CustomProjectURL(t *testing.T) {
	// Using example.com to demonstrate custom URL configuration
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		ProjectAPIURL: testutil.ExampleProjectAPIURL,
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	require.NotNil(t, client.projectClient, "project client should be initialized with project API key and slug")

	servers := client.projectClient.GetConfig().Servers
	expectedURL := fmt.Sprintf(testutil.ExampleProjectAPIURL, testutil.TestProjectSlug)
	assert.Equal(t, expectedURL, servers[0].URL)
}

func TestNewOryClient_DefaultProjectURL(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		// ProjectAPIURL is empty, should use default
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	servers := client.projectClient.GetConfig().Servers
	expectedURL := fmt.Sprintf(DefaultProjectAPIURL, testutil.TestProjectSlug)
	assert.Equal(t, expectedURL, servers[0].URL)
}

func TestNewOryClient_NoProjectClientWithoutSlug(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		// ProjectSlug is empty
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	assert.Nil(t, client.projectClient, "project client should not be initialized without project slug")
}

func TestNewOryClient_NoConsoleClientWithoutWorkspaceKey(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	assert.Nil(t, client.consoleClient, "console client should not be initialized without workspace API key")
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
	require.NoError(t, err)

	// Verify config is accessible
	retrievedCfg := client.Config()
	assert.Equal(t, cfg.WorkspaceAPIKey, retrievedCfg.WorkspaceAPIKey)
	assert.Equal(t, cfg.ConsoleAPIURL, retrievedCfg.ConsoleAPIURL)
	assert.Equal(t, cfg.ProjectAPIURL, retrievedCfg.ProjectAPIURL)
}

func TestOryClient_ProjectID(t *testing.T) {
	cfg := OryClientConfig{
		ProjectID: "test-project-id",
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	assert.Equal(t, "test-project-id", client.ProjectID())
}

func TestOryClient_WorkspaceID(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceID: "test-workspace-id",
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	assert.Equal(t, "test-workspace-id", client.WorkspaceID())
}

func TestExtractDebugInfo_NilError(t *testing.T) {
	info := extractDebugInfo(nil)
	assert.Equal(t, "<nil>", info.ErrorType)
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
	require.NotEmpty(t, str, "String() should return non-empty debug info")

	// Check key information is present
	checks := []string{"TestError", "400", "err-123", "Bad Request", "Invalid input", "req-456", "test-feature"}
	for _, check := range checks {
		assert.Contains(t, str, check)
	}
}

func TestWrapAPIError_Nil(t *testing.T) {
	assert.NoError(t, wrapAPIError(nil, "patching project"))
}

// autoLinkFeatureErrorBody is the exact 403 body the Ory console API returns
// when enabling the OIDC auto-link policy on a project whose subscription plan
// does not include the "Auto Link Policy" feature (captured against the live
// production API). The provider must surface its reason, not a bare 403.
const autoLinkFeatureErrorBody = `{"error":{"id":"feature_not_available","code":403,"status":"Forbidden",` +
	`"request":"83130baf-0313-9115-8f8e-dabe3a5516c4",` +
	`"reason":"This project's subscription plan does not include feature \"Auto Link Policy\". ` +
	`Please take a look at https://www.ory.com/pricing/ to select an appropriate plan and upgrade accordingly.",` +
	`"details":{"feature":"auto_link_policy"},"message":"The requested action was forbidden"}}`

// TestWrapAPIError_FeatureNotAvailable verifies that the plan-gated 403 for the
// OIDC auto-link policy is surfaced with the feature name, the human-readable
// reason, and the pricing link — so a user immediately understands it is a plan
// limitation rather than a mysterious permission error.
func TestWrapAPIError_FeatureNotAvailable(t *testing.T) {
	err := fmt.Errorf("403 Forbidden: %s", autoLinkFeatureErrorBody)

	wrapped := wrapAPIError(err, "patching project")
	require.Error(t, wrapped)

	msg := wrapped.Error()
	assert.Contains(t, msg, "patching project")
	assert.Contains(t, msg, "auto_link_policy", "should name the gated feature")
	assert.Contains(t, msg, "not available on current plan")
	assert.Contains(t, msg, "subscription plan does not include", "should surface the API reason")
	assert.Contains(t, msg, "ory.com/pricing", "should surface the pricing link from the reason")
	assert.Contains(t, msg, "83130baf", "should surface the request ID for Ory support")
}

// TestWrapAPIError_Forbidden covers a generic (non-feature) 403: it is enriched
// with the API reason and request ID and remains unwrappable for callers that
// inspect the underlying error.
func TestWrapAPIError_Forbidden(t *testing.T) {
	body := `{"error":{"code":403,"status":"Forbidden","request":"req-abc-123",` +
		`"reason":"insufficient permissions for this operation"}}`
	err := fmt.Errorf("403 Forbidden: %s", body)

	wrapped := wrapAPIError(err, "patching project")
	require.Error(t, wrapped)

	msg := wrapped.Error()
	assert.Contains(t, msg, "patching project")
	assert.Contains(t, msg, "forbidden (403)")
	assert.Contains(t, msg, "req-abc-123", "should surface the request ID for Ory support")
	assert.Contains(t, msg, "insufficient permissions", "should surface the API error reason")

	// The original error must remain unwrappable for callers that inspect it.
	assert.ErrorIs(t, wrapped, err)
}

// TestPatchProject_WrapsFeatureNotAvailable exercises the full client path: the
// plan-gated 403 from the console API must come back from PatchProject with the
// parsed feature/reason/request ID, not a bare "403 Forbidden". This guards
// against the regression where project_config updates surfaced an opaque 403
// (see issue #271).
func TestPatchProject_WrapsFeatureNotAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(autoLinkFeatureErrorBody))
	}))
	defer srv.Close()

	client, err := NewOryClient(OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   srv.URL,
	})
	require.NoError(t, err)

	_, patchErr := client.PatchProject(context.Background(), "prod-project", []ory.JsonPatch{{
		Op:    "replace",
		Path:  "/services/identity/config/selfservice/methods/oidc/enable_auto_link_policy",
		Value: true,
	}})
	require.Error(t, patchErr)

	msg := patchErr.Error()
	assert.Contains(t, msg, "auto_link_policy")
	assert.Contains(t, msg, "not available on current plan")
	assert.Contains(t, msg, "ory.com/pricing")
}

func TestNewOryClient_InvalidConsoleURL(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   "not-a-valid-url",
	}

	_, err := NewOryClient(cfg)
	require.Error(t, err, "expected error for invalid console URL")
	assert.Contains(t, err.Error(), "invalid console API URL")
}

func TestNewOryClient_InvalidProjectURL(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		ProjectAPIURL: "://invalid-url-template",
	}

	_, err := NewOryClient(cfg)
	require.Error(t, err, "expected error for invalid project URL")
	assert.Contains(t, err.Error(), "invalid project API URL")
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
			assert.Equal(t, tt.expected, isRateLimitError(tt.err))
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
	require.NoError(t, err)

	assert.Nil(t, client.projectClient, "project client should be nil without credentials")

	err = client.ensureProjectClient()
	require.Error(t, err, "expected error from ensureProjectClient without credentials")
	assert.Contains(t, err.Error(), "project_slug and project_api_key are required")
}

func TestOryClient_EnsureProjectClient_LazyInit(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	// Project client should already be initialized since credentials were provided at creation
	assert.NotNil(t, client.projectClient, "project client should be initialized when credentials provided at creation")

	// ensureProjectClient should succeed (client already initialized)
	assert.NoError(t, client.ensureProjectClient())
}

func TestOryClient_WithProjectCredentials(t *testing.T) {
	cfg := OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   DefaultConsoleAPIURL,
		// No project credentials initially
	}

	parent, err := NewOryClient(cfg)
	require.NoError(t, err)

	// Create a child client with project credentials
	child := parent.WithProjectCredentials(testutil.TestProjectSlug, testutil.TestProjectAPIKey)

	// Parent should still have no project credentials
	assert.Empty(t, parent.config.ProjectSlug, "parent config should not be modified")

	// Child should have the new credentials
	assert.Equal(t, testutil.TestProjectSlug, child.config.ProjectSlug)
	assert.Equal(t, testutil.TestProjectAPIKey, child.config.ProjectAPIKey)

	// Child should share the parent's console client
	assert.Same(t, parent.consoleClient, child.consoleClient, "child should share the parent's console client")

	// Child's project client should be lazily initialized
	assert.Nil(t, child.projectClient, "child project client should be nil before first use")

	// ensureProjectClient should initialize the child's project client
	require.NoError(t, child.ensureProjectClient())
	assert.NotNil(t, child.projectClient, "child project client should be initialized after ensureProjectClient")

	// Verify the child's project client URL
	servers := child.projectClient.GetConfig().Servers
	expectedURL := fmt.Sprintf(DefaultProjectAPIURL, testutil.TestProjectSlug)
	assert.Equal(t, expectedURL, servers[0].URL)
}

func TestOryClient_WithProjectCredentials_Isolation(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
	}

	parent, err := NewOryClient(cfg)
	require.NoError(t, err)

	// Create two children with different credentials
	child1 := parent.WithProjectCredentials("slug-one", "key-one")
	child2 := parent.WithProjectCredentials("slug-two", "key-two")

	// Initialize both
	require.NoError(t, child1.ensureProjectClient())
	require.NoError(t, child2.ensureProjectClient())

	// Verify each child has its own project client with correct URL
	servers1 := child1.projectClient.GetConfig().Servers
	expectedURL1 := fmt.Sprintf(DefaultProjectAPIURL, "slug-one")
	assert.Equal(t, expectedURL1, servers1[0].URL, "child1 project URL")

	servers2 := child2.projectClient.GetConfig().Servers
	expectedURL2 := fmt.Sprintf(DefaultProjectAPIURL, "slug-two")
	assert.Equal(t, expectedURL2, servers2[0].URL, "child2 project URL")

	// Parent should still have its original project client
	parentServers := parent.projectClient.GetConfig().Servers
	expectedParent := fmt.Sprintf(DefaultProjectAPIURL, testutil.TestProjectSlug)
	assert.Equal(t, expectedParent, parentServers[0].URL, "parent project URL")
}

func TestOryClient_EnsureProjectClient_MissingSlugOnly(t *testing.T) {
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		// ProjectSlug is empty
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	err = client.ensureProjectClient()
	require.Error(t, err, "expected error when slug is missing")
	assert.Contains(t, err.Error(), "project_slug and project_api_key are required")
}

func TestOryClient_EnsureProjectClient_MissingKeyOnly(t *testing.T) {
	cfg := OryClientConfig{
		ProjectSlug: testutil.TestProjectSlug,
		// ProjectAPIKey is empty
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	err = client.ensureProjectClient()
	require.Error(t, err, "expected error when API key is missing")
	assert.Contains(t, err.Error(), "project_slug and project_api_key are required")
}

func TestOryClient_RequireConsoleClient_NilReturnsError(t *testing.T) {
	// This test verifies the fix for https://github.com/ory/terraform-provider-ory/issues/137
	// where calling GetProject (and other console API methods) without a workspace
	// API key caused a nil pointer dereference panic instead of a helpful error.
	cfg := OryClientConfig{
		ProjectAPIKey: testutil.TestProjectAPIKey,
		ProjectSlug:   testutil.TestProjectSlug,
		// No WorkspaceAPIKey — consoleClient will be nil
	}

	client, err := NewOryClient(cfg)
	require.NoError(t, err)

	require.Nil(t, client.consoleClient, "consoleClient should be nil for this test")

	ctx := context.Background()

	// requireSentinel asserts the error wraps ErrConsoleClientNotConfigured.
	requireSentinel := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err, "expected error when consoleClient is nil")
		assert.ErrorIs(t, err, ErrConsoleClientNotConfigured)
	}

	// Project operations
	t.Run("GetProject", func(t *testing.T) {
		_, err := client.GetProject(ctx, "any-project-id")
		requireSentinel(t, err)
	})
	t.Run("PatchProject", func(t *testing.T) {
		_, err := client.PatchProject(ctx, "any-project-id", nil)
		requireSentinel(t, err)
	})
	t.Run("CreateProject", func(t *testing.T) {
		_, resp, err := client.CreateProject(ctx, "name", "prod", "")
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		requireSentinel(t, err)
	})
	t.Run("DeleteProject", func(t *testing.T) {
		requireSentinel(t, client.DeleteProject(ctx, "any-project-id"))
	})

	// Workspace operations
	t.Run("CreateWorkspace", func(t *testing.T) {
		_, err := client.CreateWorkspace(ctx, "name")
		requireSentinel(t, err)
	})
	t.Run("GetWorkspace", func(t *testing.T) {
		_, err := client.GetWorkspace(ctx, "any-workspace-id")
		requireSentinel(t, err)
	})
	t.Run("UpdateWorkspace", func(t *testing.T) {
		_, err := client.UpdateWorkspace(ctx, "any-workspace-id", "name")
		requireSentinel(t, err)
	})
	t.Run("ListWorkspaces", func(t *testing.T) {
		_, err := client.ListWorkspaces(ctx)
		requireSentinel(t, err)
	})

	// Project API key operations
	t.Run("CreateProjectAPIKey", func(t *testing.T) {
		_, err := client.CreateProjectAPIKey(ctx, "any-project-id", ory.CreateProjectApiKeyBody{Name: "test"})
		requireSentinel(t, err)
	})
	t.Run("ListProjectAPIKeys", func(t *testing.T) {
		_, err := client.ListProjectAPIKeys(ctx, "any-project-id")
		requireSentinel(t, err)
	})
	t.Run("DeleteProjectAPIKey", func(t *testing.T) {
		requireSentinel(t, client.DeleteProjectAPIKey(ctx, "any-project-id", "any-key-id"))
	})

	// Event stream operations
	t.Run("CreateEventStream", func(t *testing.T) {
		_, err := client.CreateEventStream(ctx, "any-project-id", ory.CreateEventStreamBody{})
		requireSentinel(t, err)
	})
	t.Run("GetEventStream", func(t *testing.T) {
		_, err := client.GetEventStream(ctx, "any-project-id", "any-stream-id")
		requireSentinel(t, err)
	})
	t.Run("SetEventStream", func(t *testing.T) {
		_, err := client.SetEventStream(ctx, "any-project-id", "any-stream-id", ory.SetEventStreamBody{})
		requireSentinel(t, err)
	})
	t.Run("DeleteEventStream", func(t *testing.T) {
		requireSentinel(t, client.DeleteEventStream(ctx, "any-project-id", "any-stream-id"))
	})
	t.Run("ListEventStreams", func(t *testing.T) {
		_, err := client.ListEventStreams(ctx, "any-project-id")
		requireSentinel(t, err)
	})

	// Organization operations
	t.Run("ListOrganizations", func(t *testing.T) {
		_, err := client.ListOrganizations(ctx, "any-project-id")
		requireSentinel(t, err)
	})
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
			assert.Equal(t, tt.expected, isRetryableError(tt.err))
		})
	}
}

func receiveWithTimeout(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		require.FailNow(t, "timed out waiting for User-Agent header")
		return ""
	}
}

func TestProviderUserAgent(t *testing.T) {
	expectedUserAgent := "Terraform/1.5.0 (+https://www.terraform.io) terraform-provider-ory/1.0.0"

	t.Run("ProjectAPI", func(t *testing.T) {
		uaChan := make(chan string, 1)
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uaChan <- r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		c, err := NewOryClient(OryClientConfig{
			ProjectAPIURL: mockServer.URL + "/?%s",
			ProjectSlug:   "dummy-project-slug",
			ProjectAPIKey: "dummy-project-api-key",
			UserAgent:     expectedUserAgent,
		})
		require.NoError(t, err, "Could not set up client")

		_, resp, err := c.ProjectAPI().ProjectAPI.GetProject(context.Background(), "dummy-project-id").Execute()
		if resp != nil && resp.Body != nil {
			err = errors.Join(resp.Body.Close(), err)
		}
		if err != nil {
			t.Logf("Expected error from mock response: %v", err)
		}

		got := receiveWithTimeout(t, uaChan)
		assert.Equal(t, expectedUserAgent, got)
	})

	t.Run("ConsoleAPI", func(t *testing.T) {
		uaChan := make(chan string, 1)
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uaChan <- r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		c, err := NewOryClient(OryClientConfig{
			ConsoleAPIURL:   mockServer.URL,
			WorkspaceAPIKey: "dummy-workspace-api-key",
			UserAgent:       expectedUserAgent,
		})
		require.NoError(t, err, "Could not set up client")

		_, resp, err := c.ConsoleAPI().ProjectAPI.GetProject(context.Background(), "dummy-project-id").Execute()
		if resp != nil && resp.Body != nil {
			err = errors.Join(resp.Body.Close(), err)
		}
		if err != nil {
			t.Logf("Expected error from mock response: %v", err)
		}

		got := receiveWithTimeout(t, uaChan)
		assert.Equal(t, expectedUserAgent, got)
	})

	t.Run("WithProjectCredentials_LazyInit", func(t *testing.T) {
		uaChan := make(chan string, 1)
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uaChan <- r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
		}))
		defer mockServer.Close()

		parent, err := NewOryClient(OryClientConfig{
			ConsoleAPIURL:   mockServer.URL,
			ProjectAPIURL:   mockServer.URL + "/?%s",
			WorkspaceAPIKey: "dummy-workspace-api-key",
			UserAgent:       expectedUserAgent,
		})
		require.NoError(t, err, "Could not set up client")

		child := parent.WithProjectCredentials("child-slug", "child-api-key")

		require.NoError(t, child.ensureProjectClient(), "ensureProjectClient failed")

		_, resp, err := child.ProjectAPI().ProjectAPI.GetProject(context.Background(), "dummy-project-id").Execute()
		if resp != nil && resp.Body != nil {
			err = errors.Join(resp.Body.Close(), err)
		}
		if err != nil {
			t.Logf("Expected error from mock response: %v", err)
		}

		got := receiveWithTimeout(t, uaChan)
		assert.Equal(t, expectedUserAgent, got)
	})
}

// TestPatchProject_SerialisedPerProject verifies the per-project mutex that
// guards PatchProject. Without it, concurrent patches against the same
// project can lose writes because the API merges patches against a snapshot
// of the project revision (see issue #213, where two parallel email-template
// deletes left one template behind). Patches against *different* projects
// must still run in parallel so multi-project Terraform runs aren't slowed
// to a serial trickle.
func TestPatchProject_SerialisedPerProject(t *testing.T) {
	t.Run("same project serialized", func(t *testing.T) {
		var inFlight, maxConcurrent int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"project":{"id":"p","slug":"s"}}`))
		}))
		defer srv.Close()

		client, err := NewOryClient(OryClientConfig{
			WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
			ConsoleAPIURL:   srv.URL,
		})
		require.NoError(t, err)

		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = client.PatchProject(context.Background(), "same-project", nil)
			}()
		}
		wg.Wait()

		assert.Equal(t, int32(1), atomic.LoadInt32(&maxConcurrent), "expected max 1 concurrent patch per project")
	})

	t.Run("different projects run in parallel", func(t *testing.T) {
		var inFlight, maxConcurrent int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, cur) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"project":{"id":"p","slug":"s"}}`))
		}))
		defer srv.Close()

		client, err := NewOryClient(OryClientConfig{
			WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
			ConsoleAPIURL:   srv.URL,
		})
		require.NoError(t, err)

		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = client.PatchProject(context.Background(), fmt.Sprintf("project-%d", i), nil)
			}()
		}
		wg.Wait()

		assert.GreaterOrEqual(t, atomic.LoadInt32(&maxConcurrent), int32(2), "expected concurrent patches across different projects")
	})
}

// TestIsNotFound covers the 404 detection used to treat a purged project as
// already gone (via parsed status code and via the bare status string), and that
// non-404 errors and nil are not misclassified.
func TestIsNotFound(t *testing.T) {
	assert.False(t, IsNotFound(nil))
	assert.True(t, IsNotFound(errors.New("404 Not Found")))
	assert.True(t, IsNotFound(errors.New(`{"error":{"code":404,"status":"Not Found","message":"project not found"}}`)))
	assert.False(t, IsNotFound(errors.New("400 Bad Request")))
	assert.False(t, IsNotFound(errors.New("500 Internal Server Error")))
}
