package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ory "github.com/ory/client-go"
	"github.com/ory/x/urlx"
)

const (
	// maxRetries is the maximum number of retry attempts for rate-limited requests.
	maxRetries = 3
	// initialBackoff is the initial backoff duration before first retry.
	initialBackoff = 1 * time.Second
)

const (
	// DefaultConsoleAPIURL is the default Ory Console API URL.
	DefaultConsoleAPIURL = "https://api.console.ory.sh"
	// DefaultProjectAPIURL is the default Ory Project API URL template.
	// The %s placeholder is replaced with the project slug.
	DefaultProjectAPIURL = "https://%s.projects.oryapis.com"
)

// oryAPIError represents the error structure returned by Ory APIs.
type oryAPIError struct {
	Error struct {
		ID      string `json:"id"`
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Request string `json:"request"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
		Details struct {
			Feature string `json:"feature"`
		} `json:"details"`
	} `json:"error"`
}

// OryErrorDebugInfo contains comprehensive debug information for API errors.
type OryErrorDebugInfo struct {
	StatusCode   int
	ErrorID      string
	ErrorMessage string
	ErrorReason  string
	RequestID    string
	Feature      string
	RawBody      string
	ErrorType    string
}

// String formats the debug info for logging.
func (d OryErrorDebugInfo) String() string {
	return fmt.Sprintf(`
======================================================================
ORY API ERROR DEBUG INFO
======================================================================
Error Type: %s
Status Code: %d
Error ID: %s
Error Message: %s
Error Reason: %s
Request ID: %s
Feature: %s
----------------------------------------------------------------------
Raw Response Body:
%s
----------------------------------------------------------------------
NOTE: Provide the Request ID to Ory support for debugging.
======================================================================`,
		d.ErrorType, d.StatusCode, d.ErrorID, d.ErrorMessage,
		d.ErrorReason, d.RequestID, d.Feature, d.RawBody)
}

// extractDebugInfo extracts comprehensive debug information from an error.
func extractDebugInfo(err error) OryErrorDebugInfo {
	info := OryErrorDebugInfo{
		ErrorType: fmt.Sprintf("%T", err),
	}

	if err == nil {
		return info
	}

	// Try to extract the error body from GenericOpenAPIError
	var bodyStr string
	var apiErr *ory.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		bodyStr = string(apiErr.Body())
		info.RawBody = bodyStr
	} else {
		// Try to find JSON in the error string
		errStr := err.Error()
		info.RawBody = errStr
		if idx := strings.Index(errStr, "{"); idx >= 0 {
			bodyStr = errStr[idx:]
		}
	}

	if bodyStr == "" {
		return info
	}

	var parsed oryAPIError
	if jsonErr := json.Unmarshal([]byte(bodyStr), &parsed); jsonErr == nil {
		info.StatusCode = parsed.Error.Code
		info.ErrorID = parsed.Error.ID
		info.ErrorMessage = parsed.Error.Message
		info.ErrorReason = parsed.Error.Reason
		info.RequestID = parsed.Error.Request
		info.Feature = parsed.Error.Details.Feature
	}

	return info
}

// parseFeatureError attempts to extract feature availability info from an error.
// Returns the feature name and reason if this is a feature_not_available error.
func parseFeatureError(err error) (feature string, reason string, requestID string, ok bool) {
	if err == nil {
		return "", "", "", false
	}

	// Try to extract the error body from GenericOpenAPIError
	var bodyStr string
	var apiErr *ory.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		bodyStr = string(apiErr.Body())
	} else {
		// Try to find JSON in the error string
		errStr := err.Error()
		if idx := strings.Index(errStr, "{"); idx >= 0 {
			bodyStr = errStr[idx:]
		}
	}

	if bodyStr == "" {
		return "", "", "", false
	}

	var parsed oryAPIError
	if jsonErr := json.Unmarshal([]byte(bodyStr), &parsed); jsonErr != nil {
		return "", "", "", false
	}

	if parsed.Error.ID == "feature_not_available" {
		return parsed.Error.Details.Feature, parsed.Error.Reason, parsed.Error.Request, true
	}

	return "", "", parsed.Error.Request, false
}

// wrapAPIError enhances API errors with more helpful context.
// It extracts debug information and provides actionable error messages.
func wrapAPIError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Extract debug info for all errors
	debugInfo := extractDebugInfo(err)

	// Check for feature availability errors first
	if feature, reason, requestID, ok := parseFeatureError(err); ok {
		return fmt.Errorf("%s: feature '%s' not available on current plan.\n"+
			"Reason: %s\n"+
			"Request ID: %s (provide this to Ory support)\n"+
			"\nDebug Info:%s",
			operation, feature, reason, requestID, debugInfo.String())
	}

	errStr := err.Error()

	// EOF errors typically mean the API closed the connection without a response.
	// This often happens when a feature requires an enterprise plan.
	if errors.Is(err, io.EOF) || strings.Contains(errStr, "EOF") || strings.Contains(errStr, "error reading from server") {
		return fmt.Errorf("%s: connection closed by server (EOF). This may indicate:\n"+
			"  - The feature requires an Ory Network enterprise plan (e.g., B2B Organizations)\n"+
			"  - Invalid or expired API credentials\n"+
			"  - Network connectivity issues\n"+
			"Request ID: %s\n"+
			"Original error: %w",
			operation, debugInfo.RequestID, err)
	}

	// Check for common HTTP error patterns
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "Unauthorized") {
		return fmt.Errorf("%s: unauthorized (401).\n"+
			"Check that your API key is valid and has the required permissions.\n"+
			"Request ID: %s\n"+
			"Original error: %w",
			operation, debugInfo.RequestID, err)
	}

	if strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden") {
		return fmt.Errorf("%s: forbidden (403).\n"+
			"Your API key may not have permission for this operation, or the feature may require an enterprise plan.\n"+
			"Request ID: %s\n"+
			"Error Reason: %s\n"+
			"Original error: %w",
			operation, debugInfo.RequestID, debugInfo.ErrorReason, err)
	}

	if strings.Contains(errStr, "404") || strings.Contains(errStr, "Not Found") {
		return fmt.Errorf("%s: resource not found (404).\n"+
			"Verify the resource ID and project configuration.\n"+
			"Request ID: %s\n"+
			"Original error: %w",
			operation, debugInfo.RequestID, err)
	}

	// For any other error, include the request ID if available
	if debugInfo.RequestID != "" {
		return fmt.Errorf("%s: %w (Request ID: %s)", operation, err, debugInfo.RequestID)
	}

	return err
}

// isRateLimitError checks if the error is a rate limit (429) error.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") || strings.Contains(errStr, "Too Many Requests")
}

// isRetryableError checks if the error is a server error (5xx) that should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "Internal Server Error") ||
		strings.Contains(errStr, "Bad Gateway") ||
		strings.Contains(errStr, "Service Unavailable") ||
		strings.Contains(errStr, "Gateway Timeout")
}

// retryWithBackoff executes a function with exponential backoff on rate limit errors.
func retryWithBackoff[T any](ctx context.Context, operation string, fn func() (T, error)) (T, error) {
	var result T
	var err error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		if !isRateLimitError(err) {
			return result, err
		}

		if attempt == maxRetries {
			return result, fmt.Errorf("%s: rate limit exceeded after %d retries: %w", operation, maxRetries, err)
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2 // Exponential backoff
		}
	}

	return result, err
}

// OryClientConfig holds configuration for the Ory API client.
type OryClientConfig struct {
	WorkspaceAPIKey string
	ProjectAPIKey   string
	ProjectID       string
	ProjectSlug     string
	WorkspaceID     string
	ConsoleAPIURL   string
	ProjectAPIURL   string // URL template with %s placeholder for slug (e.g., "https://%s.projects.oryapis.com")
}

// OryClient wraps the Ory SDK clients.
// Ory Network uses two different APIs:
// 1. Console API (api.console.ory.sh) - for projects, workspaces, organizations
// 2. Project API ({slug}.projects.oryapis.com) - for identities, OAuth2 clients
type OryClient struct {
	config OryClientConfig

	// Console API client (for organizations, projects, workspaces)
	consoleClient *ory.APIClient

	// Project API client (for identities, OAuth2)
	projectClient *ory.APIClient
}

// NewOryClient creates a new Ory API client.
func NewOryClient(cfg OryClientConfig) (*OryClient, error) {
	client := &OryClient{config: cfg}

	// Initialize console client if workspace API key is provided
	if cfg.WorkspaceAPIKey != "" {
		// Validate the console API URL
		parsedURL, err := urlx.Parse(cfg.ConsoleAPIURL)
		if err != nil {
			return nil, fmt.Errorf("invalid console API URL %q: %w", cfg.ConsoleAPIURL, err)
		}
		if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
			return nil, fmt.Errorf("invalid console API URL %q: must use http or https scheme", cfg.ConsoleAPIURL)
		}

		consoleCfg := ory.NewConfiguration()
		consoleCfg.Servers = ory.ServerConfigurations{
			{URL: cfg.ConsoleAPIURL},
		}
		consoleCfg.AddDefaultHeader("Authorization", "Bearer "+cfg.WorkspaceAPIKey)

		// CRITICAL: The SDK has hardcoded operation-specific server URLs for all
		// console API operations (ProjectAPI, WorkspaceAPI, EventsAPI). These override
		// the main Servers config. We must override each operation to use our custom URL.
		consoleServer := ory.ServerConfigurations{{URL: cfg.ConsoleAPIURL}}
		consoleCfg.OperationServers = map[string]ory.ServerConfigurations{
			// ProjectAPI operations
			"ProjectAPIService.CreateProject":                          consoleServer,
			"ProjectAPIService.GetProject":                             consoleServer,
			"ProjectAPIService.ListProjects":                           consoleServer,
			"ProjectAPIService.PatchProject":                           consoleServer,
			"ProjectAPIService.PatchProjectWithRevision":               consoleServer,
			"ProjectAPIService.PurgeProject":                           consoleServer,
			"ProjectAPIService.SetProject":                             consoleServer,
			"ProjectAPIService.GetProjectMembers":                      consoleServer,
			"ProjectAPIService.RemoveProjectMember":                    consoleServer,
			"ProjectAPIService.CreateProjectApiKey":                    consoleServer,
			"ProjectAPIService.DeleteProjectApiKey":                    consoleServer,
			"ProjectAPIService.ListProjectApiKeys":                     consoleServer,
			"ProjectAPIService.CreateOrganization":                     consoleServer,
			"ProjectAPIService.DeleteOrganization":                     consoleServer,
			"ProjectAPIService.GetOrganization":                        consoleServer,
			"ProjectAPIService.ListOrganizations":                      consoleServer,
			"ProjectAPIService.UpdateOrganization":                     consoleServer,
			"ProjectAPIService.CreateOrganizationOnboardingPortalLink": consoleServer,
			"ProjectAPIService.DeleteOrganizationOnboardingPortalLink": consoleServer,
			"ProjectAPIService.GetOrganizationOnboardingPortalLinks":   consoleServer,
			"ProjectAPIService.UpdateOrganizationOnboardingPortalLink": consoleServer,
			// WorkspaceAPI operations
			"WorkspaceAPIService.CreateWorkspace":       consoleServer,
			"WorkspaceAPIService.GetWorkspace":          consoleServer,
			"WorkspaceAPIService.ListWorkspaces":        consoleServer,
			"WorkspaceAPIService.UpdateWorkspace":       consoleServer,
			"WorkspaceAPIService.CreateWorkspaceApiKey": consoleServer,
			"WorkspaceAPIService.DeleteWorkspaceApiKey": consoleServer,
			"WorkspaceAPIService.ListWorkspaceApiKeys":  consoleServer,
			"WorkspaceAPIService.ListWorkspaceProjects": consoleServer,
			// EventsAPI operations
			"EventsAPIService.CreateEventStream": consoleServer,
			"EventsAPIService.DeleteEventStream": consoleServer,
			"EventsAPIService.ListEventStreams":  consoleServer,
			"EventsAPIService.SetEventStream":    consoleServer,
		}

		client.consoleClient = ory.NewAPIClient(consoleCfg)
	}

	// Initialize project client if project API key and slug are provided
	if cfg.ProjectAPIKey != "" && cfg.ProjectSlug != "" {
		projectCfg := ory.NewConfiguration()
		// Use configurable URL template, defaulting to production
		projectAPIURL := cfg.ProjectAPIURL
		if projectAPIURL == "" {
			projectAPIURL = DefaultProjectAPIURL
		}
		// Format the URL template with the project slug and validate it
		formattedURL := fmt.Sprintf(projectAPIURL, cfg.ProjectSlug)
		parsedURL, err := urlx.Parse(formattedURL)
		if err != nil {
			return nil, fmt.Errorf("invalid project API URL %q: %w", formattedURL, err)
		}
		if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
			return nil, fmt.Errorf("invalid project API URL %q: must use http or https scheme", formattedURL)
		}
		projectCfg.Servers = ory.ServerConfigurations{
			{URL: formattedURL},
		}
		projectCfg.AddDefaultHeader("Authorization", "Bearer "+cfg.ProjectAPIKey)
		client.projectClient = ory.NewAPIClient(projectCfg)
	}

	return client, nil
}

// ConsoleAPI returns the console API client.
func (c *OryClient) ConsoleAPI() *ory.APIClient {
	return c.consoleClient
}

// ProjectAPI returns the project API client.
func (c *OryClient) ProjectAPI() *ory.APIClient {
	return c.projectClient
}

// Config returns the client configuration.
func (c *OryClient) Config() OryClientConfig {
	return c.config
}

// ProjectID returns the configured project ID.
func (c *OryClient) ProjectID() string {
	return c.config.ProjectID
}

// WorkspaceID returns the configured workspace ID.
func (c *OryClient) WorkspaceID() string {
	return c.config.WorkspaceID
}

// =============================================================================
// Project Operations (Console API)
// =============================================================================

// CreateProject creates a new Ory project.
// Returns the project, HTTP response (for status code inspection), and any error.
func (c *OryClient) CreateProject(ctx context.Context, name, environment, homeRegion string) (*ory.Project, *http.Response, error) {
	body := ory.CreateProjectBody{
		Name:        name,
		Environment: environment,
	}
	if c.config.WorkspaceID != "" {
		body.WorkspaceId = ory.PtrString(c.config.WorkspaceID)
	}
	if homeRegion != "" {
		body.HomeRegion = &homeRegion
	}

	project, httpResp, err := c.consoleClient.ProjectAPI.CreateProject(ctx).CreateProjectBody(body).Execute()
	return project, httpResp, err
}

// GetProject retrieves a project by ID.
func (c *OryClient) GetProject(ctx context.Context, projectID string) (*ory.Project, error) {
	project, httpResp, err := c.consoleClient.ProjectAPI.GetProject(ctx, projectID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return project, err
}

// DeleteProject purges a project.
func (c *OryClient) DeleteProject(ctx context.Context, projectID string) error {
	httpResp, err := c.consoleClient.ProjectAPI.PurgeProject(ctx, projectID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return err
}

// PatchProject applies JSON Patch operations to a project.
func (c *OryClient) PatchProject(ctx context.Context, projectID string, patches []ory.JsonPatch) (*ory.SuccessfulProjectUpdate, error) {
	result, httpResp, err := c.consoleClient.ProjectAPI.PatchProject(ctx, projectID).
		JsonPatch(patches).
		Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return result, err
}

// =============================================================================
// Workspace Operations (Console API)
// =============================================================================

// CreateWorkspace creates a new workspace.
func (c *OryClient) CreateWorkspace(ctx context.Context, name string) (*ory.Workspace, error) {
	body := ory.CreateWorkspaceBody{
		Name: name,
	}
	workspace, httpResp, err := c.consoleClient.WorkspaceAPI.CreateWorkspace(ctx).CreateWorkspaceBody(body).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return workspace, err
}

// GetWorkspace retrieves a workspace by ID.
// Note: Due to Ory API permission limitations, some API keys can list workspaces
// but not get a specific workspace. We fall back to listing and filtering if
// the direct GET fails with 403.
func (c *OryClient) GetWorkspace(ctx context.Context, workspaceID string) (*ory.Workspace, error) {
	workspace, httpResp, err := c.consoleClient.WorkspaceAPI.GetWorkspace(ctx, workspaceID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	if err != nil {
		// Check if it's a 403 error - try fallback to list
		errStr := err.Error()
		if strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden") {
			// Fall back to listing workspaces and finding by ID
			listResp, listHttpResp, listErr := c.consoleClient.WorkspaceAPI.ListWorkspaces(ctx).Execute()
			if listHttpResp != nil {
				_ = listHttpResp.Body.Close()
			}
			if listErr != nil {
				return nil, err // Return original error
			}
			for _, w := range listResp.Workspaces {
				if w.GetId() == workspaceID {
					return &w, nil
				}
			}
			return nil, fmt.Errorf("workspace %s not found", workspaceID)
		}
		return nil, err
	}
	return workspace, nil
}

// UpdateWorkspace updates a workspace.
func (c *OryClient) UpdateWorkspace(ctx context.Context, workspaceID, name string) (*ory.Workspace, error) {
	body := ory.UpdateWorkspaceBody{
		Name: name,
	}
	workspace, httpResp, err := c.consoleClient.WorkspaceAPI.UpdateWorkspace(ctx, workspaceID).UpdateWorkspaceBody(body).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return workspace, err
}

// GetProjectEnvironment retrieves the environment (prod, stage, dev) for a project.
func (c *OryClient) GetProjectEnvironment(ctx context.Context, projectID string) (string, error) {
	if c.consoleClient == nil {
		return "", fmt.Errorf("console API client not configured")
	}
	project, httpResp, err := c.consoleClient.ProjectAPI.GetProject(ctx, projectID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	if err != nil {
		return "", err
	}
	return project.GetEnvironment(), nil
}

// =============================================================================
// Organization Operations (Console API with workspace key)
// Organizations require B2B features and a prod/stage project environment.
// =============================================================================

// CreateOrganization creates a new organization.
// Note: Organizations require:
// - An Ory Network plan with B2B features
// - Project environment set to "prod" or "stage" (NOT "dev")
func (c *OryClient) CreateOrganization(ctx context.Context, projectID, label string, domains []string) (*ory.Organization, error) {
	if c.consoleClient == nil {
		return nil, fmt.Errorf("creating organization: console API client not configured. " +
			"Organizations require workspace_api_key (ORY_WORKSPACE_API_KEY) to be set")
	}

	// Ory API requires domains to be an array, not null
	if domains == nil {
		domains = []string{}
	}

	body := ory.OrganizationBody{
		Label:   ory.PtrString(label),
		Domains: domains,
	}

	org, httpResp, err := c.consoleClient.ProjectAPI.CreateOrganization(ctx, projectID).OrganizationBody(body).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	if err != nil {
		return nil, wrapAPIError(err, "creating organization")
	}
	return org, nil
}

// GetOrganization retrieves an organization by ID.
// Includes retry logic to handle eventual consistency after organization creation.
func (c *OryClient) GetOrganization(ctx context.Context, projectID, orgID string) (*ory.Organization, error) {
	if c.consoleClient == nil {
		return nil, fmt.Errorf("reading organization: console API client not configured. " +
			"Set workspace_api_key (ORY_WORKSPACE_API_KEY)")
	}

	// Retry with backoff for 404 errors (eventual consistency)
	// Use 5 attempts with delays: 1s, 2s, 4s, 8s to handle slow propagation
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		resp, httpResp, err := c.consoleClient.ProjectAPI.GetOrganization(ctx, projectID, orgID).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		if err == nil {
			return &resp.Organization, nil
		}

		lastErr = err
		errStr := err.Error()

		// Only retry on 404 errors (eventual consistency)
		if !strings.Contains(errStr, "404") && !strings.Contains(errStr, "Not Found") {
			return nil, wrapAPIError(err, "reading organization")
		}

		// Wait before retry (exponential backoff: 1s, 2s, 4s, 8s)
		if attempt < 4 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}

	return nil, wrapAPIError(lastErr, "reading organization")
}

// UpdateOrganization updates an organization.
func (c *OryClient) UpdateOrganization(ctx context.Context, projectID, orgID, label string, domains []string) (*ory.Organization, error) {
	if c.consoleClient == nil {
		return nil, fmt.Errorf("updating organization: console API client not configured. " +
			"Set workspace_api_key (ORY_WORKSPACE_API_KEY)")
	}

	// Ory API requires domains to be an array, not null
	if domains == nil {
		domains = []string{}
	}

	body := ory.OrganizationBody{
		Label:   ory.PtrString(label),
		Domains: domains,
	}

	// Retry with backoff for 404 errors (eventual consistency)
	// Use 5 attempts with delays: 1s, 2s, 4s, 8s to handle slow propagation
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		org, httpResp, err := c.consoleClient.ProjectAPI.UpdateOrganization(ctx, projectID, orgID).OrganizationBody(body).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		if err == nil {
			return org, nil
		}

		lastErr = err
		errStr := err.Error()

		// Only retry on 404 errors (eventual consistency)
		if !strings.Contains(errStr, "404") && !strings.Contains(errStr, "Not Found") {
			return nil, wrapAPIError(err, "updating organization")
		}

		// Wait before retry (exponential backoff: 1s, 2s, 4s, 8s)
		if attempt < 4 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}

	return nil, wrapAPIError(lastErr, "updating organization")
}

// DeleteOrganization deletes an organization.
func (c *OryClient) DeleteOrganization(ctx context.Context, projectID, orgID string) error {
	if c.consoleClient == nil {
		return fmt.Errorf("deleting organization: console API client not configured. " +
			"Set workspace_api_key (ORY_WORKSPACE_API_KEY)")
	}
	httpResp, err := c.consoleClient.ProjectAPI.DeleteOrganization(ctx, projectID, orgID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return wrapAPIError(err, "deleting organization")
}

// =============================================================================
// Identity Operations (Project API)
// =============================================================================

// CreateIdentity creates a new identity with retry on rate limit.
func (c *OryClient) CreateIdentity(ctx context.Context, body ory.CreateIdentityBody) (*ory.Identity, error) {
	return retryWithBackoff(ctx, "creating identity", func() (*ory.Identity, error) {
		identity, httpResp, err := c.projectClient.IdentityAPI.CreateIdentity(ctx).CreateIdentityBody(body).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		return identity, err
	})
}

// GetIdentity retrieves an identity by ID with retry on rate limit.
func (c *OryClient) GetIdentity(ctx context.Context, identityID string) (*ory.Identity, error) {
	return retryWithBackoff(ctx, "getting identity", func() (*ory.Identity, error) {
		identity, httpResp, err := c.projectClient.IdentityAPI.GetIdentity(ctx, identityID).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		return identity, err
	})
}

// UpdateIdentity updates an identity with retry on rate limit.
func (c *OryClient) UpdateIdentity(ctx context.Context, identityID string, body ory.UpdateIdentityBody) (*ory.Identity, error) {
	return retryWithBackoff(ctx, "updating identity", func() (*ory.Identity, error) {
		identity, httpResp, err := c.projectClient.IdentityAPI.UpdateIdentity(ctx, identityID).UpdateIdentityBody(body).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		return identity, err
	})
}

// DeleteIdentity deletes an identity with retry on rate limit.
func (c *OryClient) DeleteIdentity(ctx context.Context, identityID string) error {
	_, err := retryWithBackoff(ctx, "deleting identity", func() (struct{}, error) {
		httpResp, err := c.projectClient.IdentityAPI.DeleteIdentity(ctx, identityID).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}
		return struct{}{}, err
	})
	return err
}

// =============================================================================
// OAuth2 Client Operations (Project API)
// =============================================================================

// CreateOAuth2Client creates a new OAuth2 client.
func (c *OryClient) CreateOAuth2Client(ctx context.Context, oauthClient ory.OAuth2Client) (*ory.OAuth2Client, error) {
	result, httpResp, err := c.projectClient.OAuth2API.CreateOAuth2Client(ctx).OAuth2Client(oauthClient).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return result, err
}

// GetOAuth2Client retrieves an OAuth2 client by ID.
func (c *OryClient) GetOAuth2Client(ctx context.Context, clientID string) (*ory.OAuth2Client, error) {
	oauthClient, httpResp, err := c.projectClient.OAuth2API.GetOAuth2Client(ctx, clientID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return oauthClient, err
}

// UpdateOAuth2Client updates an OAuth2 client.
func (c *OryClient) UpdateOAuth2Client(ctx context.Context, clientID string, oauthClient ory.OAuth2Client) (*ory.OAuth2Client, error) {
	result, httpResp, err := c.projectClient.OAuth2API.SetOAuth2Client(ctx, clientID).OAuth2Client(oauthClient).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return result, err
}

// DeleteOAuth2Client deletes an OAuth2 client.
func (c *OryClient) DeleteOAuth2Client(ctx context.Context, clientID string) error {
	httpResp, err := c.projectClient.OAuth2API.DeleteOAuth2Client(ctx, clientID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return err
}

// =============================================================================
// Project API Key Operations (Console API)
// =============================================================================

// CreateProjectAPIKey creates a new API key for a project.
func (c *OryClient) CreateProjectAPIKey(ctx context.Context, projectID string, body ory.CreateProjectApiKeyRequest) (*ory.ProjectApiKey, error) {
	key, httpResp, err := c.consoleClient.ProjectAPI.CreateProjectApiKey(ctx, projectID).CreateProjectApiKeyRequest(body).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return key, err
}

// ListProjectAPIKeys lists all API keys for a project.
func (c *OryClient) ListProjectAPIKeys(ctx context.Context, projectID string) ([]ory.ProjectApiKey, error) {
	keys, httpResp, err := c.consoleClient.ProjectAPI.ListProjectApiKeys(ctx, projectID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return keys, err
}

// DeleteProjectAPIKey deletes an API key with retry logic for transient errors.
func (c *OryClient) DeleteProjectAPIKey(ctx context.Context, projectID, keyID string) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		httpResp, err := c.consoleClient.ProjectAPI.DeleteProjectApiKey(ctx, projectID, keyID).Execute()
		if httpResp != nil {
			_ = httpResp.Body.Close()
		}

		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on rate limit or 5xx errors
		if !isRateLimitError(err) && !isRetryableError(err) {
			return err
		}

		if attempt == maxRetries {
			break
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("deleting API key: failed after %d retries: %w", maxRetries, lastErr)
}

// =============================================================================
// JSON Web Key Set Operations (Project API)
// =============================================================================

// CreateJsonWebKeySet creates a new JWK set.
func (c *OryClient) CreateJsonWebKeySet(ctx context.Context, setID string, body ory.CreateJsonWebKeySet) (*ory.JsonWebKeySet, error) {
	jwks, httpResp, err := c.projectClient.JwkAPI.CreateJsonWebKeySet(ctx, setID).CreateJsonWebKeySet(body).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return jwks, err
}

// GetJsonWebKeySet retrieves a JWK set by ID.
func (c *OryClient) GetJsonWebKeySet(ctx context.Context, setID string) (*ory.JsonWebKeySet, error) {
	jwks, httpResp, err := c.projectClient.JwkAPI.GetJsonWebKeySet(ctx, setID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return jwks, err
}

// DeleteJsonWebKeySet deletes a JWK set.
func (c *OryClient) DeleteJsonWebKeySet(ctx context.Context, setID string) error {
	httpResp, err := c.projectClient.JwkAPI.DeleteJsonWebKeySet(ctx, setID).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return err
}

// =============================================================================
// Relationship Operations (Project API - Ory Keto)
// =============================================================================

// CreateRelationship creates a new relationship tuple.
func (c *OryClient) CreateRelationship(ctx context.Context, body ory.CreateRelationshipBody) (*ory.Relationship, error) {
	rel, httpResp, err := c.projectClient.RelationshipAPI.CreateRelationship(ctx).CreateRelationshipBody(body).Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return rel, err
}

// GetRelationships queries relationships.
func (c *OryClient) GetRelationships(ctx context.Context, namespace string, object *string, relation *string, subjectID *string) (*ory.Relationships, error) {
	req := c.projectClient.RelationshipAPI.GetRelationships(ctx).Namespace(namespace)
	if object != nil {
		req = req.Object(*object)
	}
	if relation != nil {
		req = req.Relation(*relation)
	}
	if subjectID != nil {
		req = req.SubjectId(*subjectID)
	}
	rels, httpResp, err := req.Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return rels, err
}

// DeleteRelationships deletes relationships matching the query.
func (c *OryClient) DeleteRelationships(ctx context.Context, namespace string, object *string, relation *string, subjectID *string) error {
	req := c.projectClient.RelationshipAPI.DeleteRelationships(ctx).Namespace(namespace)
	if object != nil {
		req = req.Object(*object)
	}
	if relation != nil {
		req = req.Relation(*relation)
	}
	if subjectID != nil {
		req = req.SubjectId(*subjectID)
	}
	httpResp, err := req.Execute()
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return err
}
