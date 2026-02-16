package oauth2client

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ datasource.DataSource              = &OAuth2ClientDataSource{}
	_ datasource.DataSourceWithConfigure = &OAuth2ClientDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &OAuth2ClientDataSource{}
}

type OAuth2ClientDataSource struct {
	client *client.OryClient
}

type OAuth2ClientDataSourceModel struct {
	ID                      types.String `tfsdk:"id"`
	ClientName              types.String `tfsdk:"client_name"`
	GrantTypes              types.List   `tfsdk:"grant_types"`
	ResponseTypes           types.List   `tfsdk:"response_types"`
	Scope                   types.String `tfsdk:"scope"`
	RedirectURIs            types.List   `tfsdk:"redirect_uris"`
	TokenEndpointAuthMethod types.String `tfsdk:"token_endpoint_auth_method"`
	Audience                types.List   `tfsdk:"audience"`
}

func (d *OAuth2ClientDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth2_client"
}

func (d *OAuth2ClientDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an Ory OAuth2 client.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The OAuth2 client ID to look up.",
				Required:    true,
			},
			"client_name": schema.StringAttribute{
				Description: "The OAuth2 client name.",
				Computed:    true,
			},
			"grant_types": schema.ListAttribute{
				Description: "The grant types allowed for this client.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"response_types": schema.ListAttribute{
				Description: "The response types allowed for this client.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"scope": schema.StringAttribute{
				Description: "The scope of the client.",
				Computed:    true,
			},
			"redirect_uris": schema.ListAttribute{
				Description: "The redirect URIs of the client.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"token_endpoint_auth_method": schema.StringAttribute{
				Description: "The token endpoint authentication method.",
				Computed:    true,
			},
			"audience": schema.ListAttribute{
				Description: "The audience of the client.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *OAuth2ClientDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OAuth2ClientDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OAuth2ClientDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient, err := d.client.GetOAuth2Client(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading OAuth2 Client", err.Error())
		return
	}

	data.ID = types.StringValue(oauthClient.GetClientId())
	data.ClientName = types.StringValue(oauthClient.GetClientName())
	data.Scope = types.StringValue(oauthClient.GetScope())

	if oauthClient.TokenEndpointAuthMethod != nil {
		data.TokenEndpointAuthMethod = types.StringValue(*oauthClient.TokenEndpointAuthMethod)
	}

	if len(oauthClient.GrantTypes) > 0 {
		grantTypesList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.GrantTypes)
		resp.Diagnostics.Append(diags...)
		data.GrantTypes = grantTypesList
	}

	if len(oauthClient.ResponseTypes) > 0 {
		responseTypesList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.ResponseTypes)
		resp.Diagnostics.Append(diags...)
		data.ResponseTypes = responseTypesList
	}

	if len(oauthClient.RedirectUris) > 0 {
		redirectList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.RedirectUris)
		resp.Diagnostics.Append(diags...)
		data.RedirectURIs = redirectList
	}

	if len(oauthClient.Audience) > 0 {
		audienceList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.Audience)
		resp.Diagnostics.Append(diags...)
		data.Audience = audienceList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
