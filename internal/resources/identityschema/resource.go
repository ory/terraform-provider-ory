package identityschema

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
)

var (
	_ resource.Resource                = &IdentitySchemaResource{}
	_ resource.ResourceWithConfigure   = &IdentitySchemaResource{}
	_ resource.ResourceWithImportState = &IdentitySchemaResource{}
)

func NewResource() resource.Resource {
	return &IdentitySchemaResource{}
}

type IdentitySchemaResource struct {
	client *client.OryClient
}

type IdentitySchemaResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ProjectID  types.String `tfsdk:"project_id"`
	SchemaID   types.String `tfsdk:"schema_id"`
	Schema     types.String `tfsdk:"schema"`
	SetDefault types.Bool   `tfsdk:"set_default"`
}

func (r *IdentitySchemaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_schema"
}

func (r *IdentitySchemaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network identity schema.",
		MarkdownDescription: `
Manages an Ory Network identity schema.

**Note:** Identity schemas are immutable in Ory Network. Any changes to the schema content will require resource replacement.

**Note:** Ory Network does not support deleting identity schemas. When this resource is destroyed, the schema will remain in Ory but will no longer be managed by Terraform.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID. If not set, uses provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema_id": schema.StringAttribute{
				Description: "Unique identifier for the schema (e.g., 'user', 'employee').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema": schema.StringAttribute{
				Description: "JSON Schema definition for the identity traits (JSON string).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(), // Schemas are immutable
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"set_default": schema.BoolAttribute{
				Description: "Set this schema as the project's default schema.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *IdentitySchemaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T", req.ProviderData))
		return
	}
	r.client = oryClient
}

func (r *IdentitySchemaResource) encodeSchema(schemaJSON string) (string, error) {
	// Validate it's valid JSON
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		return "", fmt.Errorf("invalid JSON schema: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(schemaJSON))
	return "base64://" + encoded, nil
}

func (r *IdentitySchemaResource) getSchemas(ctx context.Context, projectID string) ([]map[string]interface{}, error) {
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if project.Services.Identity == nil {
		return nil, nil
	}

	configMap := project.Services.Identity.Config
	if configMap == nil {
		return nil, nil
	}

	identity, _ := configMap["identity"].(map[string]interface{})
	schemas, _ := identity["schemas"].([]interface{})

	result := make([]map[string]interface{}, 0, len(schemas))
	for _, s := range schemas {
		if sm, ok := s.(map[string]interface{}); ok {
			result = append(result, sm)
		}
	}
	return result, nil
}

func (r *IdentitySchemaResource) findSchemaIndex(schemas []map[string]interface{}, schemaID string) int {
	for i, s := range schemas {
		if s["id"] == schemaID {
			return i
		}
	}
	return -1
}

// findSchemaByURL finds a schema by matching its URL content.
// This is needed because Ory API may transform custom schema IDs to hash-based IDs.
func (r *IdentitySchemaResource) findSchemaByURL(schemas []map[string]interface{}, schemaURL string) int {
	for i, s := range schemas {
		if url, ok := s["url"].(string); ok && url == schemaURL {
			return i
		}
	}
	return -1
}

func (r *IdentitySchemaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IdentitySchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	schemaID := plan.SchemaID.ValueString()
	schemaJSON := plan.Schema.ValueString()

	schemaURL, err := r.encodeSchema(schemaJSON)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Schema", err.Error())
		return
	}

	// Get existing schemas and their IDs before the patch
	existingSchemas, err := r.getSchemas(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Schemas", err.Error())
		return
	}

	existingIDs := make(map[string]bool)
	for _, s := range existingSchemas {
		if id, ok := s["id"].(string); ok {
			existingIDs[id] = true
		}
	}

	var patches []ory.JsonPatch

	existingIndex := r.findSchemaIndex(existingSchemas, schemaID)
	if existingIndex >= 0 {
		// Replace existing
		patches = append(patches, ory.JsonPatch{
			Op:   "replace",
			Path: fmt.Sprintf("/services/identity/config/identity/schemas/%d", existingIndex),
			Value: map[string]string{
				"id":  schemaID,
				"url": schemaURL,
			},
		})
	} else {
		// Add new schema
		patches = append(patches, ory.JsonPatch{
			Op:   "add",
			Path: "/services/identity/config/identity/schemas/-",
			Value: map[string]string{
				"id":  schemaID,
				"url": schemaURL,
			},
		})
	}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Identity Schema", err.Error())
		return
	}

	// Find the newly created schema by looking for new IDs
	var actualID string
	for attempt := 0; attempt < 5; attempt++ {
		updatedSchemas, err := r.getSchemas(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Created Schema", err.Error())
			return
		}

		// Try to find by the user-provided schema ID first (works for preset:// schemas)
		if idx := r.findSchemaIndex(updatedSchemas, schemaID); idx >= 0 {
			if id, ok := updatedSchemas[idx]["id"].(string); ok {
				actualID = id
			}
			break
		}

		// Try to find by URL match (Ory may keep the base64 URL for some cases)
		if idx := r.findSchemaByURL(updatedSchemas, schemaURL); idx >= 0 {
			if id, ok := updatedSchemas[idx]["id"].(string); ok {
				actualID = id
			}
			break
		}

		// Find the newly added schema by looking for IDs that didn't exist before
		// This handles the case where Ory transforms the ID to a hash
		for _, s := range updatedSchemas {
			if id, ok := s["id"].(string); ok {
				if !existingIDs[id] {
					// This is a new schema that wasn't there before
					actualID = id
					break
				}
			}
		}

		if actualID != "" {
			break
		}

		// Wait before retry (exponential backoff: 500ms, 1s, 2s, 4s, 8s)
		if attempt < 4 {
			select {
			case <-ctx.Done():
				resp.Diagnostics.AddError("Context Cancelled", "Operation was cancelled while waiting for schema creation")
				return
			case <-time.After(time.Duration(500<<attempt) * time.Millisecond):
			}
		}
	}

	// If we still couldn't find by comparison, use the schema_id directly as a fallback
	// This handles cases where Ory preserves the original ID or it matches by some other mechanism
	if actualID == "" {
		actualID = schemaID
	}

	if actualID == "" {
		resp.Diagnostics.AddError("Error Finding Created Schema",
			"Could not find the created schema. The schema was created but its ID could not be determined.")
		return
	}

	// Set as default if requested
	if plan.SetDefault.ValueBool() {
		defaultPatches := []ory.JsonPatch{{
			Op:    "replace",
			Path:  "/services/identity/config/identity/default_schema_id",
			Value: actualID,
		}}
		_, err = r.client.PatchProject(ctx, projectID, defaultPatches)
		if err != nil {
			resp.Diagnostics.AddError("Error Setting Default Schema", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(actualID)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *IdentitySchemaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IdentitySchemaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()

	// If projectID is empty, try to get it from the client
	if projectID == "" {
		projectID = r.client.ProjectID()
		state.ProjectID = types.StringValue(projectID)
	}

	storedID := state.ID.ValueString()
	schemaID := state.SchemaID.ValueString()

	// Generate schemaURL for matching if we have schema content
	var schemaURL string
	if !state.Schema.IsNull() && !state.Schema.IsUnknown() {
		var err error
		schemaURL, err = r.encodeSchema(state.Schema.ValueString())
		if err != nil {
			schemaURL = ""
		}
	}

	// Retry logic for eventual consistency after create/update
	var schemas []map[string]interface{}
	var index int
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		schemas, err = r.getSchemas(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Identity Schema", err.Error())
			return
		}

		// The ID stored in state is the actual API-assigned ID (which may be a hash)
		// Try to find by ID first (using the actual API ID from state)
		index = -1
		if storedID != "" {
			index = r.findSchemaIndex(schemas, storedID)
		}

		// Fallback: try finding by the user-provided schema_id
		if index < 0 {
			index = r.findSchemaIndex(schemas, schemaID)
		}

		// Last resort: try to match by regenerating the URL from the schema content
		if index < 0 && schemaURL != "" {
			index = r.findSchemaByURL(schemas, schemaURL)
		}

		if index >= 0 {
			break
		}

		// Wait before retry (exponential backoff: 1s, 2s, 4s, 8s)
		if attempt < 4 {
			select {
			case <-ctx.Done():
				resp.State.RemoveResource(ctx)
				return
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}

	if index < 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Update the ID if we found it by a different method
	if id, ok := schemas[index]["id"].(string); ok {
		state.ID = types.StringValue(id)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IdentitySchemaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IdentitySchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	schemaID := plan.SchemaID.ValueString()

	// Only thing that can be updated without replacement is set_default
	if plan.SetDefault.ValueBool() {
		patches := []ory.JsonPatch{{
			Op:    "replace",
			Path:  "/services/identity/config/identity/default_schema_id",
			Value: schemaID,
		}}
		_, err := r.client.PatchProject(ctx, projectID, patches)
		if err != nil {
			resp.Diagnostics.AddError("Error Setting Default Schema", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(schemaID)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *IdentitySchemaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Ory Network does not support deleting identity schemas.
	// See: https://github.com/ory/network/issues/262
	//
	// We just remove it from Terraform state. The schema will remain in Ory.
	resp.Diagnostics.AddWarning(
		"Schema Not Deleted",
		"Ory Network does not support deleting identity schemas. The schema has been removed from Terraform state but still exists in Ory Network.",
	)
}

func (r *IdentitySchemaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("schema_id"), req.ID)...)
}
