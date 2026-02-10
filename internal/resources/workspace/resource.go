package workspace

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &WorkspaceResource{}
	_ resource.ResourceWithConfigure   = &WorkspaceResource{}
	_ resource.ResourceWithImportState = &WorkspaceResource{}
)

// NewResource returns a new Workspace resource.
func NewResource() resource.Resource {
	return &WorkspaceResource{}
}

// WorkspaceResource defines the resource implementation.
type WorkspaceResource struct {
	client *client.OryClient
}

// WorkspaceResourceModel describes the resource data model.
type WorkspaceResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (r *WorkspaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *WorkspaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network workspace (import-only).",
		MarkdownDescription: `
Manages an Ory Network workspace.

Workspaces are organizational units that can contain multiple projects.

~> **Import-Only Resource:** Workspaces can only be created through the
[Ory Console](https://console.ory.sh). Use this resource to import existing
workspaces into Terraform and manage their configuration. The Ory API does
not support workspace deletion, so destroying this resource will only remove
it from Terraform state.

## Usage

1. Create a workspace in the [Ory Console](https://console.ory.sh)
2. Get the workspace ID from the URL or API
3. Import it into Terraform:

` + "```shell" + `
terraform import ory_workspace.main <workspace-id>
` + "```" + `

4. Add the resource block to your configuration:

` + "```hcl" + `
resource "ory_workspace" "main" {
  name = "My Workspace"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the workspace.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the workspace.",
				Required:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the workspace was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the workspace was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *WorkspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Workspaces cannot be created via workspace API keys - they require account-level access
	// which is only available through the Ory Console
	resp.Diagnostics.AddError(
		"Workspace Creation Not Supported",
		"Workspaces can only be created through the Ory Console (https://console.ory.sh). "+
			"To manage an existing workspace with Terraform, import it using:\n\n"+
			"  terraform import ory_workspace.<name> <workspace-id>",
	)
}

func (r *WorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WorkspaceResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := r.client.GetWorkspace(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Workspace",
			"Could not read workspace ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Name = types.StringValue(workspace.GetName())
	state.CreatedAt = types.StringValue(workspace.CreatedAt.String())
	state.UpdatedAt = types.StringValue(workspace.UpdatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WorkspaceResourceModel
	var state WorkspaceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspace, err := r.client.UpdateWorkspace(ctx, state.ID.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Workspace",
			"Could not update workspace: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.Name = types.StringValue(workspace.GetName())
	plan.CreatedAt = types.StringValue(workspace.CreatedAt.String())
	plan.UpdatedAt = types.StringValue(workspace.UpdatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *WorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Ory API does not support workspace deletion
	// Just remove from state without any API call
	resp.Diagnostics.AddWarning(
		"Workspace Not Deleted",
		"Ory Network does not support workspace deletion. The workspace has been removed from Terraform state but still exists in Ory Network.",
	)
}

func (r *WorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
