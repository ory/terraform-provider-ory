package identityschema

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ resource.Resource              = &IdentitySchemaResource{}
	_ resource.ResourceWithConfigure = &IdentitySchemaResource{}
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

const identitySchemaMarkdownDescription = `
Manages an Ory Network identity schema.

## Important Notes

- **Schemas are immutable**: Identity schemas cannot be modified after creation. Any changes to the schema content or ` + "`schema_id`" + ` will require Terraform to destroy and recreate the resource.
- **Schemas cannot be deleted**: When this resource is destroyed, the schema remains in Ory but is no longer managed by Terraform.
- **Import is not supported**: Existing schemas created via the Ory Console or API cannot be imported into Terraform. To manage an existing schema, recreate it in your Terraform configuration using the same content.

## Understanding IDs

This resource has two ID-related attributes:

| Attribute | Description |
|-----------|-------------|
| ` + "`id`" + ` | The API-assigned identifier (may be a hash like ` + "`abc123def456...`" + `). Read-only. |
| ` + "`schema_id`" + ` | Your chosen identifier (e.g., ` + "`customer`" + `, ` + "`employee_v2`" + `). You define this. |

When you create a schema with ` + "`schema_id = \"customer\"`" + `, Ory may internally store it with a different ID (hash).
The ` + "`id`" + ` attribute tracks the API's internal ID, while ` + "`schema_id`" + ` tracks your chosen name.

## Example Usage

` + "```hcl" + `
resource "ory_identity_schema" "customer" {
  schema_id   = "customer"
  set_default = true
  schema = jsonencode({
    "$id"     = "https://example.com/customer.schema.json"
    "$schema" = "http://json-schema.org/draft-07/schema#"
    title     = "Customer"
    type      = "object"
    properties = {
      traits = {
        type = "object"
        properties = {
          email = {
            type   = "string"
            format = "email"
            "ory.sh/kratos" = {
              credentials = { password = { identifier = true } }
              verification = { via = "email" }
              recovery     = { via = "email" }
            }
          }
        }
        required = ["email"]
      }
    }
  })
}
` + "```" + `
`

