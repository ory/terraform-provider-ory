package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	ory "github.com/ory/client-go"
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
	if apiErr, isAPIErr := err.(*ory.GenericOpenAPIError); isAPIErr {
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
	if apiErr, isAPIErr := err.(*ory.GenericOpenAPIError); isAPIErr {
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
	if err == io.EOF || strings.Contains(errStr, "EOF") || strings.Contains(errStr, "error reading from server") {
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
		consoleCfg := ory.NewConfiguration()
		consoleCfg.Servers = ory.ServerConfigurations{
			{URL: cfg.ConsoleAPIURL},
		}
		consoleCfg.AddDefaultHeader("Authorization", "Bearer "+cfg.WorkspaceAPIKey)
		client.consoleClient = ory.NewAPIClient(consoleCfg)
	}

	// Initialize project client if project API key and slug are provided
	if cfg.ProjectAPIKey != "" && cfg.ProjectSlug != "" {
		projectCfg := ory.NewConfiguration()
		// Use configurable URL template, defaulting to production
		projectAPIURL := cfg.ProjectAPIURL
		if projectAPIURL == "" {
			projectAPIURL = "https://%s.projects.oryapis.com"
		}
		projectCfg.Servers = ory.ServerConfigurations{
			{URL: fmt.Sprintf(projectAPIURL, cfg.ProjectSlug)},
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
func (c *OryClient) CreateProject(ctx context.Context, name, environment string) (*ory.Project, error) {
	body := ory.CreateProjectBody{
		Name:        name,
		Environment: environment,
	}
	if c.config.WorkspaceID != "" {
		body.WorkspaceId = ory.PtrString(c.config.WorkspaceID)
	}

	project, _, err := c.consoleClient.ProjectAPI.CreateProject(ctx).CreateProjectBody(body).Execute()
	return project, err
}

// GetProject retrieves a project by ID.
func (c *OryClient) GetProject(ctx context.Context, projectID string) (*ory.Project, error) {
	project, _, err := c.consoleClient.ProjectAPI.GetProject(ctx, projectID).Execute()
	return project, err
}

// DeleteProject purges a project.
func (c *OryClient) DeleteProject(ctx context.Context, projectID string) error {
	_, err := c.consoleClient.ProjectAPI.PurgeProject(ctx, projectID).Execute()
	return err
}

// PatchProject applies JSON Patch operations to a project.
func (c *OryClient) PatchProject(ctx context.Context, projectID string, patches []ory.JsonPatch) (*ory.SuccessfulProjectUpdate, error) {
	result, _, err := c.consoleClient.ProjectAPI.PatchProject(ctx, projectID).
		JsonPatch(patches).
		Execute()
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
	workspace, _, err := c.consoleClient.WorkspaceAPI.CreateWorkspace(ctx).CreateWorkspaceBody(body).Execute()
	return workspace, err
}

// GetWorkspace retrieves a workspace by ID.
func (c *OryClient) GetWorkspace(ctx context.Context, workspaceID string) (*ory.Workspace, error) {
	workspace, _, err := c.consoleClient.WorkspaceAPI.GetWorkspace(ctx, workspaceID).Execute()
	return workspace, err
}

// UpdateWorkspace updates a workspace.
func (c *OryClient) UpdateWorkspace(ctx context.Context, workspaceID, name string) (*ory.Workspace, error) {
	body := ory.UpdateWorkspaceBody{
		Name: name,
	}
	workspace, _, err := c.consoleClient.WorkspaceAPI.UpdateWorkspace(ctx, workspaceID).UpdateWorkspaceBody(body).Execute()
	return workspace, err
}

// GetProjectEnvironment retrieves the environment (prod, stage, dev) for a project.
func (c *OryClient) GetProjectEnvironment(ctx context.Context, projectID string) (string, error) {
	if c.consoleClient == nil {
		return "", fmt.Errorf("console API client not configured")
	}
	project, _, err := c.consoleClient.ProjectAPI.GetProject(ctx, projectID).Execute()
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

	org, _, err := c.consoleClient.ProjectAPI.CreateOrganization(ctx, projectID).OrganizationBody(body).Execute()
	if err != nil {
		return nil, wrapAPIError(err, "creating organization")
	}
	return org, nil
}

// GetOrganization retrieves an organization by ID.
func (c *OryClient) GetOrganization(ctx context.Context, projectID, orgID string) (*ory.Organization, error) {
	if c.consoleClient == nil {
		return nil, fmt.Errorf("reading organization: console API client not configured. " +
			"Set workspace_api_key (ORY_WORKSPACE_API_KEY)")
	}
	resp, _, err := c.consoleClient.ProjectAPI.GetOrganization(ctx, projectID, orgID).Execute()
	if err != nil {
		return nil, wrapAPIError(err, "reading organization")
	}
	return &resp.Organization, nil
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

	org, _, err := c.consoleClient.ProjectAPI.UpdateOrganization(ctx, projectID, orgID).OrganizationBody(body).Execute()
	if err != nil {
		return nil, wrapAPIError(err, "updating organization")
	}
	return org, nil
}

// DeleteOrganization deletes an organization.
func (c *OryClient) DeleteOrganization(ctx context.Context, projectID, orgID string) error {
	if c.consoleClient == nil {
		return fmt.Errorf("deleting organization: console API client not configured. " +
			"Set workspace_api_key (ORY_WORKSPACE_API_KEY)")
	}
	_, err := c.consoleClient.ProjectAPI.DeleteOrganization(ctx, projectID, orgID).Execute()
	return wrapAPIError(err, "deleting organization")
}

// =============================================================================
// Identity Operations (Project API)
// =============================================================================

// CreateIdentity creates a new identity.
func (c *OryClient) CreateIdentity(ctx context.Context, body ory.CreateIdentityBody) (*ory.Identity, error) {
	identity, _, err := c.projectClient.IdentityAPI.CreateIdentity(ctx).CreateIdentityBody(body).Execute()
	return identity, err
}

// GetIdentity retrieves an identity by ID.
func (c *OryClient) GetIdentity(ctx context.Context, identityID string) (*ory.Identity, error) {
	identity, _, err := c.projectClient.IdentityAPI.GetIdentity(ctx, identityID).Execute()
	return identity, err
}

// UpdateIdentity updates an identity.
func (c *OryClient) UpdateIdentity(ctx context.Context, identityID string, body ory.UpdateIdentityBody) (*ory.Identity, error) {
	identity, _, err := c.projectClient.IdentityAPI.UpdateIdentity(ctx, identityID).UpdateIdentityBody(body).Execute()
	return identity, err
}

// DeleteIdentity deletes an identity.
func (c *OryClient) DeleteIdentity(ctx context.Context, identityID string) error {
	_, err := c.projectClient.IdentityAPI.DeleteIdentity(ctx, identityID).Execute()
	return err
}

// =============================================================================
// OAuth2 Client Operations (Project API)
// =============================================================================

// CreateOAuth2Client creates a new OAuth2 client.
func (c *OryClient) CreateOAuth2Client(ctx context.Context, oauthClient ory.OAuth2Client) (*ory.OAuth2Client, error) {
	result, _, err := c.projectClient.OAuth2API.CreateOAuth2Client(ctx).OAuth2Client(oauthClient).Execute()
	return result, err
}

// GetOAuth2Client retrieves an OAuth2 client by ID.
func (c *OryClient) GetOAuth2Client(ctx context.Context, clientID string) (*ory.OAuth2Client, error) {
	oauthClient, _, err := c.projectClient.OAuth2API.GetOAuth2Client(ctx, clientID).Execute()
	return oauthClient, err
}

// UpdateOAuth2Client updates an OAuth2 client.
func (c *OryClient) UpdateOAuth2Client(ctx context.Context, clientID string, oauthClient ory.OAuth2Client) (*ory.OAuth2Client, error) {
	result, _, err := c.projectClient.OAuth2API.SetOAuth2Client(ctx, clientID).OAuth2Client(oauthClient).Execute()
	return result, err
}

// DeleteOAuth2Client deletes an OAuth2 client.
func (c *OryClient) DeleteOAuth2Client(ctx context.Context, clientID string) error {
	_, err := c.projectClient.OAuth2API.DeleteOAuth2Client(ctx, clientID).Execute()
	return err
}

// =============================================================================
// Project API Key Operations (Console API)
// =============================================================================

// CreateProjectAPIKey creates a new API key for a project.
func (c *OryClient) CreateProjectAPIKey(ctx context.Context, projectID string, body ory.CreateProjectApiKeyRequest) (*ory.ProjectApiKey, error) {
	key, _, err := c.consoleClient.ProjectAPI.CreateProjectApiKey(ctx, projectID).CreateProjectApiKeyRequest(body).Execute()
	return key, err
}

// ListProjectAPIKeys lists all API keys for a project.
func (c *OryClient) ListProjectAPIKeys(ctx context.Context, projectID string) ([]ory.ProjectApiKey, error) {
	keys, _, err := c.consoleClient.ProjectAPI.ListProjectApiKeys(ctx, projectID).Execute()
	return keys, err
}

// DeleteProjectAPIKey deletes an API key.
func (c *OryClient) DeleteProjectAPIKey(ctx context.Context, projectID, keyID string) error {
	_, err := c.consoleClient.ProjectAPI.DeleteProjectApiKey(ctx, projectID, keyID).Execute()
	return err
}

// =============================================================================
// JSON Web Key Set Operations (Project API)
// =============================================================================

// CreateJsonWebKeySet creates a new JWK set.
func (c *OryClient) CreateJsonWebKeySet(ctx context.Context, setID string, body ory.CreateJsonWebKeySet) (*ory.JsonWebKeySet, error) {
	jwks, _, err := c.projectClient.JwkAPI.CreateJsonWebKeySet(ctx, setID).CreateJsonWebKeySet(body).Execute()
	return jwks, err
}

// GetJsonWebKeySet retrieves a JWK set by ID.
func (c *OryClient) GetJsonWebKeySet(ctx context.Context, setID string) (*ory.JsonWebKeySet, error) {
	jwks, _, err := c.projectClient.JwkAPI.GetJsonWebKeySet(ctx, setID).Execute()
	return jwks, err
}

// DeleteJsonWebKeySet deletes a JWK set.
func (c *OryClient) DeleteJsonWebKeySet(ctx context.Context, setID string) error {
	_, err := c.projectClient.JwkAPI.DeleteJsonWebKeySet(ctx, setID).Execute()
	return err
}

// =============================================================================
// Relationship Operations (Project API - Ory Keto)
// =============================================================================

// CreateRelationship creates a new relationship tuple.
func (c *OryClient) CreateRelationship(ctx context.Context, body ory.CreateRelationshipBody) (*ory.Relationship, error) {
	rel, _, err := c.projectClient.RelationshipAPI.CreateRelationship(ctx).CreateRelationshipBody(body).Execute()
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
	rels, _, err := req.Execute()
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
	_, err := req.Execute()
	return err
}
