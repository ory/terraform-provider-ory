package organization

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ datasource.DataSource              = &OrganizationDataSource{}
	_ datasource.DataSourceWithConfigure = &OrganizationDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

type OrganizationDataSource struct {
	client *client.OryClient
}

type OrganizationDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Label     types.String `tfsdk:"label"`
	Domains   types.List   `tfsdk:"domains"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (d *OrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *OrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an Ory organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The organization ID to look up.",
				Required:    true,
			},
			"project_id": schema.StringAttribute{
				Description: "The project ID. If not set, uses the provider's project_id.",
				Optional:    true,
				Computed:    true,
			},
			"label": schema.StringAttribute{
				Description: "Human-readable organization name.",
				Computed:    true,
			},
			"domains": schema.ListAttribute{
				Description: "List of SSO domains for this organization.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the organization was created.",
				Computed:    true,
			},
		},
	}
}

func (d *OrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := d.resolveProjectID(data.ProjectID)
	if projectID == "" {
		resp.Diagnostics.AddError("Missing Project ID",
			"Either specify 'project_id' in the data source or configure 'project_id' in the provider.")
		return
	}

	org, err := d.client.GetOrganization(ctx, projectID, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Organization", err.Error())
		return
	}

	data.ID = types.StringValue(org.GetId())
	data.ProjectID = types.StringValue(projectID)
	data.Label = types.StringValue(org.GetLabel())
	data.CreatedAt = types.StringValue(org.CreatedAt.String())

	if len(org.Domains) > 0 {
		domainList, diags := types.ListValueFrom(ctx, types.StringType, org.Domains)
		resp.Diagnostics.Append(diags...)
		data.Domains = domainList
	} else {
		data.Domains = types.ListValueMust(types.StringType, []attr.Value{})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *OrganizationDataSource) resolveProjectID(tfProjectID types.String) string {
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() {
		return tfProjectID.ValueString()
	}
	return d.client.ProjectID()
}
