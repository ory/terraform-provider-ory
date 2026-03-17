// Package acctest provides shared acceptance test utilities.
package acctest

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/provider"
)

// TestProjectPrefix is used to name ephemeral test projects so that stale
// ones can be automatically purged. DO NOT CHANGE.
const TestProjectPrefix = "ory-cy-e2e-da2f162d-af61-42dd-90dc-e3fcfa7c84a0"

// TestProject holds information about a test project created for acceptance tests.
type TestProject struct {
	ID          string
	Slug        string
	Name        string
	Environment string
	APIKey      string // #nosec G117 -- test-only struct field, not a credential
}

var (
	// sharedTestProject is the singleton test project used by all acceptance tests.
	sharedTestProject *TestProject
	// projectMutex protects access to sharedTestProject.
	projectMutex sync.Mutex
	// projectOnce ensures the project is only loaded/created once per process.
	projectOnce sync.Once
	// oryClient is the shared client used for test setup/teardown.
	oryClient *client.OryClient
	// initError stores any error from project initialization.
	initError error
)

// TestAccProtoV6ProviderFactories returns the provider factories for acceptance tests.
func TestAccProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"ory": providerserver.NewProtocol6WithError(provider.New("test")()),
	}
}

// AccPreCheck performs common pre-check validations for acceptance tests.
// It ensures required environment variables are set and initializes the test project.
func AccPreCheck(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}

	if os.Getenv("ORY_WORKSPACE_API_KEY") == "" {
		t.Skip("ORY_WORKSPACE_API_KEY must be set for acceptance tests")
	}

	if os.Getenv("ORY_WORKSPACE_ID") == "" {
		t.Skip("ORY_WORKSPACE_ID must be set for acceptance tests")
	}

	// Ensure we have a test project
	project := GetTestProject(t)
	if project == nil {
		t.Fatal("Failed to get or create test project")
	}

	// Set environment variables for the provider to use
	_ = os.Setenv("ORY_PROJECT_ID", project.ID)
	_ = os.Setenv("ORY_PROJECT_SLUG", project.Slug)
	_ = os.Setenv("ORY_PROJECT_API_KEY", project.APIKey)
	_ = os.Setenv("ORY_PROJECT_ENVIRONMENT", project.Environment)
}

// GetTestProject returns the shared test project, loading from env vars or creating if necessary.
// This ensures all tests in a single test run share the same project.
//
// ORY_PROJECT_ID, ORY_PROJECT_SLUG, and ORY_PROJECT_API_KEY should be set to use a
// pre-created project (the standard path for both CI and local development).
// If not set, a fallback ephemeral project is created and cleaned up when tests finish.
func GetTestProject(t *testing.T) *TestProject {
	t.Helper()

	// Use sync.Once to ensure project is only loaded/created once per process
	projectOnce.Do(func() {
		initTestProject(t)
	})

	if initError != nil {
		t.Fatalf("Test project initialization failed: %v", initError)
		return nil
	}

	return sharedTestProject
}

// GetTestProjectID returns the project ID of the shared test project.
func GetTestProjectID(t *testing.T) string {
	t.Helper()
	return GetTestProject(t).ID
}

// initTestProject initializes the test project, either from env vars or by creating a new one.
func initTestProject(t *testing.T) {
	// If project credentials are provided, use the pre-created project
	if os.Getenv("ORY_PROJECT_ID") != "" && os.Getenv("ORY_PROJECT_SLUG") != "" && os.Getenv("ORY_PROJECT_API_KEY") != "" {
		loadProjectFromEnv(t)
		return
	}

	// Fallback: create an ephemeral project if no pre-created project is configured
	createSharedProject(t)
}

