package identity

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ datasource.DataSource              = &IdentityDataSource{}
	_ datasource.DataSourceWithConfigure = &IdentityDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &IdentityDataSource{}
}

type IdentityDataSource struct {
	client *client.OryClient
}

type IdentityDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	SchemaID       types.String `tfsdk:"schema_id"`
	SchemaURL      types.String `tfsdk:"schema_url"`
	State          types.String `tfsdk:"state"`
	Traits         types.String `tfsdk:"traits"`
	MetadataPublic types.String `tfsdk:"metadata_public"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (d *IdentityDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity"
}

func (d *IdentityDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an Ory identity.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The identity ID to look up.",
				Required:    true,
			},
			"schema_id": schema.StringAttribute{
				Description: "The identity schema ID.",
				Computed:    true,
			},
			"schema_url": schema.StringAttribute{
				Description: "The URL of the identity schema.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "The identity state (active or inactive).",
				Computed:    true,
			},
			"traits": schema.StringAttribute{
				Description: "Identity traits as a JSON string.",
				Computed:    true,
			},
			"metadata_public": schema.StringAttribute{
				Description: "Public metadata as a JSON string.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the identity was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the identity was last updated.",
				Computed:    true,
			},
		},
	}
}

func (d *IdentityDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IdentityDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IdentityDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	identity, err := d.client.GetIdentity(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Identity", err.Error())
		return
	}

	data.ID = types.StringValue(identity.GetId())
	data.SchemaID = types.StringValue(identity.GetSchemaId())
	data.SchemaURL = types.StringValue(identity.GetSchemaUrl())
	data.State = types.StringValue(identity.GetState())

	if identity.Traits != nil {
		traitsJSON, err := json.Marshal(identity.Traits)
		if err != nil {
			resp.Diagnostics.AddError("Error Serializing Traits",
				fmt.Sprintf("Could not serialize identity traits to JSON: %s", err.Error()))
			return
		}
		data.Traits = types.StringValue(string(traitsJSON))
	}

	if identity.MetadataPublic != nil {
		metadataJSON, err := json.Marshal(identity.MetadataPublic)
		if err != nil {
			resp.Diagnostics.AddError("Error Serializing Metadata",
				fmt.Sprintf("Could not serialize identity metadata_public to JSON: %s", err.Error()))
			return
		}
		data.MetadataPublic = types.StringValue(string(metadataJSON))
	}

	if identity.CreatedAt != nil {
		data.CreatedAt = types.StringValue(identity.CreatedAt.String())
	}
	if identity.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(identity.UpdatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
