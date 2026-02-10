package helpers

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ResolveProjectID resolves the project ID from the plan/state or provider configuration.
// It returns the project ID and adds an error to diagnostics if both are empty.
func ResolveProjectID(planID types.String, clientID string, diags *diag.Diagnostics) string {
	projectID := planID.ValueString()
	if projectID == "" {
		projectID = clientID
	}
	if projectID == "" {
		diags.AddError(
			"Missing Project ID",
			"project_id must be set either in the resource configuration or in the provider configuration (ORY_PROJECT_ID environment variable).",
		)
	}
	return projectID
}

// ResolveProjectCreds validates that both project slug and API key are configured.
// Returns true if valid, false if not (with error added to diagnostics).
func ResolveProjectCreds(slug, apiKey string, diags *diag.Diagnostics) bool {
	if slug == "" || apiKey == "" {
		diags.AddError(
			"Missing Project Credentials",
			"Both project_slug and project_api_key must be set in the provider configuration.\n"+
				"Set them via provider config or environment variables: ORY_PROJECT_SLUG, ORY_PROJECT_API_KEY.",
		)
		return false
	}
	return true
}