// loadProjectFromEnv loads the test project from environment variables.
// This is the standard path for both CI and local development.
func loadProjectFromEnv(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	projectSlug := os.Getenv("ORY_PROJECT_SLUG")
	projectAPIKey := os.Getenv("ORY_PROJECT_API_KEY")
	projectEnv := os.Getenv("ORY_PROJECT_ENVIRONMENT")

	t.Logf("Using pre-created test project: %s (slug: %s, environment: %s)", projectID, projectSlug, projectEnv)

	sharedTestProject = &TestProject{
		ID:          projectID,
		Slug:        projectSlug,
		Name:        "pre-created",
		Environment: projectEnv,
		APIKey:      projectAPIKey,
	}
}

// createSharedProject creates an ephemeral test project as a fallback when no
// pre-created project is configured. The project uses the TestProjectPrefix so
// stale projects can be automatically purged. Cleanup is best-effort via
// CleanupEphemeralProject().
func createSharedProject(t *testing.T) {
	ctx := context.Background()
	c, err := GetOryClient()
	if err != nil {
		initError = fmt.Errorf("failed to create Ory client: %w", err)
		return
	}

	projectName := fmt.Sprintf("%s-tf-%d", TestProjectPrefix, time.Now().UnixNano())
	t.Logf("Creating test project: %s (environment: prod)", projectName)

	// Create as "prod" environment to support all features including organizations
	// Empty home_region uses the default (eu-central)
	project, _, err := c.CreateProject(ctx, projectName, "prod", "")
	if err != nil {
		initError = fmt.Errorf("failed to create test project: %w", err)
		return
	}

	t.Logf("Created test project: %s (slug: %s, environment: %s)", project.GetId(), project.GetSlug(), project.GetEnvironment())

	// Create an API key for the project
	apiKeyReq := ory.CreateProjectApiKeyRequest{
		Name: "tf-acc-test-key",
	}
	apiKey, err := c.CreateProjectAPIKey(ctx, project.GetId(), apiKeyReq)
	if err != nil {
		// Clean up the project if API key creation fails
		_ = c.DeleteProject(ctx, project.GetId())
		initError = fmt.Errorf("failed to create project API key: %w", err)
		return
	}

	// Configure project with keto namespaces and enable dynamic client registration
	patches := []ory.JsonPatch{
		{
			Op:   "add",
			Path: "/services/permission/config/namespaces",
			Value: []map[string]interface{}{
				{"name": "documents", "id": 1},
				{"name": "folders", "id": 2},
				{"name": "groups", "id": 3},
				{"name": "users", "id": 4},
			},
		},
		{
			Op:    "replace",
			Path:  "/services/oauth2/config/oidc/dynamic_client_registration/enabled",
			Value: true,
		},
	}
	_, err = c.PatchProject(ctx, project.GetId(), patches)
	if err != nil {
		t.Logf("Warning: Failed to configure project: %v (some tests may fail)", err)
	}

	sharedTestProject = &TestProject{
		ID:          project.GetId(),
		Slug:        project.GetSlug(),
		Name:        project.GetName(),
		Environment: project.GetEnvironment(),
		APIKey:      apiKey.GetValue(),
	}
}

// CleanupEphemeralProject deletes the shared test project if it was created
// ephemerally (not loaded from env vars). This is safe to call multiple times.
// In practice, stale projects are automatically purged, so failing
// to call this is not catastrophic.
func CleanupEphemeralProject(t *testing.T) {
	projectMutex.Lock()
	defer projectMutex.Unlock()

	if sharedTestProject == nil {
		return
	}

	// Never delete pre-created projects
	if sharedTestProject.Name == "pre-created" {
		return
	}

	ctx := context.Background()
	c, err := GetOryClient()
	if err != nil {
		t.Logf("Warning: Failed to create Ory client for cleanup: %v", err)
		return
	}

	t.Logf("Cleaning up ephemeral test project: %s", sharedTestProject.ID)
	if err := c.DeleteProject(ctx, sharedTestProject.ID); err != nil {
		t.Logf("Warning: Failed to delete test project: %v", err)
	} else {
		t.Logf("Successfully deleted ephemeral test project: %s", sharedTestProject.ID)
	}

	sharedTestProject = nil
}

