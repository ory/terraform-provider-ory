package oauth2client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &OAuth2ClientResource{}
	_ resource.ResourceWithConfigure   = &OAuth2ClientResource{}
	_ resource.ResourceWithImportState = &OAuth2ClientResource{}
)

// NewResource returns a new OAuth2Client resource.
func NewResource() resource.Resource {
	return &OAuth2ClientResource{}
}

// OAuth2ClientResource defines the resource implementation.
type OAuth2ClientResource struct {
	client *client.OryClient
}

// OAuth2ClientResourceModel describes the resource data model.
type OAuth2ClientResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	ClientID                types.String `tfsdk:"client_id"`
	ClientSecret            types.String `tfsdk:"client_secret"`
	ClientName              types.String `tfsdk:"client_name"`
	GrantTypes              types.List   `tfsdk:"grant_types"`
	ResponseTypes           types.List   `tfsdk:"response_types"`
	Scope                   types.String `tfsdk:"scope"`
	Audience                types.List   `tfsdk:"audience"`
	RedirectURIs            types.List   `tfsdk:"redirect_uris"`
	PostLogoutRedirectURIs  types.List   `tfsdk:"post_logout_redirect_uris"`
	TokenEndpointAuthMethod types.String `tfsdk:"token_endpoint_auth_method"`
	Metadata                types.String `tfsdk:"metadata"`
}

func (r *OAuth2ClientResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth2_client"
}

