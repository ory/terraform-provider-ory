package oidcdynamicclient

import (
	"context"
	"fmt"

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
	_ resource.Resource                = &OIDCDynamicClientResource{}
	_ resource.ResourceWithConfigure   = &OIDCDynamicClientResource{}
	_ resource.ResourceWithImportState = &OIDCDynamicClientResource{}
)

// NewResource returns a new OIDC Dynamic Client resource.
func NewResource() resource.Resource {
	return &OIDCDynamicClientResource{}
}

// OIDCDynamicClientResource defines the resource implementation.
type OIDCDynamicClientResource struct {
	client *client.OryClient
}

// OIDCDynamicClientResourceModel describes the resource data model.
type OIDCDynamicClientResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	ClientID                types.String `tfsdk:"client_id"`
	ClientSecret            types.String `tfsdk:"client_secret"`
	RegistrationAccessToken types.String `tfsdk:"registration_access_token"`
	RegistrationClientURI   types.String `tfsdk:"registration_client_uri"`
	ClientName              types.String `tfsdk:"client_name"`
	GrantTypes              types.List   `tfsdk:"grant_types"`
	ResponseTypes           types.List   `tfsdk:"response_types"`
	Scope                   types.String `tfsdk:"scope"`
	RedirectURIs            types.List   `tfsdk:"redirect_uris"`
	TokenEndpointAuthMethod types.String `tfsdk:"token_endpoint_auth_method"`
}

func (r *OIDCDynamicClientResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_dynamic_client"
}