// GetOryClient returns a shared Ory client for test setup/teardown.
func GetOryClient() (*client.OryClient, error) {
	if oryClient != nil {
		return oryClient, nil
	}

	consoleURL := os.Getenv("ORY_CONSOLE_API_URL")
	if consoleURL == "" {
		consoleURL = "https://api.console.ory.sh"
	}

	projectURL := os.Getenv("ORY_PROJECT_API_URL")
	if projectURL == "" {
		projectURL = "https://%s.projects.oryapis.com"
	}

	cfg := client.OryClientConfig{
		WorkspaceAPIKey: os.Getenv("ORY_WORKSPACE_API_KEY"),
		WorkspaceID:     os.Getenv("ORY_WORKSPACE_ID"),
		ConsoleAPIURL:   consoleURL,
		ProjectAPIURL:   projectURL,
	}

	// Also set project credentials if available
	if sharedTestProject != nil {
		cfg.ProjectAPIKey = sharedTestProject.APIKey
		cfg.ProjectSlug = sharedTestProject.Slug
		cfg.ProjectID = sharedTestProject.ID
	} else {
		// Fall back to environment variables
		cfg.ProjectAPIKey = os.Getenv("ORY_PROJECT_API_KEY")
		cfg.ProjectSlug = os.Getenv("ORY_PROJECT_SLUG")
		cfg.ProjectID = os.Getenv("ORY_PROJECT_ID")
	}

	var err error
	oryClient, err = client.NewOryClient(cfg)
	return oryClient, err
}

// SkipIfFeatureDisabled skips the test if the specified feature flag is not set to "true".
func SkipIfFeatureDisabled(t *testing.T, envVar, featureName string) {
	t.Helper()
	if os.Getenv(envVar) != "true" {
		t.Skipf("%s must be 'true' to run %s tests", envVar, featureName)
	}
}

// RequireKetoTests skips the test if ORY_KETO_TESTS_ENABLED is not "true".
func RequireKetoTests(t *testing.T) {
	t.Helper()
	SkipIfFeatureDisabled(t, "ORY_KETO_TESTS_ENABLED", "relationship/keto")
}

// RequireB2BTests skips the test if ORY_B2B_ENABLED is not "true".
func RequireB2BTests(t *testing.T) {
	t.Helper()
	SkipIfFeatureDisabled(t, "ORY_B2B_ENABLED", "B2B/organization")
}

// RequireSchemaTests skips the test if ORY_SCHEMA_TESTS_ENABLED is not "true".
func RequireSchemaTests(t *testing.T) {
	t.Helper()
	SkipIfFeatureDisabled(t, "ORY_SCHEMA_TESTS_ENABLED", "identity schema")
}

// RequireSocialProviderTests skips the test if ORY_SOCIAL_PROVIDER_TESTS_ENABLED is not "true".
func RequireSocialProviderTests(t *testing.T) {
	t.Helper()
	SkipIfFeatureDisabled(t, "ORY_SOCIAL_PROVIDER_TESTS_ENABLED", "social provider")
}

// RequireProjectTests skips the test if ORY_PROJECT_TESTS_ENABLED is not "true".
func RequireProjectTests(t *testing.T) {
	t.Helper()
	SkipIfFeatureDisabled(t, "ORY_PROJECT_TESTS_ENABLED", "project")
}

// RequireEventStreamTests skips the test if ORY_EVENT_STREAM_TESTS_ENABLED is not "true".
func RequireEventStreamTests(t *testing.T) {
	t.Helper()
	SkipIfFeatureDisabled(t, "ORY_EVENT_STREAM_TESTS_ENABLED", "event stream")
}

// RunTest runs an acceptance test.
// This is a convenience wrapper around resource.Test() that follows
// provider conventions and can be extended in the future.
func RunTest(t *testing.T, tc resource.TestCase) {
	t.Helper()
	resource.Test(t, tc)
}