func (r *IdentitySchemaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Network identity schema.",
		MarkdownDescription: identitySchemaMarkdownDescription,
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
					stringplanmodifier.UseStateForUnknown(),
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

func extractSchemasFromProject(project *ory.Project) []map[string]interface{} {
	if project.Services.Identity == nil {
		return nil
	}

	configMap := project.Services.Identity.Config
	if configMap == nil {
		return nil
	}

	identity, _ := configMap["identity"].(map[string]interface{})
	schemas, _ := identity["schemas"].([]interface{})

	result := make([]map[string]interface{}, 0, len(schemas))
	for _, s := range schemas {
		if sm, ok := s.(map[string]interface{}); ok {
			result = append(result, sm)
		}
	}
	return result
}

func (r *IdentitySchemaResource) getSchemas(ctx context.Context, projectID string) ([]map[string]interface{}, error) {
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return extractSchemasFromProject(project), nil
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

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
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

	// If set_default is true, include the default_schema_id patch in the same
	// API call to avoid a race condition with eventual consistency.
	if plan.SetDefault.ValueBool() {
		patches = append(patches, ory.JsonPatch{
			Op:    "add",
			Path:  "/services/identity/config/identity/default_schema_id",
			Value: schemaID,
		})
	}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Identity Schema", err.Error())
		return
	}

	// Resolve the canonical schema ID using a fresh GetProject call.
	// PatchProject's response may preserve our input name, but the API may
	// internally assign a different ID (e.g., hash-based). A fresh GetProject
	// returns the canonical IDs.
	var actualID string
	err = helpers.WaitForCondition(ctx, func() (bool, error) {
		freshSchemas, err := r.getSchemas(ctx, projectID)
		if err != nil {
			return false, err
		}

		// Try by user-provided schema_id
		if idx := r.findSchemaIndex(freshSchemas, schemaID); idx >= 0 {
			if id, ok := freshSchemas[idx]["id"].(string); ok {
				actualID = id
				return true, nil
			}
		}

		// Try by URL match
		if idx := r.findSchemaByURL(freshSchemas, schemaURL); idx >= 0 {
			if id, ok := freshSchemas[idx]["id"].(string); ok {
				actualID = id
				return true, nil
			}
		}

		// Try to find a new ID not in the pre-creation set
		for _, s := range freshSchemas {
			if id, ok := s["id"].(string); ok {
				if !existingIDs[id] {
					actualID = id
					return true, nil
				}
			}
		}

		return false, nil
	})
	if err != nil || actualID == "" {
		// Fallback to user-provided schemaID if we can't resolve
		actualID = schemaID
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

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ProjectID = types.StringValue(projectID)

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

	var schemas []map[string]interface{}
	var index = -1

	if cached := r.client.GetCachedProject(projectID); cached != nil {
		schemas = extractSchemasFromProject(cached)
		if storedID != "" {
			index = r.findSchemaIndex(schemas, storedID)
		}
		if index < 0 {
			index = r.findSchemaIndex(schemas, schemaID)
		}
		if index < 0 && schemaURL != "" {
			index = r.findSchemaByURL(schemas, schemaURL)
		}
	}

	if index < 0 {
		var err error
		for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
			schemas, err = r.getSchemas(ctx, projectID)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading Identity Schema", err.Error())
				return
			}

			index = -1
			if storedID != "" {
				index = r.findSchemaIndex(schemas, storedID)
			}

			if index < 0 {
				index = r.findSchemaIndex(schemas, schemaID)
			}

			if index < 0 && schemaURL != "" {
				index = r.findSchemaByURL(schemas, schemaURL)
			}

			if index >= 0 {
				break
			}

			if attempt < helpers.ReadRetryMaxAttempts-1 {
				select {
				case <-ctx.Done():
					resp.State.RemoveResource(ctx)
					return
				case <-time.After(time.Second):
				}
			}
		}
	}

	if index < 0 {
		// List available schemas to help with debugging import issues
		var availableIDs []string
		for _, s := range schemas {
			if id, ok := s["id"].(string); ok {
				availableIDs = append(availableIDs, id)
			}
		}
		if len(availableIDs) > 0 {
			resp.Diagnostics.AddWarning(
				"Identity Schema Not Found",
				fmt.Sprintf("Could not find schema with id=%q or schema_id=%q.\n"+
					"Available schema IDs in this project: %v\n\n"+
					"When importing, use the exact ID shown above.",
					storedID, schemaID, availableIDs))
		}
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
	var state IdentitySchemaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use the API-assigned ID from state (which may differ from schema_id)
	apiID := state.ID.ValueString()
	if apiID == "" {
		apiID = plan.SchemaID.ValueString()
	}

	if plan.SetDefault.ValueBool() {
		// Re-resolve the schema ID from the API and retry to handle eventual consistency.
		// After Create, the API may not immediately return the newly added schema,
		// or may have assigned a different ID (e.g., hash-based).
		var lastErr error
		err := helpers.WaitForCondition(ctx, func() (bool, error) {
			schemas, err := r.getSchemas(ctx, projectID)
			if err != nil {
				return false, fmt.Errorf("failed to get schemas: %w", err)
			}

			// Build list of candidate IDs to try: state ID, schema_id, and any URL-matched ID
			candidateIDs := r.collectCandidateIDs(schemas, apiID, plan.SchemaID.ValueString(), state.Schema)
			if len(candidateIDs) == 0 {
				// Schema not found yet — may be eventual consistency
				lastErr = fmt.Errorf("schema not found in project (looked for id=%q or schema_id=%q, available: %v)",
					apiID, plan.SchemaID.ValueString(), schemaIDList(schemas))
				return false, nil
			}

			// Try each candidate until one succeeds
			for _, candidateID := range candidateIDs {
				patches := []ory.JsonPatch{{
					Op:    "add",
					Path:  "/services/identity/config/identity/default_schema_id",
					Value: candidateID,
				}}
				_, patchErr := r.client.PatchProject(ctx, projectID, patches)
				if patchErr == nil {
					apiID = candidateID
					return true, nil
				}
				lastErr = patchErr
			}
			// All candidates failed — retry after delay
			return false, nil
		})
		if err != nil {
			detail := err.Error()
			if lastErr != nil {
				detail = fmt.Sprintf("%s (last API error: %s)", detail, lastErr.Error())
			}
			resp.Diagnostics.AddError("Error Setting Default Schema", detail)
			return
		}
	}

	// Preserve the API-assigned ID from state
	plan.ID = state.ID
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

// collectCandidateIDs returns a deduplicated list of schema IDs that could be our schema.
// It tries: stored ID, user-provided schema_id, URL match, and all schema IDs as fallback.
func (r *IdentitySchemaResource) collectCandidateIDs(schemas []map[string]interface{}, storedID, schemaID string, schemaAttr types.String) []string {
	seen := make(map[string]bool)
	var candidates []string
	addCandidate := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			candidates = append(candidates, id)
		}
	}

	// Priority 1: stored ID from state
	if idx := r.findSchemaIndex(schemas, storedID); idx >= 0 {
		if id, ok := schemas[idx]["id"].(string); ok {
			addCandidate(id)
		}
	}

	// Priority 2: user-provided schema_id
	if idx := r.findSchemaIndex(schemas, schemaID); idx >= 0 {
		if id, ok := schemas[idx]["id"].(string); ok {
			addCandidate(id)
		}
	}

	// Priority 3: URL content match
	if !schemaAttr.IsNull() && !schemaAttr.IsUnknown() {
		if schemaURL, err := r.encodeSchema(schemaAttr.ValueString()); err == nil {
			if idx := r.findSchemaByURL(schemas, schemaURL); idx >= 0 {
				if id, ok := schemas[idx]["id"].(string); ok {
					addCandidate(id)
				}
			}
		}
	}

	// Priority 4: try storedID and schemaID directly even if not found in schema list
	// (the API may accept them even if GetProject doesn't list them yet)
	addCandidate(storedID)
	addCandidate(schemaID)

	return candidates
}

// schemaIDList extracts all schema IDs from a schema list for diagnostic messages.
func schemaIDList(schemas []map[string]interface{}) []string {
	ids := make([]string, 0, len(schemas))
	for _, s := range schemas {
		if id, ok := s["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// Note: ImportState is intentionally NOT implemented.
// Identity schemas in Ory Network are immutable and cannot be modified or reliably imported.
// Existing schemas created via the UI or API should be recreated in Terraform.
