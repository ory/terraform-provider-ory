package workspaceapikey

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
	_ resource.Resource                = &WorkspaceAPIKeyResource{}
	_ resource.ResourceWithConfigure   = &WorkspaceAPIKeyResource{}
	_ resource.ResourceWithImportState = &WorkspaceAPIKeyResource{}
)

// NewResource returns a new WorkspaceAPIKey resource.
func NewResource() resource.Resource {
	return &WorkspaceAPIKeyResource{}
}

// WorkspaceAPIKeyResource defines the resource implementation.
type WorkspaceAPIKeyResource struct {
	client *client.OryClient
}

// WorkspaceAPIKeyResourceModel describes the resource data model.
type WorkspaceAPIKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	Value       types.String `tfsdk:"value"`
	OwnerID     types.String `tfsdk:"owner_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *WorkspaceAPIKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_api_key"
}

func (r *WorkspaceAPIKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network workspace API key (import-only).",
		MarkdownDescription: `
Manages an Ory Network workspace API key (import-only).

Workspace API keys are used to authenticate API requests scoped to a workspace.

~> **Import-Only Resource:** Workspace API keys can only be created through the
[Ory Console](https://console.ory.sh). Use this resource to import existing
workspace API keys into Terraform and track their lifecycle. The Ory API does
not support managing workspace API keys when authenticating with another workspace
API key.

~> **Warning:** Destroying this resource only removes it from Terraform state.
The API key will continue to exist and remain valid in Ory Network. If you
delete the key that the Terraform provider itself uses to authenticate, the
provider will stop working. You must then update the provider configuration
with a new workspace API key.

## Usage

1. Create a workspace API key in the [Ory Console](https://console.ory.sh)
2. Save the key value securely (it is only shown once)
3. Import it into Terraform:

` + "```shell" + `
terraform import ory_workspace_api_key.main <workspace-id>/<key-id>
` + "```" + `

4. Add the resource block to your configuration:

` + "```hcl" + `
resource "ory_workspace_api_key" "main" {
  name = "My API Key"
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
			"name": schema.StringAttribute{
				Description: "A human-readable name for the API key.",
				Required:    true,
			},
			"workspace_id": schema.StringAttribute{
				Description: "The workspace ID. If not set, uses the provider's workspace_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"expires_at": schema.StringAttribute{
				Description: "Expiration time of the API key in RFC3339 format.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"value": schema.StringAttribute{
				Description: "The API key value. Only available when created via the Ory Console; not returned on import or read.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"owner_id": schema.StringAttribute{
				Description: "The ID of the user who owns this API key.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "When the API key was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "When the API key was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *WorkspaceAPIKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WorkspaceAPIKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Workspace API keys cannot be created via the API when authenticating with
	// another workspace API key (which is the typical Terraform provider auth method).
	// They require an identity session (Ory Console login).
	resp.Diagnostics.AddError(
		"Workspace API Key Creation Not Supported",
		"Workspace API keys can only be created through the Ory Console (https://console.ory.sh). "+
			"To manage an existing workspace API key with Terraform, import it using:\n\n"+
			"  terraform import ory_workspace_api_key.<name> <workspace-id>/<key-id>",
	)
}

func (r *WorkspaceAPIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WorkspaceAPIKeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := state.WorkspaceID.ValueString()
	if workspaceID == "" {
		workspaceID = r.client.WorkspaceID()
	}

	// List all keys and find ours
	keys, err := r.client.ListWorkspaceAPIKeys(ctx, workspaceID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Workspace API Keys",
			"Could not list workspace API keys: "+err.Error(),
		)
		return
	}

	var found *ory.WorkspaceApiKey
	for i := range keys {
		if keys[i].GetId() == state.ID.ValueString() {
			found = &keys[i]
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.GetName())
	state.WorkspaceID = types.StringValue(found.GetWorkspaceId())
	state.OwnerID = types.StringValue(found.GetOwnerId())

	if createdAt := found.GetCreatedAt(); !createdAt.IsZero() {
		state.CreatedAt = types.StringValue(createdAt.Format(time.RFC3339))
	}

	if updatedAt := found.GetUpdatedAt(); !updatedAt.IsZero() {
		state.UpdatedAt = types.StringValue(updatedAt.Format(time.RFC3339))
	}

	if expiresAt := found.GetExpiresAt(); !expiresAt.IsZero() {
		state.ExpiresAt = types.StringValue(expiresAt.Format(time.RFC3339))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WorkspaceAPIKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// There is no update API for workspace API keys.
	// Re-read and preserve existing state.
	var plan WorkspaceAPIKeyResourceModel
	var state WorkspaceAPIKeyResourceModel

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
	plan.ExpiresAt = state.ExpiresAt
	plan.UpdatedAt = state.UpdatedAt
	if plan.WorkspaceID.IsNull() || plan.WorkspaceID.IsUnknown() {
		plan.WorkspaceID = state.WorkspaceID
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *WorkspaceAPIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Workspace API keys cannot be deleted via the API when authenticating with
	// another workspace API key. Just remove from state without any API call.
	resp.Diagnostics.AddWarning(
		"Workspace API Key Not Deleted",
		"Workspace API keys cannot be deleted via the API when authenticating with a workspace API key. "+
			"The key has been removed from Terraform state but still exists and remains valid in Ory Network. "+
			"If this is the key used by the Terraform provider to authenticate, deleting it from the "+
			"Ory Console will cause the provider to stop working.",
	)
}

func (r *WorkspaceAPIKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: workspace_id/key_id
	id := req.ID

	if !strings.Contains(id, "/") {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format workspace_id/key_id, got: "+id,
		)
		return
	}

	parts := strings.SplitN(id, "/", 2)
	workspaceID := parts[0]
	keyID := parts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), keyID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
}