func (r *OIDCDynamicClientResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network OIDC Dynamic Client via RFC 7591 Dynamic Client Registration.",
		MarkdownDescription: `
Manages an Ory Network OIDC Dynamic Client via RFC 7591 (Dynamic Client Registration).

Dynamic clients are registered without prior authentication and receive a
registration access token. After creation, the provider manages the client via the
Ory admin API, so the registration access token does not need to be preserved for
Terraform to function.

**Prerequisite:** Dynamic client registration must be enabled on your Ory project.

**Important:** The ` + "`client_secret`" + `, ` + "`registration_access_token`" + `, and
` + "`registration_client_uri`" + ` are only returned when the client is created. They are
not available after import or subsequent reads.

## Example Usage

` + "```hcl" + `
resource "ory_oidc_dynamic_client" "app" {
  client_name    = "My Application"
  grant_types    = ["authorization_code", "refresh_token"]
  response_types = ["code"]
  scope          = "openid offline_access"
  redirect_uris  = ["https://app.example.com/callback"]
}

output "client_id" {
  value = ory_oidc_dynamic_client.app.client_id
}

output "client_secret" {
  value     = ory_oidc_dynamic_client.app.client_secret
  sensitive = true
}
` + "```" + `

## Import

OIDC dynamic clients can be imported using their client ID:

` + "```shell" + `
terraform import ory_oidc_dynamic_client.app <client-id>
` + "```" + `

**Note:** When importing, ` + "`client_secret`" + `, ` + "`registration_access_token`" + `,
and ` + "`registration_client_uri`" + ` will not be available.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Internal Terraform ID (same as client_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_id": schema.StringAttribute{
				Description: "The OAuth2 client ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_secret": schema.StringAttribute{
				Description: "The OAuth2 client secret. Only returned on creation.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"registration_access_token": schema.StringAttribute{
				Description: "The registration access token for managing this client via RFC 7592. Only returned on creation.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"registration_client_uri": schema.StringAttribute{
				Description: "The URI for managing this client registration via RFC 7592.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_name": schema.StringAttribute{
				Description: "Human-readable name for the client.",
				Required:    true,
			},
			"grant_types": schema.ListAttribute{
				Description: "OAuth2 grant types: authorization_code, implicit, client_credentials, refresh_token.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"response_types": schema.ListAttribute{
				Description: "OAuth2 response types: code, token, id_token.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"scope": schema.StringAttribute{
				Description: "Space-separated list of OAuth2 scopes. If not specified, the API will set a default scope.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"redirect_uris": schema.ListAttribute{
				Description: "List of allowed redirect URIs for authorization code flow.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"token_endpoint_auth_method": schema.StringAttribute{
				Description: "Token endpoint authentication method: client_secret_post, client_secret_basic, private_key_jwt, none. Defaults to the API's default if not specified.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *OIDCDynamicClientResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OIDCDynamicClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OIDCDynamicClientResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient := ory.OAuth2Client{
		ClientName: ory.PtrString(plan.ClientName.ValueString()),
	}

	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		oauthClient.Scope = ory.PtrString(plan.Scope.ValueString())
	}

	if !plan.GrantTypes.IsNull() && !plan.GrantTypes.IsUnknown() {
		var grantTypes []string
		resp.Diagnostics.Append(plan.GrantTypes.ElementsAs(ctx, &grantTypes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.GrantTypes = grantTypes
	}

	if !plan.ResponseTypes.IsNull() && !plan.ResponseTypes.IsUnknown() {
		var responseTypes []string
		resp.Diagnostics.Append(plan.ResponseTypes.ElementsAs(ctx, &responseTypes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.ResponseTypes = responseTypes
	}

	if !plan.RedirectURIs.IsNull() && !plan.RedirectURIs.IsUnknown() {
		var redirectURIs []string
		resp.Diagnostics.Append(plan.RedirectURIs.ElementsAs(ctx, &redirectURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.RedirectUris = redirectURIs
	}

	if !plan.TokenEndpointAuthMethod.IsNull() && !plan.TokenEndpointAuthMethod.IsUnknown() {
		oauthClient.TokenEndpointAuthMethod = ory.PtrString(plan.TokenEndpointAuthMethod.ValueString())
	}

	created, err := r.client.CreateOIDCDynamicClient(ctx, oauthClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating OIDC Dynamic Client",
			"Could not create OIDC dynamic client: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(created.GetClientId())
	plan.ClientID = types.StringValue(created.GetClientId())
	plan.ClientName = types.StringValue(created.GetClientName())
	plan.Scope = types.StringValue(created.GetScope())

	if created.ClientSecret != nil && *created.ClientSecret != "" {
		plan.ClientSecret = types.StringValue(*created.ClientSecret)
	} else {
		plan.ClientSecret = types.StringValue("")
	}

	if created.RegistrationAccessToken != nil && *created.RegistrationAccessToken != "" {
		plan.RegistrationAccessToken = types.StringValue(*created.RegistrationAccessToken)
	} else {
		plan.RegistrationAccessToken = types.StringValue("")
	}

	if created.RegistrationClientUri != nil && *created.RegistrationClientUri != "" {
		plan.RegistrationClientURI = types.StringValue(*created.RegistrationClientUri)
	} else {
		plan.RegistrationClientURI = types.StringValue("")
	}

	if created.TokenEndpointAuthMethod != nil {
		plan.TokenEndpointAuthMethod = types.StringValue(*created.TokenEndpointAuthMethod)
	}

	if len(created.GrantTypes) > 0 {
		grantTypesList, diags := types.ListValueFrom(ctx, types.StringType, created.GrantTypes)
		resp.Diagnostics.Append(diags...)
		plan.GrantTypes = grantTypesList
	}

	if len(created.ResponseTypes) > 0 {
		responseTypesList, diags := types.ListValueFrom(ctx, types.StringType, created.ResponseTypes)
		resp.Diagnostics.Append(diags...)
		plan.ResponseTypes = responseTypesList
	}

	if len(created.RedirectUris) > 0 {
		redirectList, diags := types.ListValueFrom(ctx, types.StringType, created.RedirectUris)
		resp.Diagnostics.Append(diags...)
		plan.RedirectURIs = redirectList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *OIDCDynamicClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OIDCDynamicClientResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient, err := r.client.GetOIDCDynamicClient(ctx, state.ClientID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading OIDC Dynamic Client",
			"Could not read OIDC dynamic client "+state.ClientID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.ClientName = types.StringValue(oauthClient.GetClientName())
	state.Scope = types.StringValue(oauthClient.GetScope())

	if oauthClient.TokenEndpointAuthMethod != nil {
		state.TokenEndpointAuthMethod = types.StringValue(*oauthClient.TokenEndpointAuthMethod)
	}

	if len(oauthClient.GrantTypes) > 0 {
		grantTypesList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.GrantTypes)
		resp.Diagnostics.Append(diags...)
		state.GrantTypes = grantTypesList
	}

	if len(oauthClient.ResponseTypes) > 0 {
		responseTypesList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.ResponseTypes)
		resp.Diagnostics.Append(diags...)
		state.ResponseTypes = responseTypesList
	}

	if len(oauthClient.RedirectUris) > 0 {
		redirectList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.RedirectUris)
		resp.Diagnostics.Append(diags...)
		state.RedirectURIs = redirectList
	}

	if oauthClient.RegistrationClientUri != nil && *oauthClient.RegistrationClientUri != "" {
		state.RegistrationClientURI = types.StringValue(*oauthClient.RegistrationClientUri)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OIDCDynamicClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OIDCDynamicClientResourceModel
	var state OIDCDynamicClientResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient := ory.OAuth2Client{
		ClientId:   ory.PtrString(state.ClientID.ValueString()),
		ClientName: ory.PtrString(plan.ClientName.ValueString()),
	}

	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		oauthClient.Scope = ory.PtrString(plan.Scope.ValueString())
	}

	if !plan.GrantTypes.IsNull() && !plan.GrantTypes.IsUnknown() {
		var grantTypes []string
		resp.Diagnostics.Append(plan.GrantTypes.ElementsAs(ctx, &grantTypes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.GrantTypes = grantTypes
	}

	if !plan.ResponseTypes.IsNull() && !plan.ResponseTypes.IsUnknown() {
		var responseTypes []string
		resp.Diagnostics.Append(plan.ResponseTypes.ElementsAs(ctx, &responseTypes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.ResponseTypes = responseTypes
	}

	if !plan.RedirectURIs.IsNull() && !plan.RedirectURIs.IsUnknown() {
		var redirectURIs []string
		resp.Diagnostics.Append(plan.RedirectURIs.ElementsAs(ctx, &redirectURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.RedirectUris = redirectURIs
	}

	if !plan.TokenEndpointAuthMethod.IsNull() && !plan.TokenEndpointAuthMethod.IsUnknown() {
		oauthClient.TokenEndpointAuthMethod = ory.PtrString(plan.TokenEndpointAuthMethod.ValueString())
	}

	updated, err := r.client.UpdateOIDCDynamicClient(ctx, state.ClientID.ValueString(), oauthClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating OIDC Dynamic Client",
			"Could not update OIDC dynamic client: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.ClientID = state.ClientID
	plan.ClientSecret = state.ClientSecret
	plan.RegistrationAccessToken = state.RegistrationAccessToken
	plan.ClientName = types.StringValue(updated.GetClientName())
	plan.Scope = types.StringValue(updated.GetScope())

	if updated.TokenEndpointAuthMethod != nil {
		plan.TokenEndpointAuthMethod = types.StringValue(*updated.TokenEndpointAuthMethod)
	}

	if len(updated.GrantTypes) > 0 {
		grantTypesList, diags := types.ListValueFrom(ctx, types.StringType, updated.GrantTypes)
		resp.Diagnostics.Append(diags...)
		plan.GrantTypes = grantTypesList
	}

	if len(updated.ResponseTypes) > 0 {
		responseTypesList, diags := types.ListValueFrom(ctx, types.StringType, updated.ResponseTypes)
		resp.Diagnostics.Append(diags...)
		plan.ResponseTypes = responseTypesList
	}

	if len(updated.RedirectUris) > 0 {
		redirectList, diags := types.ListValueFrom(ctx, types.StringType, updated.RedirectUris)
		resp.Diagnostics.Append(diags...)
		plan.RedirectURIs = redirectList
	}

	if updated.RegistrationClientUri != nil && *updated.RegistrationClientUri != "" {
		plan.RegistrationClientURI = types.StringValue(*updated.RegistrationClientUri)
	} else {
		plan.RegistrationClientURI = state.RegistrationClientURI
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *OIDCDynamicClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OIDCDynamicClientResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteOIDCDynamicClient(ctx, state.ClientID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting OIDC Dynamic Client",
			"Could not delete OIDC dynamic client: "+err.Error(),
		)
		return
	}
}

func (r *OIDCDynamicClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
