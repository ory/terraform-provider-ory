package projectconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Import has no access to the resource configuration. An explicit field list
// lets a partial configuration adopt live values without importing unrelated
// fields that Terraform would otherwise plan to remove. Bare project IDs keep
// the existing IDs-only import behavior.
func (r *ProjectConfigResource) importSelectedFields(ctx context.Context, projectID, fields string, resp *resource.ImportStateResponse) {
	if projectID == "" || strings.TrimSpace(projectID) != projectID || fields == "" {
		resp.Diagnostics.AddError("Invalid Project Config Import ID", "Use project_id:attribute_one,attribute_two with a non-empty project ID and field list.")
		return
	}

	names := strings.Split(fields, ",")
	selected := make(map[string]attr.Value, len(names))
	attributes := resp.State.Schema.GetAttributes()
	for _, name := range names {
		attribute, exists := attributes[name]
		if !exists || name == "id" || name == "project_id" || (!attribute.IsOptional() && !attribute.IsRequired()) {
			resp.Diagnostics.AddError("Invalid Project Config Import Field", fmt.Sprintf("%q is not a configurable project setting.", name))
			return
		}
		if _, duplicate := selected[name]; duplicate {
			resp.Diagnostics.AddError("Duplicate Project Config Import Field", fmt.Sprintf("%q is listed more than once.", name))
			return
		}
		if attribute.IsSensitive() || attribute.IsWriteOnly() {
			resp.Diagnostics.AddError("Unsupported Project Config Import Field", fmt.Sprintf("%q is sensitive or write-only and cannot be imported by field selection.", name))
			return
		}
		if name == "mfa_enforcement" || name == "smtp_connection_uri_wo_version" {
			resp.Diagnostics.AddError("Unsupported Project Config Import Field", fmt.Sprintf("%q has no reader and cannot be imported by field selection.", name))
			return
		}
		// This reader needs an existing inline payload to resolve storage URLs.
		if name == "courier_http_request_config_body" {
			resp.Diagnostics.AddError("Unsupported Project Config Import Field", fmt.Sprintf("%q reads back as a storage URL, so field selection cannot recover its inline base64 payload.", name))
			return
		}
		// The API appends its own entries to this list and the reader keeps
		// only the configured ones, so there is no live baseline to adopt.
		if name == "allowed_return_urls" {
			resp.Diagnostics.AddError("Unsupported Project Config Import Field", fmt.Sprintf("%q reads back with server-appended entries that the provider filters against your configuration, so field selection cannot establish its baseline. Configure it and apply instead.", name))
			return
		}
		switch attribute.GetType() {
		case types.StringType:
			selected[name] = types.StringUnknown()
		case types.BoolType:
			selected[name] = types.BoolUnknown()
		case types.Int64Type:
			selected[name] = types.Int64Unknown()
		case types.ListType{ElemType: types.StringType}:
			selected[name] = types.ListUnknown(types.StringType)
		case types.MapType{ElemType: types.StringType}:
			selected[name] = types.MapUnknown(types.StringType)
		default:
			resp.Diagnostics.AddError("Unsupported Project Config Import Field", fmt.Sprintf("%q is a nested object. Field selection supports readable string, bool, integer, list of string, and map of string settings only.", name))
			return
		}
	}

	// cors_enabled is read even when it was not selected, so the warning
	// below can be limited to projects where its default would change
	// something. It is returned to null afterwards.
	_, selectedCORS := selected["cors_enabled"]
	if !selectedCORS {
		selected["cors_enabled"] = types.BoolUnknown()
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	for name, value := range selected {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), value)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Unknown values request a read without inventing defaults. Resolve them
	// before returning: import state must never contain unknown values.
	read := resource.ReadResponse{State: resp.State}
	r.Read(ctx, resource.ReadRequest{State: resp.State}, &read)
	resp.Diagnostics.Append(read.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	if read.State.Raw.IsNull() {
		resp.Diagnostics.AddError("Project Config Not Found", "The selected project no longer exists or has been deleted.")
		return
	}
	var values map[string]tftypes.Value
	if err := read.State.Raw.As(&values); err != nil {
		resp.Diagnostics.AddError("Error Reading Imported Project Config", err.Error())
		return
	}
	for _, name := range names {
		if !values[name].IsKnown() || values[name].IsNull() {
			resp.Diagnostics.AddError("Project Config Import Field Unavailable", fmt.Sprintf("The provider could not read a value for %q. The API may have omitted the value, the attribute may have no reader, or a read request may have failed. If you selected a setting and its deprecated alias, keep one of them. Check the provider logs and retry if a request failed. Otherwise, omit this field from the selection. No default has been substituted.", name))
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if !selectedCORS {
		var live types.Bool
		resp.Diagnostics.Append(read.State.GetAttribute(ctx, path.Root("cors_enabled"), &live)...)
		if live.IsUnknown() || live.IsNull() || live.ValueBool() {
			warnCORSDefault(&resp.Diagnostics)
		}
		resp.Diagnostics.Append(read.State.SetAttribute(ctx, path.Root("cors_enabled"), types.BoolNull())...)
	}
	resp.State = read.State
}

// warnCORSDefault explains the one schema default that can change a project
// after import. Both import forms leave cors_enabled null unless it was
// selected, and the default of false then materializes on the next plan.
func warnCORSDefault(diags *diag.Diagnostics) {
	diags.AddWarning("Project Config Import Leaves CORS Defaulted",
		"cors_enabled was not imported and has a provider default of false. If your configuration also omits it, the next plan proposes false, and applying that plan disables public CORS. Set cors_enabled to the project's current value in your configuration, or import it by field selection: terraform import ory_project_config.main '<project-id>:cors_enabled,...'.")
}
