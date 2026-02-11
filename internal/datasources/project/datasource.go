package project

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ datasource.DataSource              = &ProjectDataSource{}
	_ datasource.DataSourceWithConfigure = &ProjectDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

type ProjectDataSource struct {
	client *client.OryClient
}

type ProjectDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	State       types.String `tfsdk:"state"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

func (d *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an Ory project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Project ID to look up. If not specified, uses the provider's project_id.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The project name.",
				Computed:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The project slug.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "The project state.",
				Computed:    true,
			},
			"workspace_id": schema.StringAttribute{
				Description: "The workspace ID the project belongs to.",
				Computed:    true,
			},
		},
	}
}

func (d *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := data.ID.ValueString()
	if projectID == "" {
		projectID = d.client.ProjectID()
	}

	if projectID == "" {
		resp.Diagnostics.AddError("Missing Project ID",
			"Either specify 'id' in the data source or configure 'project_id' in the provider.")
		return
	}

	project, err := d.client.GetProject(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Project", err.Error())
		return
	}

	data.ID = types.StringValue(project.Id)
	data.Name = types.StringValue(project.Name)
	data.Slug = types.StringValue(project.Slug)
	data.State = types.StringValue(project.State)
	if project.WorkspaceId.IsSet() && project.WorkspaceId.Get() != nil {
		data.WorkspaceID = types.StringValue(*project.WorkspaceId.Get())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
