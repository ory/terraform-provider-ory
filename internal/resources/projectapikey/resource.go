package projectapikey

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ProjectAPIKeyResource{}
	_ resource.ResourceWithConfigure   = &ProjectAPIKeyResource{}
	_ resource.ResourceWithImportState = &ProjectAPIKeyResource{}
)

// NewResource returns a new ProjectAPIKey resource.
func NewResource() resource.Resource {
	return &ProjectAPIKeyResource{}
}

// ProjectAPIKeyResource defines the resource implementation.
type ProjectAPIKeyResource struct {
	client *client.OryClient
}

// ProjectAPIKeyResourceModel describes the resource data model.
type ProjectAPIKeyResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	Value     types.String `tfsdk:"value"`
	CreatedAt types.String `tfsdk:"created_at"`
	OwnerID   types.String `tfsdk:"owner_id"`
}

func (r *ProjectAPIKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_api_key"
}

func (r *ProjectAPIKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network project API key.",
		MarkdownDescription: `
Manages an Ory Network project API key.

API keys are used to authenticate API requests to a specific project.

**Important:** The ` + "`value`" + ` (the actual API key) is only returned when the key is created.
Store it securely immediately after creation.

## Example Usage

` + "```hcl" + `
resource "ory_project_api_key" "backend" {
  name = "Backend API Key"
}

output "api_key" {
  value     = ory_project_api_key.backend.value
  sensitive = true
}
` + "```" + `

## Example with Expiration

` + "```hcl" + `
resource "ory_project_api_key" "temporary" {
  name       = "Temporary Key"
  expires_at = "2025-12-31T23:59:59Z"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The API key ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "The project ID. If not set, uses the provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "A descriptive name for the API key.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Description: "Optional expiration time in RFC3339 format (e.g., 2025-12-31T23:59:59Z).",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				Description: "The API key value. Only returned on creation.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "When the API key was created.",
				Computed:    true,
			},
			"owner_id": schema.StringAttribute{
				Description: "The ID of the user who owns this API key.",
				Computed:    true,
			},
		},
	}
}

func (r *ProjectAPIKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = oryClient
}

func (r *ProjectAPIKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProjectAPIKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use provider's project ID if not specified
	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	body := ory.CreateProjectApiKeyRequest{
		Name: plan.Name.ValueString(),
	}

	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		expiresAt, err := time.Parse(time.RFC3339, plan.ExpiresAt.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid expires_at Format",
				"Could not parse expires_at as RFC3339 timestamp: "+err.Error(),
			)
			return
		}
		body.ExpiresAt = &expiresAt
	}

	key, err := r.client.CreateProjectAPIKey(ctx, projectID, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Project API Key",
			"Could not create project API key: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(key.Id)
	plan.ProjectID = types.StringValue(projectID)
	plan.Name = types.StringValue(key.Name)
	plan.OwnerID = types.StringValue(key.OwnerId)

	if key.Value != nil {
		plan.Value = types.StringValue(*key.Value)
	}

	if key.CreatedAt != nil {
		plan.CreatedAt = types.StringValue(key.CreatedAt.Format(time.RFC3339))
	}

	if key.ExpiresAt != nil {
		plan.ExpiresAt = types.StringValue(key.ExpiresAt.Format(time.RFC3339))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectAPIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProjectAPIKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	// List all keys and find ours
	keys, err := r.client.ListProjectAPIKeys(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Project API Keys",
			"Could not list project API keys: "+err.Error(),
		)
		return
	}

	var found *ory.ProjectApiKey
	for i := range keys {
		if keys[i].Id == state.ID.ValueString() {
			found = &keys[i]
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.OwnerID = types.StringValue(found.OwnerId)

	if found.CreatedAt != nil {
		state.CreatedAt = types.StringValue(found.CreatedAt.Format(time.RFC3339))
	}

	if found.ExpiresAt != nil {
		state.ExpiresAt = types.StringValue(found.ExpiresAt.Format(time.RFC3339))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ProjectAPIKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// API keys cannot be updated, only the name could change but the API doesn't support that
	// This will be handled by RequiresReplace on the fields that matter
	var plan ProjectAPIKeyResourceModel
	var state ProjectAPIKeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve computed values from state
	plan.ID = state.ID
	plan.Value = state.Value
	plan.CreatedAt = state.CreatedAt
	plan.OwnerID = state.OwnerID
	if plan.ProjectID.IsNull() || plan.ProjectID.IsUnknown() {
		plan.ProjectID = state.ProjectID
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectAPIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProjectAPIKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	err := r.client.DeleteProjectAPIKey(ctx, projectID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Project API Key",
			"Could not delete project API key: "+err.Error(),
		)
		return
	}
}

func (r *ProjectAPIKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: project_id/key_id or just key_id (uses provider's project_id)
	id := req.ID
	var projectID, keyID string

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		projectID = parts[0]
		keyID = parts[1]
	} else {
		keyID = id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), keyID)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}
