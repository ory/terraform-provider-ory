package helpers

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// ResolveProjectClient returns a request-scoped client, optionally using
// resource-level project credentials. The baseClient is never mutated, which
// avoids data races when Terraform runs multiple instances of the same resource
// type concurrently (e.g., for_each across projects).
func ResolveProjectClient(baseClient *client.OryClient, slug, apiKey types.String) *client.OryClient {
	if !slug.IsNull() && !slug.IsUnknown() && !apiKey.IsNull() && !apiKey.IsUnknown() {
		return baseClient.WithProjectCredentials(slug.ValueString(), apiKey.ValueString())
	}
	return baseClient
}

// ValidateProjectCredentialPair validates that project_slug and project_api_key
// are either both set or both unset. Returns diagnostics with attribute errors
// when only one is provided. Unknown values are skipped (resolved at apply time).
func ValidateProjectCredentialPair(slug, apiKey types.String) diag.Diagnostics {
	var diags diag.Diagnostics

	// Skip validation if either value is unknown (will be resolved at apply time)
	if slug.IsUnknown() || apiKey.IsUnknown() {
		return diags
	}

	slugSet := !slug.IsNull()
	keySet := !apiKey.IsNull()

	if slugSet != keySet {
		if !slugSet {
			diags.AddAttributeError(
				path.Root("project_slug"),
				"Missing Required Attribute",
				"project_slug is required when project_api_key is set. Both must be specified together.",
			)
		}
		if !keySet {
			diags.AddAttributeError(
				path.Root("project_api_key"),
				"Missing Required Attribute",
				"project_api_key is required when project_slug is set. Both must be specified together.",
			)
		}
	}

	return diags
}

// HasResourceLevelCredentials reports whether both project_slug and
// project_api_key are known and non-null, indicating the user has provided
// resource-level credentials.
func HasResourceLevelCredentials(slug, apiKey types.String) bool {
	return !slug.IsNull() && !slug.IsUnknown() && !apiKey.IsNull() && !apiKey.IsUnknown()
}
