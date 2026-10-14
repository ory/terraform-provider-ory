package projectconfig

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Keys in the normalized project revision that govern Keto strict mode.
//
// keto_strict_mode_readonly is backoffice metadata: projects created since
// 2026-07-29 get it set to true together with strict mode, and the Console API
// then answers every write to keto_feature_flags_strict_mode with HTTP 200
// while keeping the stored value. Without a guard, Terraform would store the
// planned value, Read would refresh the stored one, and every later plan
// would show the same change again.
const (
	ketoStrictModeKey         = "keto_feature_flags_strict_mode"
	ketoStrictModeReadonlyKey = "keto_strict_mode_readonly"
)

// ketoStrictModeLocked reports whether a write of planned to
// keto_feature_flags_strict_mode would be discarded for this revision, and the
// value the API keeps. A revision without the readonly flag is writable. A
// locked revision still accepts a write that matches the stored value, because
// that write is a no-op, so only a mismatch counts as locked. A locked revision
// without a stored value keeps the default, false.
func ketoStrictModeLocked(revision map[string]interface{}, planned bool) (stored bool, locked bool) {
	readonly, ok := getNestedBool(revision, ketoStrictModeReadonlyKey)
	if !ok || !readonly {
		return false, false
	}
	stored, _ = getNestedBool(revision, ketoStrictModeKey)
	return stored, stored != planned
}

// checkKetoStrictModeWritable fails the apply when the plan sets
// keto_feature_flags_strict_mode to a value that the project's lock would
// discard. It reads the normalized revision only when the attribute is set.
// When that read fails, the apply fails too: without the lock state the
// provider cannot tell whether the API would keep the write, and a discarded
// write would otherwise be stored in state and reported as applied.
func (r *ProjectConfigResource) checkKetoStrictModeWritable(ctx context.Context, projectID string, plan *ProjectConfigResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if plan.KetoFeatureFlagsStrictMode.IsNull() || plan.KetoFeatureFlagsStrictMode.IsUnknown() {
		return diags
	}

	revision, err := r.client.GetProjectNormalizedRevision(ctx, projectID)
	if err != nil {
		tflog.Error(ctx, "Could not read the normalized project revision to check the Keto strict mode lock", map[string]interface{}{
			"project_id": projectID,
			"error":      err.Error(),
		})
		diags.AddAttributeError(
			path.Root(ketoStrictModeKey),
			"Could not verify the Keto strict mode lock",
			fmt.Sprintf("Could not read the normalized revision for project %s: %s. "+
				"The lock decides whether the Ory API keeps a write to %s, so the apply stops before sending it. "+
				"Retry, or remove the attribute.",
				projectID, err, ketoStrictModeKey),
		)
		return diags
	}

	planned := plan.KetoFeatureFlagsStrictMode.ValueBool()
	stored, locked := ketoStrictModeLocked(revision, planned)
	if !locked {
		return diags
	}

	diags.AddAttributeError(
		path.Root(ketoStrictModeKey),
		"Keto strict mode is locked on this project",
		fmt.Sprintf("Project %s was created with Keto strict mode locked (%s is true). "+
			"The Ory API accepts a write to %s but keeps the stored value %t, so Terraform cannot set it to %t. "+
			"Remove the attribute or set it to %t.",
			projectID, ketoStrictModeReadonlyKey, ketoStrictModeKey, stored, planned, stored),
	)
	return diags
}
