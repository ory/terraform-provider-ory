package trustedjwtissuer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &TrustedJwtIssuerResource{}
	_ resource.ResourceWithConfigure   = &TrustedJwtIssuerResource{}
	_ resource.ResourceWithImportState = &TrustedJwtIssuerResource{}
)

// NewResource returns a new TrustedJwtIssuer resource.
func NewResource() resource.Resource {
	return &TrustedJwtIssuerResource{}
}

// TrustedJwtIssuerResource defines the resource implementation.
type TrustedJwtIssuerResource struct {
	client *client.OryClient
}

// TrustedJwtIssuerResourceModel describes the resource data model.
type TrustedJwtIssuerResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Issuer          types.String `tfsdk:"issuer"`
	Scope           types.List   `tfsdk:"scope"`
	ExpiresAt       types.String `tfsdk:"expires_at"`
	Subject         types.String `tfsdk:"subject"`
	AllowAnySubject types.Bool   `tfsdk:"allow_any_subject"`
	Jwk             types.String `tfsdk:"jwk"`
	CreatedAt       types.String `tfsdk:"created_at"`
}

func (r *TrustedJwtIssuerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trusted_oauth2_jwt_grant_issuer"
}

func (r *TrustedJwtIssuerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a trusted OAuth2 JWT grant issuer in Ory Network.",
		MarkdownDescription: `
Manages a trusted OAuth2 JWT grant issuer in Ory Network.

A trusted JWT grant issuer allows exchanging JWTs signed by the issuer for
OAuth2 access tokens via the JWT bearer grant type (RFC 7523).

**Important:** This resource is create-and-delete only. To change any field
that requires replacement (issuer, expires_at, jwk), the trust relationship
must be destroyed and recreated.

## Example Usage

` + "```hcl" + `
resource "ory_trusted_oauth2_jwt_grant_issuer" "example" {
  issuer     = "https://jwt-idp.example.com"
  scope      = ["openid", "offline_access"]
  expires_at = "2025-12-31T23:59:59Z"
  jwk        = jsonencode({
    kty = "RSA"
    kid = "my-key-id"
    # ... other JWK fields
  })
}
` + "```" + `

## Import

Trusted JWT grant issuers can be imported using their ID:

` + "```shell" + `
terraform import ory_trusted_oauth2_jwt_grant_issuer.example <issuer-id>
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the trusted JWT grant issuer.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"issuer": schema.StringAttribute{
				Description: "The JWT issuer (iss claim). Tokens with this issuer will be trusted.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scope": schema.ListAttribute{
				Description: "List of OAuth2 scopes that can be granted when exchanging a JWT from this issuer.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Description: "The expiration time of the trust relationship in RFC3339 format. After this time, JWTs from this issuer will no longer be accepted.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject": schema.StringAttribute{
				Description: "The specific subject (sub claim) that is allowed. If set, only JWTs with this exact subject will be accepted.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"allow_any_subject": schema.BoolAttribute{
				Description: "When true, JWTs with any subject will be accepted from this issuer. Cannot be used together with subject.",
				Optional:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"jwk": schema.StringAttribute{
				Description: "The JSON Web Key (JWK) as a JSON string. This is the public key used to verify JWTs from the issuer.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "The time when the trust relationship was created in RFC3339 format.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TrustedJwtIssuerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TrustedJwtIssuerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TrustedJwtIssuerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse expires_at
	expiresAt, err := time.Parse(time.RFC3339, plan.ExpiresAt.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid expires_at",
			"Could not parse expires_at as RFC3339 timestamp: "+err.Error(),
		)
		return
	}

	// Parse scope
	var scope []string
	resp.Diagnostics.Append(plan.Scope.ElementsAs(ctx, &scope, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse JWK
	var jwk ory.JsonWebKey
	if err := json.Unmarshal([]byte(plan.Jwk.ValueString()), &jwk); err != nil {
		resp.Diagnostics.AddError(
			"Invalid JWK JSON",
			"Could not parse jwk as JSON Web Key: "+err.Error(),
		)
		return
	}

	body := ory.TrustOAuth2JwtGrantIssuer{
		Issuer:    plan.Issuer.ValueString(),
		Scope:     scope,
		ExpiresAt: expiresAt,
		Jwk:       jwk,
	}

	if !plan.Subject.IsNull() && !plan.Subject.IsUnknown() {
		body.Subject = ory.PtrString(plan.Subject.ValueString())
	}
	if !plan.AllowAnySubject.IsNull() && !plan.AllowAnySubject.IsUnknown() {
		body.AllowAnySubject = ory.PtrBool(plan.AllowAnySubject.ValueBool())
	}

	created, err := r.client.TrustOAuth2JwtGrantIssuer(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Trusted JWT Grant Issuer",
			"Could not create trusted JWT grant issuer: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(created.GetId())
	plan.Issuer = types.StringValue(created.GetIssuer())
	plan.ExpiresAt = types.StringValue(created.GetExpiresAt().Format(time.RFC3339))
	plan.CreatedAt = types.StringValue(created.GetCreatedAt().Format(time.RFC3339))

	if len(created.GetScope()) > 0 {
		scopeList, diags := types.ListValueFrom(ctx, types.StringType, created.GetScope())
		resp.Diagnostics.Append(diags...)
		plan.Scope = scopeList
	}

	if created.GetSubject() != "" {
		plan.Subject = types.StringValue(created.GetSubject())
	}

	if created.GetAllowAnySubject() {
		plan.AllowAnySubject = types.BoolValue(true)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *TrustedJwtIssuerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TrustedJwtIssuerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	issuer, err := r.client.GetTrustedOAuth2JwtGrantIssuer(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Trusted JWT Grant Issuer",
			"Could not read trusted JWT grant issuer "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Issuer = types.StringValue(issuer.GetIssuer())
	state.ExpiresAt = types.StringValue(issuer.GetExpiresAt().Format(time.RFC3339))
	state.CreatedAt = types.StringValue(issuer.GetCreatedAt().Format(time.RFC3339))

	if len(issuer.GetScope()) > 0 {
		scopeList, diags := types.ListValueFrom(ctx, types.StringType, issuer.GetScope())
		resp.Diagnostics.Append(diags...)
		state.Scope = scopeList
	}

	if issuer.GetSubject() != "" {
		state.Subject = types.StringValue(issuer.GetSubject())
	}

	if issuer.GetAllowAnySubject() {
		state.AllowAnySubject = types.BoolValue(true)
	}

	// JWK is sensitive and not returned by the read endpoint;
	// preserve the value from state.

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TrustedJwtIssuerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This resource does not support in-place updates.
	// All mutable-looking fields that are not marked RequiresReplace will trigger
	// a destroy-and-recreate via plan modifiers. If Terraform still routes here,
	// return an explicit error.
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Trusted OAuth2 JWT grant issuers cannot be updated in place. "+
			"The resource must be destroyed and recreated to apply changes.",
	)
}

func (r *TrustedJwtIssuerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TrustedJwtIssuerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTrustedOAuth2JwtGrantIssuer(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Trusted JWT Grant Issuer",
			"Could not delete trusted JWT grant issuer: "+err.Error(),
		)
		return
	}
}

func (r *TrustedJwtIssuerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
