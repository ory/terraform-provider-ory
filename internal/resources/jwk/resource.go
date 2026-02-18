package jwk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &JWKResource{}
	_ resource.ResourceWithConfigure   = &JWKResource{}
	_ resource.ResourceWithImportState = &JWKResource{}
)

// NewResource returns a new JWK resource.
func NewResource() resource.Resource {
	return &JWKResource{}
}

// JWKResource defines the resource implementation.
type JWKResource struct {
	client *client.OryClient
}

// JWKResourceModel describes the resource data model.
type JWKResourceModel struct {
	ID        types.String `tfsdk:"id"`
	SetID     types.String `tfsdk:"set_id"`
	KeyID     types.String `tfsdk:"key_id"`
	Algorithm types.String `tfsdk:"algorithm"`
	Use       types.String `tfsdk:"use"`
	Keys      types.String `tfsdk:"keys"`
}

func (r *JWKResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_json_web_key_set"
}

const jwkMarkdownDescription = `
Manages an Ory Network JSON Web Key Set (JWKS).

JSON Web Keys are used for signing and encrypting tokens. This resource allows you to
generate and manage custom key sets for your Ory project.

## Example Usage

### Generate RSA Key for Signing

` + "```hcl" + `
resource "ory_json_web_key_set" "signing" {
  set_id    = "my-signing-keys"
  key_id    = "sig-key-1"
  algorithm = "RS256"
  use       = "sig"
}
` + "```" + `

### Generate EC Key for Encryption

` + "```hcl" + `
resource "ory_json_web_key_set" "encryption" {
  set_id    = "my-encryption-keys"
  key_id    = "enc-key-1"
  algorithm = "ES256"
  use       = "enc"
}
` + "```" + `

## Import

JWK sets can be imported using their set ID:

` + "```shell" + `
terraform import ory_json_web_key_set.signing my-signing-keys
` + "```" + `
`

func (r *JWKResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Network JSON Web Key Set.",
		MarkdownDescription: jwkMarkdownDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Internal Terraform ID (same as set_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"set_id": schema.StringAttribute{
				Description: "The ID of the JSON Web Key Set.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_id": schema.StringAttribute{
				Description: "The Key ID (kid) for the generated key.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"algorithm": schema.StringAttribute{
				Description: "The algorithm for the key: RS256, ES256, ES512, HS256, HS512.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("RS256", "ES256", "ES512", "HS256", "HS512"),
				},
			},
			"use": schema.StringAttribute{
				Description: "The intended use: sig (signature) or enc (encryption).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("sig", "enc"),
				},
			},
			"keys": schema.StringAttribute{
				Description: "The JSON Web Key Set as a JSON string (public parts only).",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *JWKResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *JWKResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan JWKResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := ory.CreateJsonWebKeySet{
		Alg: plan.Algorithm.ValueString(),
		Kid: plan.KeyID.ValueString(),
		Use: plan.Use.ValueString(),
	}

	jwks, err := r.client.CreateJsonWebKeySet(ctx, plan.SetID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating JSON Web Key Set",
			"Could not create JWK set: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(plan.SetID.ValueString())

	// Serialize the keys to JSON
	if len(jwks.Keys) > 0 {
		keysJSON, err := json.Marshal(jwks)
		if err == nil {
			plan.Keys = types.StringValue(string(keysJSON))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *JWKResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state JWKResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jwks, err := r.client.GetJsonWebKeySet(ctx, state.SetID.ValueString())
	if err != nil {
		// Check if it's a 404
		resp.Diagnostics.AddError(
			"Error Reading JSON Web Key Set",
			"Could not read JWK set "+state.SetID.ValueString()+": "+err.Error(),
		)
		return
	}

	if len(jwks.Keys) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Serialize the keys to JSON
	keysJSON, err := json.Marshal(jwks)
	if err == nil {
		state.Keys = types.StringValue(string(keysJSON))
	}

	// Try to extract algorithm and use from the first key
	if len(jwks.Keys) > 0 {
		firstKey := jwks.Keys[0]
		state.Algorithm = types.StringValue(firstKey.Alg)
		state.Use = types.StringValue(firstKey.Use)
		state.KeyID = types.StringValue(firstKey.Kid)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *JWKResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// JWK sets are immutable - all changes require replacement
	// This is handled by RequiresReplace on all fields
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"JSON Web Key Sets cannot be updated. Changes require resource replacement.",
	)
}

func (r *JWKResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state JWKResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteJsonWebKeySet(ctx, state.SetID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting JSON Web Key Set",
			"Could not delete JWK set: "+err.Error(),
		)
		return
	}
}

func (r *JWKResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("set_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