func (r *OAuth2ClientResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network OAuth2 client.",
		MarkdownDescription: `
Manages an Ory Network OAuth2 client.

OAuth2 clients are used for machine-to-machine authentication or user-facing
OAuth2 flows.

**Important:** The ` + "`client_secret`" + ` is only returned when the client is created.
Store it securely immediately after creation.

## Example Usage

` + "```hcl" + `
resource "ory_oauth2_client" "api" {
  client_name                 = "API Client"
  grant_types                 = ["client_credentials"]
  scope                       = "read write"
  token_endpoint_auth_method  = "client_secret_post"
}

output "client_id" {
  value = ory_oauth2_client.api.client_id
}

output "client_secret" {
  value     = ory_oauth2_client.api.client_secret
  sensitive = true
}
` + "```" + `

## Import

OAuth2 clients can be imported using their client ID:

` + "```shell" + `
terraform import ory_oauth2_client.api <client-id>
` + "```" + `

**Note:** When importing, the ` + "`client_secret`" + ` will not be available.
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
			"audience": schema.ListAttribute{
				Description: "List of allowed audiences for tokens.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"redirect_uris": schema.ListAttribute{
				Description: "List of allowed redirect URIs for authorization code flow.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"post_logout_redirect_uris": schema.ListAttribute{
				Description: "List of allowed post-logout redirect URIs for OpenID Connect logout.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"token_endpoint_auth_method": schema.StringAttribute{
				Description: "Token endpoint authentication method: client_secret_post, client_secret_basic, private_key_jwt, none.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("client_secret_post"),
			},
			"metadata": schema.StringAttribute{
				Description: "Custom metadata as JSON string.",
				Optional:    true,
			},
		},
	}
}

func (r *OAuth2ClientResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OAuth2ClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OAuth2ClientResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient := ory.OAuth2Client{
		ClientName: ory.PtrString(plan.ClientName.ValueString()),
		Scope:      ory.PtrString(plan.Scope.ValueString()),
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

	if !plan.Audience.IsNull() && !plan.Audience.IsUnknown() {
		var audience []string
		resp.Diagnostics.Append(plan.Audience.ElementsAs(ctx, &audience, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.Audience = audience
	}

	if !plan.RedirectURIs.IsNull() && !plan.RedirectURIs.IsUnknown() {
		var redirectURIs []string
		resp.Diagnostics.Append(plan.RedirectURIs.ElementsAs(ctx, &redirectURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.RedirectUris = redirectURIs
	}

	if !plan.PostLogoutRedirectURIs.IsNull() && !plan.PostLogoutRedirectURIs.IsUnknown() {
		var postLogoutURIs []string
		resp.Diagnostics.Append(plan.PostLogoutRedirectURIs.ElementsAs(ctx, &postLogoutURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.PostLogoutRedirectUris = postLogoutURIs
	}

	if !plan.TokenEndpointAuthMethod.IsNull() && !plan.TokenEndpointAuthMethod.IsUnknown() {
		oauthClient.TokenEndpointAuthMethod = ory.PtrString(plan.TokenEndpointAuthMethod.ValueString())
	}

	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(plan.Metadata.ValueString()), &metadata); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Metadata JSON",
				"Could not parse metadata as JSON: "+err.Error(),
			)
			return
		}
		oauthClient.Metadata = metadata
	}

	created, err := r.client.CreateOAuth2Client(ctx, oauthClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating OAuth2 Client",
			"Could not create OAuth2 client: "+err.Error(),
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
		// Public clients (token_endpoint_auth_method = "none") don't have a secret
		plan.ClientSecret = types.StringValue("")
	}

	if created.TokenEndpointAuthMethod != nil {
		plan.TokenEndpointAuthMethod = types.StringValue(*created.TokenEndpointAuthMethod)
	}

	if len(created.GrantTypes) > 0 {
		grantTypesList, diags := types.ListValueFrom(ctx, types.StringType, created.GrantTypes)
		resp.Diagnostics.Append(diags...)
		plan.GrantTypes = grantTypesList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *OAuth2ClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OAuth2ClientResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient, err := r.client.GetOAuth2Client(ctx, state.ClientID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading OAuth2 Client",
			"Could not read OAuth2 client "+state.ClientID.ValueString()+": "+err.Error(),
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

	if len(oauthClient.Audience) > 0 {
		audienceList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.Audience)
		resp.Diagnostics.Append(diags...)
		state.Audience = audienceList
	}

	if len(oauthClient.RedirectUris) > 0 {
		redirectList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.RedirectUris)
		resp.Diagnostics.Append(diags...)
		state.RedirectURIs = redirectList
	}

	if len(oauthClient.PostLogoutRedirectUris) > 0 {
		postLogoutList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.PostLogoutRedirectUris)
		resp.Diagnostics.Append(diags...)
		state.PostLogoutRedirectURIs = postLogoutList
	}

	// Only set metadata if it's non-empty
	// The API returns {} (empty object) by default, but we want null in Terraform state
	// when metadata wasn't specified in the config
	if len(oauthClient.Metadata) > 0 {
		metadataJSON, err := json.Marshal(oauthClient.Metadata)
		if err == nil {
			state.Metadata = types.StringValue(string(metadataJSON))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OAuth2ClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OAuth2ClientResourceModel
	var state OAuth2ClientResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oauthClient := ory.OAuth2Client{
		ClientId:   ory.PtrString(state.ClientID.ValueString()),
		ClientName: ory.PtrString(plan.ClientName.ValueString()),
		Scope:      ory.PtrString(plan.Scope.ValueString()),
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

	if !plan.Audience.IsNull() && !plan.Audience.IsUnknown() {
		var audience []string
		resp.Diagnostics.Append(plan.Audience.ElementsAs(ctx, &audience, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.Audience = audience
	}

	if !plan.RedirectURIs.IsNull() && !plan.RedirectURIs.IsUnknown() {
		var redirectURIs []string
		resp.Diagnostics.Append(plan.RedirectURIs.ElementsAs(ctx, &redirectURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.RedirectUris = redirectURIs
	}

	if !plan.PostLogoutRedirectURIs.IsNull() && !plan.PostLogoutRedirectURIs.IsUnknown() {
		var postLogoutURIs []string
		resp.Diagnostics.Append(plan.PostLogoutRedirectURIs.ElementsAs(ctx, &postLogoutURIs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.PostLogoutRedirectUris = postLogoutURIs
	}

	if !plan.TokenEndpointAuthMethod.IsNull() && !plan.TokenEndpointAuthMethod.IsUnknown() {
		oauthClient.TokenEndpointAuthMethod = ory.PtrString(plan.TokenEndpointAuthMethod.ValueString())
	}

	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(plan.Metadata.ValueString()), &metadata); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Metadata JSON",
				"Could not parse metadata as JSON: "+err.Error(),
			)
			return
		}
		oauthClient.Metadata = metadata
	}

	updated, err := r.client.UpdateOAuth2Client(ctx, state.ClientID.ValueString(), oauthClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating OAuth2 Client",
			"Could not update OAuth2 client: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.ClientID = state.ClientID
	plan.ClientSecret = state.ClientSecret
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

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *OAuth2ClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OAuth2ClientResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteOAuth2Client(ctx, state.ClientID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting OAuth2 Client",
			"Could not delete OAuth2 client: "+err.Error(),
		)
		return
	}
}

func (r *OAuth2ClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
