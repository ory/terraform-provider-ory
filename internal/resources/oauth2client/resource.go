package oauth2client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	AllowedCorsOrigins      types.List   `tfsdk:"allowed_cors_origins"`
	ClientURI               types.String `tfsdk:"client_uri"`
	LogoURI                 types.String `tfsdk:"logo_uri"`
	PolicyURI               types.String `tfsdk:"policy_uri"`
	TosURI                  types.String `tfsdk:"tos_uri"`
	FrontchannelLogoutURI   types.String `tfsdk:"frontchannel_logout_uri"`
	BackchannelLogoutURI    types.String `tfsdk:"backchannel_logout_uri"`
	AccessTokenStrategy     types.String `tfsdk:"access_token_strategy"`
	SkipConsent             types.Bool   `tfsdk:"skip_consent"`
	SkipLogoutConsent       types.Bool   `tfsdk:"skip_logout_consent"`
	SubjectType             types.String `tfsdk:"subject_type"`
	Contacts                types.List   `tfsdk:"contacts"`

	// Per-grant token lifespans
	AuthorizationCodeGrantAccessTokenLifespan    types.String `tfsdk:"authorization_code_grant_access_token_lifespan"`
	AuthorizationCodeGrantIdTokenLifespan        types.String `tfsdk:"authorization_code_grant_id_token_lifespan"`
	AuthorizationCodeGrantRefreshTokenLifespan   types.String `tfsdk:"authorization_code_grant_refresh_token_lifespan"`
	ClientCredentialsGrantAccessTokenLifespan    types.String `tfsdk:"client_credentials_grant_access_token_lifespan"`
	DeviceAuthorizationGrantAccessTokenLifespan  types.String `tfsdk:"device_authorization_grant_access_token_lifespan"`
	DeviceAuthorizationGrantIdTokenLifespan      types.String `tfsdk:"device_authorization_grant_id_token_lifespan"`
	DeviceAuthorizationGrantRefreshTokenLifespan types.String `tfsdk:"device_authorization_grant_refresh_token_lifespan"`
	ImplicitGrantAccessTokenLifespan             types.String `tfsdk:"implicit_grant_access_token_lifespan"`
	ImplicitGrantIdTokenLifespan                 types.String `tfsdk:"implicit_grant_id_token_lifespan"`
	JwtBearerGrantAccessTokenLifespan            types.String `tfsdk:"jwt_bearer_grant_access_token_lifespan"`
	RefreshTokenGrantAccessTokenLifespan         types.String `tfsdk:"refresh_token_grant_access_token_lifespan"`
	RefreshTokenGrantIdTokenLifespan             types.String `tfsdk:"refresh_token_grant_id_token_lifespan"`
	RefreshTokenGrantRefreshTokenLifespan        types.String `tfsdk:"refresh_token_grant_refresh_token_lifespan"`

	// OIDC fields
	JwksURI                   types.String `tfsdk:"jwks_uri"`
	UserinfoSignedResponseAlg types.String `tfsdk:"userinfo_signed_response_alg"`
	RequestObjectSigningAlg   types.String `tfsdk:"request_object_signing_alg"`

	// Logout session
	FrontchannelLogoutSessionRequired types.Bool `tfsdk:"frontchannel_logout_session_required"`
	BackchannelLogoutSessionRequired  types.Bool `tfsdk:"backchannel_logout_session_required"`
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
			"allowed_cors_origins": schema.ListAttribute{
				Description: "List of allowed CORS origins for this client.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"client_uri": schema.StringAttribute{
				Description: "URL of the client's homepage.",
				Optional:    true,
			},
			"logo_uri": schema.StringAttribute{
				Description: "URL of the client's logo.",
				Optional:    true,
			},
			"policy_uri": schema.StringAttribute{
				Description: "URL of the client's privacy policy.",
				Optional:    true,
			},
			"tos_uri": schema.StringAttribute{
				Description: "URL of the client's terms of service.",
				Optional:    true,
			},
			"frontchannel_logout_uri": schema.StringAttribute{
				Description: "OpenID Connect front-channel logout URI.",
				Optional:    true,
			},
			"backchannel_logout_uri": schema.StringAttribute{
				Description: "OpenID Connect back-channel logout URI.",
				Optional:    true,
			},
			"access_token_strategy": schema.StringAttribute{
				Description: "Access token strategy: jwt or opaque.",
				Optional:    true,
			},
			"skip_consent": schema.BoolAttribute{
				Description: "Skip the consent screen for this client. When true, the user is never asked to grant consent.",
				Optional:    true,
				Computed:    true,
			},
			"skip_logout_consent": schema.BoolAttribute{
				Description: "Skip the logout consent screen for this client. When true, the user is not asked to confirm logout.",
				Optional:    true,
				Computed:    true,
			},
			"subject_type": schema.StringAttribute{
				Description: "OpenID Connect subject type: public (same sub for all clients) or pairwise (unique sub per client).",
				Optional:    true,
				Computed:    true,
			},
			"contacts": schema.ListAttribute{
				Description: "List of contact email addresses for the client maintainers.",
				Optional:    true,
				ElementType: types.StringType,
			},

			// Per-grant token lifespans
			"authorization_code_grant_access_token_lifespan": schema.StringAttribute{
				Description: "Access token lifespan for authorization code grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"authorization_code_grant_id_token_lifespan": schema.StringAttribute{
				Description: "ID token lifespan for authorization code grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"authorization_code_grant_refresh_token_lifespan": schema.StringAttribute{
				Description: "Refresh token lifespan for authorization code grant (e.g., '720h').",
				Optional:    true,
			},
			"client_credentials_grant_access_token_lifespan": schema.StringAttribute{
				Description: "Access token lifespan for client credentials grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"device_authorization_grant_access_token_lifespan": schema.StringAttribute{
				Description: "Access token lifespan for device authorization grant (e.g., '1h').",
				Optional:    true,
			},
			"device_authorization_grant_id_token_lifespan": schema.StringAttribute{
				Description: "ID token lifespan for device authorization grant (e.g., '1h').",
				Optional:    true,
			},
			"device_authorization_grant_refresh_token_lifespan": schema.StringAttribute{
				Description: "Refresh token lifespan for device authorization grant (e.g., '720h').",
				Optional:    true,
			},
			"implicit_grant_access_token_lifespan": schema.StringAttribute{
				Description: "Access token lifespan for implicit grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"implicit_grant_id_token_lifespan": schema.StringAttribute{
				Description: "ID token lifespan for implicit grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"jwt_bearer_grant_access_token_lifespan": schema.StringAttribute{
				Description: "Access token lifespan for JWT bearer grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"refresh_token_grant_access_token_lifespan": schema.StringAttribute{
				Description: "Access token lifespan for refresh token grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"refresh_token_grant_id_token_lifespan": schema.StringAttribute{
				Description: "ID token lifespan for refresh token grant (e.g., '1h', '30m').",
				Optional:    true,
			},
			"refresh_token_grant_refresh_token_lifespan": schema.StringAttribute{
				Description: "Refresh token lifespan for refresh token grant (e.g., '720h').",
				Optional:    true,
			},

			// OIDC fields
			"jwks_uri": schema.StringAttribute{
				Description: "URL of the client's JSON Web Key Set for private_key_jwt authentication.",
				Optional:    true,
			},
			"userinfo_signed_response_alg": schema.StringAttribute{
				Description: "JWS algorithm for signing UserInfo responses (e.g., 'RS256', 'ES256').",
				Optional:    true,
			},
			"request_object_signing_alg": schema.StringAttribute{
				Description: "JWS algorithm for signing request objects (e.g., 'RS256', 'ES256').",
				Optional:    true,
			},

			// Logout session
			"frontchannel_logout_session_required": schema.BoolAttribute{
				Description: "Whether the client requires a session identifier in front-channel logout notifications.",
				Optional:    true,
				Computed:    true,
			},
			"backchannel_logout_session_required": schema.BoolAttribute{
				Description: "Whether the client requires a session identifier in back-channel logout notifications.",
				Optional:    true,
				Computed:    true,
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

	if !plan.AllowedCorsOrigins.IsNull() && !plan.AllowedCorsOrigins.IsUnknown() {
		var origins []string
		resp.Diagnostics.Append(plan.AllowedCorsOrigins.ElementsAs(ctx, &origins, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.AllowedCorsOrigins = origins
	}

	if !plan.ClientURI.IsNull() && !plan.ClientURI.IsUnknown() {
		oauthClient.ClientUri = ory.PtrString(plan.ClientURI.ValueString())
	}
	if !plan.LogoURI.IsNull() && !plan.LogoURI.IsUnknown() {
		oauthClient.LogoUri = ory.PtrString(plan.LogoURI.ValueString())
	}
	if !plan.PolicyURI.IsNull() && !plan.PolicyURI.IsUnknown() {
		oauthClient.PolicyUri = ory.PtrString(plan.PolicyURI.ValueString())
	}
	if !plan.TosURI.IsNull() && !plan.TosURI.IsUnknown() {
		oauthClient.TosUri = ory.PtrString(plan.TosURI.ValueString())
	}
	if !plan.FrontchannelLogoutURI.IsNull() && !plan.FrontchannelLogoutURI.IsUnknown() {
		oauthClient.FrontchannelLogoutUri = ory.PtrString(plan.FrontchannelLogoutURI.ValueString())
	}
	if !plan.BackchannelLogoutURI.IsNull() && !plan.BackchannelLogoutURI.IsUnknown() {
		oauthClient.BackchannelLogoutUri = ory.PtrString(plan.BackchannelLogoutURI.ValueString())
	}
	if !plan.AccessTokenStrategy.IsNull() && !plan.AccessTokenStrategy.IsUnknown() {
		oauthClient.AccessTokenStrategy = ory.PtrString(plan.AccessTokenStrategy.ValueString())
	}
	if !plan.SkipConsent.IsNull() && !plan.SkipConsent.IsUnknown() {
		oauthClient.SkipConsent = ory.PtrBool(plan.SkipConsent.ValueBool())
	}
	if !plan.SkipLogoutConsent.IsNull() && !plan.SkipLogoutConsent.IsUnknown() {
		oauthClient.SkipLogoutConsent = ory.PtrBool(plan.SkipLogoutConsent.ValueBool())
	}
	if !plan.SubjectType.IsNull() && !plan.SubjectType.IsUnknown() {
		oauthClient.SubjectType = ory.PtrString(plan.SubjectType.ValueString())
	}
	if !plan.Contacts.IsNull() && !plan.Contacts.IsUnknown() {
		var contacts []string
		resp.Diagnostics.Append(plan.Contacts.ElementsAs(ctx, &contacts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.Contacts = contacts
	}

	setNullableStringFromPlan(&oauthClient.AuthorizationCodeGrantAccessTokenLifespan, plan.AuthorizationCodeGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.AuthorizationCodeGrantIdTokenLifespan, plan.AuthorizationCodeGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.AuthorizationCodeGrantRefreshTokenLifespan, plan.AuthorizationCodeGrantRefreshTokenLifespan)
	setNullableStringFromPlan(&oauthClient.ClientCredentialsGrantAccessTokenLifespan, plan.ClientCredentialsGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.DeviceAuthorizationGrantAccessTokenLifespan, plan.DeviceAuthorizationGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.DeviceAuthorizationGrantIdTokenLifespan, plan.DeviceAuthorizationGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.DeviceAuthorizationGrantRefreshTokenLifespan, plan.DeviceAuthorizationGrantRefreshTokenLifespan)
	setNullableStringFromPlan(&oauthClient.ImplicitGrantAccessTokenLifespan, plan.ImplicitGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.ImplicitGrantIdTokenLifespan, plan.ImplicitGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.JwtBearerGrantAccessTokenLifespan, plan.JwtBearerGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.RefreshTokenGrantAccessTokenLifespan, plan.RefreshTokenGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.RefreshTokenGrantIdTokenLifespan, plan.RefreshTokenGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.RefreshTokenGrantRefreshTokenLifespan, plan.RefreshTokenGrantRefreshTokenLifespan)

	if !plan.JwksURI.IsNull() && !plan.JwksURI.IsUnknown() {
		oauthClient.JwksUri = ory.PtrString(plan.JwksURI.ValueString())
	}
	if !plan.UserinfoSignedResponseAlg.IsNull() && !plan.UserinfoSignedResponseAlg.IsUnknown() {
		oauthClient.UserinfoSignedResponseAlg = ory.PtrString(plan.UserinfoSignedResponseAlg.ValueString())
	}
	if !plan.RequestObjectSigningAlg.IsNull() && !plan.RequestObjectSigningAlg.IsUnknown() {
		oauthClient.RequestObjectSigningAlg = ory.PtrString(plan.RequestObjectSigningAlg.ValueString())
	}
	if !plan.FrontchannelLogoutSessionRequired.IsNull() && !plan.FrontchannelLogoutSessionRequired.IsUnknown() {
		oauthClient.FrontchannelLogoutSessionRequired = ory.PtrBool(plan.FrontchannelLogoutSessionRequired.ValueBool())
	}
	if !plan.BackchannelLogoutSessionRequired.IsNull() && !plan.BackchannelLogoutSessionRequired.IsUnknown() {
		oauthClient.BackchannelLogoutSessionRequired = ory.PtrBool(plan.BackchannelLogoutSessionRequired.ValueBool())
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

	if created.SkipConsent != nil {
		plan.SkipConsent = types.BoolValue(*created.SkipConsent)
	} else {
		plan.SkipConsent = types.BoolValue(false)
	}
	if created.SkipLogoutConsent != nil {
		plan.SkipLogoutConsent = types.BoolValue(*created.SkipLogoutConsent)
	} else {
		plan.SkipLogoutConsent = types.BoolValue(false)
	}
	if created.SubjectType != nil && *created.SubjectType != "" {
		plan.SubjectType = types.StringValue(*created.SubjectType)
	} else {
		plan.SubjectType = types.StringValue("public")
	}
	if created.FrontchannelLogoutSessionRequired != nil {
		plan.FrontchannelLogoutSessionRequired = types.BoolValue(*created.FrontchannelLogoutSessionRequired)
	} else {
		plan.FrontchannelLogoutSessionRequired = types.BoolValue(false)
	}
	if created.BackchannelLogoutSessionRequired != nil {
		plan.BackchannelLogoutSessionRequired = types.BoolValue(*created.BackchannelLogoutSessionRequired)
	} else {
		plan.BackchannelLogoutSessionRequired = types.BoolValue(false)
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

	if len(oauthClient.AllowedCorsOrigins) > 0 {
		originsList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.AllowedCorsOrigins)
		resp.Diagnostics.Append(diags...)
		state.AllowedCorsOrigins = originsList
	}
	if oauthClient.ClientUri != nil && *oauthClient.ClientUri != "" {
		state.ClientURI = types.StringValue(*oauthClient.ClientUri)
	}
	if oauthClient.LogoUri != nil && *oauthClient.LogoUri != "" {
		state.LogoURI = types.StringValue(*oauthClient.LogoUri)
	}
	if oauthClient.PolicyUri != nil && *oauthClient.PolicyUri != "" {
		state.PolicyURI = types.StringValue(*oauthClient.PolicyUri)
	}
	if oauthClient.TosUri != nil && *oauthClient.TosUri != "" {
		state.TosURI = types.StringValue(*oauthClient.TosUri)
	}
	if oauthClient.FrontchannelLogoutUri != nil && *oauthClient.FrontchannelLogoutUri != "" {
		state.FrontchannelLogoutURI = types.StringValue(*oauthClient.FrontchannelLogoutUri)
	}
	if oauthClient.BackchannelLogoutUri != nil && *oauthClient.BackchannelLogoutUri != "" {
		state.BackchannelLogoutURI = types.StringValue(*oauthClient.BackchannelLogoutUri)
	}
	if oauthClient.AccessTokenStrategy != nil && *oauthClient.AccessTokenStrategy != "" {
		state.AccessTokenStrategy = types.StringValue(*oauthClient.AccessTokenStrategy)
	}
	if oauthClient.SkipConsent != nil {
		state.SkipConsent = types.BoolValue(*oauthClient.SkipConsent)
	} else {
		state.SkipConsent = types.BoolValue(false)
	}
	if oauthClient.SkipLogoutConsent != nil {
		state.SkipLogoutConsent = types.BoolValue(*oauthClient.SkipLogoutConsent)
	} else {
		state.SkipLogoutConsent = types.BoolValue(false)
	}
	if oauthClient.SubjectType != nil && *oauthClient.SubjectType != "" {
		state.SubjectType = types.StringValue(*oauthClient.SubjectType)
	} else {
		state.SubjectType = types.StringValue("public")
	}
	if len(oauthClient.Contacts) > 0 {
		contactsList, diags := types.ListValueFrom(ctx, types.StringType, oauthClient.Contacts)
		resp.Diagnostics.Append(diags...)
		state.Contacts = contactsList
	}

	readNullableStringToState(oauthClient.AuthorizationCodeGrantAccessTokenLifespan, &state.AuthorizationCodeGrantAccessTokenLifespan)
	readNullableStringToState(oauthClient.AuthorizationCodeGrantIdTokenLifespan, &state.AuthorizationCodeGrantIdTokenLifespan)
	readNullableStringToState(oauthClient.AuthorizationCodeGrantRefreshTokenLifespan, &state.AuthorizationCodeGrantRefreshTokenLifespan)
	readNullableStringToState(oauthClient.ClientCredentialsGrantAccessTokenLifespan, &state.ClientCredentialsGrantAccessTokenLifespan)
	readNullableStringToState(oauthClient.DeviceAuthorizationGrantAccessTokenLifespan, &state.DeviceAuthorizationGrantAccessTokenLifespan)
	readNullableStringToState(oauthClient.DeviceAuthorizationGrantIdTokenLifespan, &state.DeviceAuthorizationGrantIdTokenLifespan)
	readNullableStringToState(oauthClient.DeviceAuthorizationGrantRefreshTokenLifespan, &state.DeviceAuthorizationGrantRefreshTokenLifespan)
	readNullableStringToState(oauthClient.ImplicitGrantAccessTokenLifespan, &state.ImplicitGrantAccessTokenLifespan)
	readNullableStringToState(oauthClient.ImplicitGrantIdTokenLifespan, &state.ImplicitGrantIdTokenLifespan)
	readNullableStringToState(oauthClient.JwtBearerGrantAccessTokenLifespan, &state.JwtBearerGrantAccessTokenLifespan)
	readNullableStringToState(oauthClient.RefreshTokenGrantAccessTokenLifespan, &state.RefreshTokenGrantAccessTokenLifespan)
	readNullableStringToState(oauthClient.RefreshTokenGrantIdTokenLifespan, &state.RefreshTokenGrantIdTokenLifespan)
	readNullableStringToState(oauthClient.RefreshTokenGrantRefreshTokenLifespan, &state.RefreshTokenGrantRefreshTokenLifespan)

	if oauthClient.JwksUri != nil && *oauthClient.JwksUri != "" {
		state.JwksURI = types.StringValue(*oauthClient.JwksUri)
	}
	if oauthClient.UserinfoSignedResponseAlg != nil && *oauthClient.UserinfoSignedResponseAlg != "" && *oauthClient.UserinfoSignedResponseAlg != "none" {
		state.UserinfoSignedResponseAlg = types.StringValue(*oauthClient.UserinfoSignedResponseAlg)
	}
	if oauthClient.RequestObjectSigningAlg != nil && *oauthClient.RequestObjectSigningAlg != "" && *oauthClient.RequestObjectSigningAlg != "none" {
		state.RequestObjectSigningAlg = types.StringValue(*oauthClient.RequestObjectSigningAlg)
	}
	if oauthClient.FrontchannelLogoutSessionRequired != nil {
		state.FrontchannelLogoutSessionRequired = types.BoolValue(*oauthClient.FrontchannelLogoutSessionRequired)
	} else {
		state.FrontchannelLogoutSessionRequired = types.BoolValue(false)
	}
	if oauthClient.BackchannelLogoutSessionRequired != nil {
		state.BackchannelLogoutSessionRequired = types.BoolValue(*oauthClient.BackchannelLogoutSessionRequired)
	} else {
		state.BackchannelLogoutSessionRequired = types.BoolValue(false)
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

	if !plan.AllowedCorsOrigins.IsNull() && !plan.AllowedCorsOrigins.IsUnknown() {
		var origins []string
		resp.Diagnostics.Append(plan.AllowedCorsOrigins.ElementsAs(ctx, &origins, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.AllowedCorsOrigins = origins
	}

	if !plan.ClientURI.IsNull() && !plan.ClientURI.IsUnknown() {
		oauthClient.ClientUri = ory.PtrString(plan.ClientURI.ValueString())
	}
	if !plan.LogoURI.IsNull() && !plan.LogoURI.IsUnknown() {
		oauthClient.LogoUri = ory.PtrString(plan.LogoURI.ValueString())
	}
	if !plan.PolicyURI.IsNull() && !plan.PolicyURI.IsUnknown() {
		oauthClient.PolicyUri = ory.PtrString(plan.PolicyURI.ValueString())
	}
	if !plan.TosURI.IsNull() && !plan.TosURI.IsUnknown() {
		oauthClient.TosUri = ory.PtrString(plan.TosURI.ValueString())
	}
	if !plan.FrontchannelLogoutURI.IsNull() && !plan.FrontchannelLogoutURI.IsUnknown() {
		oauthClient.FrontchannelLogoutUri = ory.PtrString(plan.FrontchannelLogoutURI.ValueString())
	}
	if !plan.BackchannelLogoutURI.IsNull() && !plan.BackchannelLogoutURI.IsUnknown() {
		oauthClient.BackchannelLogoutUri = ory.PtrString(plan.BackchannelLogoutURI.ValueString())
	}
	if !plan.AccessTokenStrategy.IsNull() && !plan.AccessTokenStrategy.IsUnknown() {
		oauthClient.AccessTokenStrategy = ory.PtrString(plan.AccessTokenStrategy.ValueString())
	}
	if !plan.SkipConsent.IsNull() && !plan.SkipConsent.IsUnknown() {
		oauthClient.SkipConsent = ory.PtrBool(plan.SkipConsent.ValueBool())
	}
	if !plan.SkipLogoutConsent.IsNull() && !plan.SkipLogoutConsent.IsUnknown() {
		oauthClient.SkipLogoutConsent = ory.PtrBool(plan.SkipLogoutConsent.ValueBool())
	}
	if !plan.SubjectType.IsNull() && !plan.SubjectType.IsUnknown() {
		oauthClient.SubjectType = ory.PtrString(plan.SubjectType.ValueString())
	}
	if !plan.Contacts.IsNull() && !plan.Contacts.IsUnknown() {
		var contacts []string
		resp.Diagnostics.Append(plan.Contacts.ElementsAs(ctx, &contacts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		oauthClient.Contacts = contacts
	}

	setNullableStringFromPlan(&oauthClient.AuthorizationCodeGrantAccessTokenLifespan, plan.AuthorizationCodeGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.AuthorizationCodeGrantIdTokenLifespan, plan.AuthorizationCodeGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.AuthorizationCodeGrantRefreshTokenLifespan, plan.AuthorizationCodeGrantRefreshTokenLifespan)
	setNullableStringFromPlan(&oauthClient.ClientCredentialsGrantAccessTokenLifespan, plan.ClientCredentialsGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.DeviceAuthorizationGrantAccessTokenLifespan, plan.DeviceAuthorizationGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.DeviceAuthorizationGrantIdTokenLifespan, plan.DeviceAuthorizationGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.DeviceAuthorizationGrantRefreshTokenLifespan, plan.DeviceAuthorizationGrantRefreshTokenLifespan)
	setNullableStringFromPlan(&oauthClient.ImplicitGrantAccessTokenLifespan, plan.ImplicitGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.ImplicitGrantIdTokenLifespan, plan.ImplicitGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.JwtBearerGrantAccessTokenLifespan, plan.JwtBearerGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.RefreshTokenGrantAccessTokenLifespan, plan.RefreshTokenGrantAccessTokenLifespan)
	setNullableStringFromPlan(&oauthClient.RefreshTokenGrantIdTokenLifespan, plan.RefreshTokenGrantIdTokenLifespan)
	setNullableStringFromPlan(&oauthClient.RefreshTokenGrantRefreshTokenLifespan, plan.RefreshTokenGrantRefreshTokenLifespan)

	if !plan.JwksURI.IsNull() && !plan.JwksURI.IsUnknown() {
		oauthClient.JwksUri = ory.PtrString(plan.JwksURI.ValueString())
	}
	if !plan.UserinfoSignedResponseAlg.IsNull() && !plan.UserinfoSignedResponseAlg.IsUnknown() {
		oauthClient.UserinfoSignedResponseAlg = ory.PtrString(plan.UserinfoSignedResponseAlg.ValueString())
	}
	if !plan.RequestObjectSigningAlg.IsNull() && !plan.RequestObjectSigningAlg.IsUnknown() {
		oauthClient.RequestObjectSigningAlg = ory.PtrString(plan.RequestObjectSigningAlg.ValueString())
	}
	if !plan.FrontchannelLogoutSessionRequired.IsNull() && !plan.FrontchannelLogoutSessionRequired.IsUnknown() {
		oauthClient.FrontchannelLogoutSessionRequired = ory.PtrBool(plan.FrontchannelLogoutSessionRequired.ValueBool())
	}
	if !plan.BackchannelLogoutSessionRequired.IsNull() && !plan.BackchannelLogoutSessionRequired.IsUnknown() {
		oauthClient.BackchannelLogoutSessionRequired = ory.PtrBool(plan.BackchannelLogoutSessionRequired.ValueBool())
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

	if updated.SkipConsent != nil {
		plan.SkipConsent = types.BoolValue(*updated.SkipConsent)
	} else {
		plan.SkipConsent = types.BoolValue(false)
	}
	if updated.SkipLogoutConsent != nil {
		plan.SkipLogoutConsent = types.BoolValue(*updated.SkipLogoutConsent)
	} else {
		plan.SkipLogoutConsent = types.BoolValue(false)
	}
	if updated.SubjectType != nil && *updated.SubjectType != "" {
		plan.SubjectType = types.StringValue(*updated.SubjectType)
	} else {
		plan.SubjectType = types.StringValue("public")
	}
	if updated.FrontchannelLogoutSessionRequired != nil {
		plan.FrontchannelLogoutSessionRequired = types.BoolValue(*updated.FrontchannelLogoutSessionRequired)
	} else {
		plan.FrontchannelLogoutSessionRequired = types.BoolValue(false)
	}
	if updated.BackchannelLogoutSessionRequired != nil {
		plan.BackchannelLogoutSessionRequired = types.BoolValue(*updated.BackchannelLogoutSessionRequired)
	} else {
		plan.BackchannelLogoutSessionRequired = types.BoolValue(false)
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

func setNullableStringFromPlan(field *ory.NullableString, tfValue types.String) {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		v := tfValue.ValueString()
		field.Set(&v)
	}
}

func readNullableStringToState(field ory.NullableString, tfValue *types.String) {
	if field.IsSet() {
		if v := field.Get(); v != nil && *v != "" {
			*tfValue = types.StringValue(normalizeDuration(*v))
		}
	}
}

func normalizeDuration(s string) string {
	d, err := time.ParseDuration(s)
	if err != nil {
		return s
	}
	if d == 0 {
		return "0s"
	}
	hours := int(d.Hours())
	remaining := d - time.Duration(hours)*time.Hour
	minutes := int(remaining.Minutes())
	remaining -= time.Duration(minutes) * time.Minute
	seconds := int(remaining.Seconds())

	var result string
	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 {
		result += fmt.Sprintf("%ds", seconds)
	}
	if result == "" {
		return d.String()
	}
	return result
}
