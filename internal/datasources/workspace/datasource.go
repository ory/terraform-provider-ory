package workspace

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ datasource.DataSource              = &WorkspaceDataSource{}
	_ datasource.DataSourceWithConfigure = &WorkspaceDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &WorkspaceDataSource{}
}

type WorkspaceDataSource struct {
	client *client.OryClient
}

type WorkspaceDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *WorkspaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *WorkspaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an Ory workspace.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Workspace ID to look up. If not specified, uses the provider's workspace_id.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The workspace name.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the workspace was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the workspace was last updated.",
				Computed:    true,
			},
		},
	}
}

func (d *WorkspaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T", req.ProviderData))
		return
	}
	d.client = oryClient
}

func (d *WorkspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WorkspaceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := data.ID.ValueString()
	if workspaceID == "" {
		workspaceID = d.client.WorkspaceID()
	}

	if workspaceID == "" {
		resp.Diagnostics.AddError("Missing Workspace ID",
			"Either specify 'id' in the data source or configure 'workspace_id' in the provider.")
		return
	}

	workspace, err := d.client.GetWorkspace(ctx, workspaceID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Workspace", err.Error())
		return
	}

	data.ID = types.StringValue(workspace.GetId())
	data.Name = types.StringValue(workspace.GetName())
	data.CreatedAt = types.StringValue(workspace.CreatedAt.String())
	data.UpdatedAt = types.StringValue(workspace.UpdatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
