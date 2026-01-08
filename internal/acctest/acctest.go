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

	"github.com/ory/terraform-provider-orynetwork/internal/client"
	"github.com/ory/terraform-provider-orynetwork/internal/provider"
)

// TestProject holds information about a test project created for acceptance tests.
type TestProject struct {
	ID          string
	Slug        string
	Name        string
	Environment string
	APIKey      string
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
	os.Setenv("ORY_PROJECT_ID", project.ID)
	os.Setenv("ORY_PROJECT_SLUG", project.Slug)
	os.Setenv("ORY_PROJECT_API_KEY", project.APIKey)
	os.Setenv("ORY_PROJECT_ENVIRONMENT", project.Environment)
}

// GetTestProject returns the shared test project, loading from env vars or creating if necessary.
// This ensures all tests in a single test run share the same project.
//
// When ORY_TEST_PROJECT_PRECREATED=1 is set (by scripts/run-acceptance-tests.sh), the project
// details are loaded from environment variables. Otherwise, a new project is created.
// The project is created as a "prod" environment to support all features including organizations.
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

// initTestProject initializes the test project, either from env vars or by creating a new one.
func initTestProject(t *testing.T) {
	// Check if project was pre-created by the wrapper script
	if os.Getenv("ORY_TEST_PROJECT_PRECREATED") == "1" {
		loadProjectFromEnv(t)
		return
	}

	// Otherwise, create a new project (for running individual test packages)
	createSharedProject(t)

	// Register cleanup only when we created the project ourselves
	if sharedTestProject != nil {
		t.Cleanup(func() {
			cleanupTestProject(t)
		})
	}
}

// loadProjectFromEnv loads the test project from environment variables.
// This is used when the project was pre-created by scripts/run-acceptance-tests.sh.
func loadProjectFromEnv(t *testing.T) {
	projectID := os.Getenv("ORY_PROJECT_ID")
	projectSlug := os.Getenv("ORY_PROJECT_SLUG")
	projectAPIKey := os.Getenv("ORY_PROJECT_API_KEY")
	projectEnv := os.Getenv("ORY_PROJECT_ENVIRONMENT")

	if projectID == "" || projectSlug == "" || projectAPIKey == "" {
		initError = fmt.Errorf("ORY_TEST_PROJECT_PRECREATED=1 but missing required env vars: ORY_PROJECT_ID, ORY_PROJECT_SLUG, ORY_PROJECT_API_KEY")
		return
	}

	t.Logf("Using pre-created test project: %s (slug: %s, environment: %s)", projectID, projectSlug, projectEnv)

	sharedTestProject = &TestProject{
		ID:          projectID,
		Slug:        projectSlug,
		Name:        "pre-created",
		Environment: projectEnv,
		APIKey:      projectAPIKey,
	}
}

// createSharedProject creates the shared test project.
// This is called when running individual test packages without the wrapper script.
func createSharedProject(t *testing.T) {
	ctx := context.Background()
	c, err := getOryClient()
	if err != nil {
		initError = fmt.Errorf("failed to create Ory client: %w", err)
		return
	}

	projectName := fmt.Sprintf("tf-acc-test-%d", time.Now().UnixNano())
	t.Logf("Creating test project: %s (environment: prod)", projectName)

	// Create as "prod" environment to support all features including organizations
	project, err := c.CreateProject(ctx, projectName, "prod")
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

	// Configure project with keto namespaces for relationship tests
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
	}
	_, err = c.PatchProject(ctx, project.GetId(), patches)
	if err != nil {
		t.Logf("Warning: Failed to configure keto namespaces: %v (relationship tests may fail)", err)
	}

	sharedTestProject = &TestProject{
		ID:          project.GetId(),
		Slug:        project.GetSlug(),
		Name:        project.GetName(),
		Environment: project.GetEnvironment(),
		APIKey:      apiKey.GetValue(),
	}
}

// cleanupTestProject deletes the shared test project.
func cleanupTestProject(t *testing.T) {
	projectMutex.Lock()
	defer projectMutex.Unlock()

	if sharedTestProject == nil {
		return
	}

	ctx := context.Background()
	c, err := getOryClient()
	if err != nil {
		t.Logf("Warning: Failed to create Ory client for cleanup: %v", err)
		return
	}

	t.Logf("Cleaning up shared test project: %s", sharedTestProject.ID)
	if err := c.DeleteProject(ctx, sharedTestProject.ID); err != nil {
		t.Logf("Warning: Failed to delete test project: %v", err)
	} else {
		t.Logf("Successfully deleted shared test project: %s", sharedTestProject.ID)
	}

	sharedTestProject = nil
}

// getOryClient returns a shared Ory client for test setup/teardown.
func getOryClient() (*client.OryClient, error) {
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

// GetOryClient returns a shared Ory client for use in tests.
// This is an exported version for use in custom CheckDestroy functions.
func GetOryClient() (*client.OryClient, error) {
	return getOryClient()
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

// RunTest runs an acceptance test.
// This is a convenience wrapper around resource.Test() that follows
// provider conventions and can be extended in the future.
func RunTest(t *testing.T, tc resource.TestCase) {
	t.Helper()
	resource.Test(t, tc)
}
