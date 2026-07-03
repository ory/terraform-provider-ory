package jwk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &JWKResource{}
	_ resource.ResourceWithConfigure      = &JWKResource{}
	_ resource.ResourceWithImportState    = &JWKResource{}
	_ resource.ResourceWithValidateConfig = &JWKResource{}
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
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	ProjectSlug   types.String `tfsdk:"project_slug"`
	ProjectAPIKey types.String `tfsdk:"project_api_key"`
	SetID         types.String `tfsdk:"set_id"`
	KeyID         types.String `tfsdk:"key_id"`
	Algorithm     types.String `tfsdk:"algorithm"`
	Use           types.String `tfsdk:"use"`
	Keys          types.String `tfsdk:"keys"`
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
  project_id = var.ory_project_id
  set_id     = "my-signing-keys"
  key_id     = "sig-key-1"
  algorithm  = "RS256"
  use        = "sig"
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

### Same-Apply with Project Creation

Use resource-level credentials when creating a JWK set in the same apply as the project:

` + "```hcl" + `
resource "ory_json_web_key_set" "signing" {
  project_slug    = ory_project.main.slug
  project_api_key = ory_project_api_key.main.value

  set_id    = "token-signing-keys"
  key_id    = "rsa-sig-1"
  algorithm = "RS256"
  use       = "sig"
}
` + "```" + `

## Import

JWK sets can be imported using the format ` + "`project_id/set_id`" + ` or just ` + "`set_id`" + `:

` + "```shell" + `
terraform import ory_json_web_key_set.signing <project-id>/my-signing-keys
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
			"project_id": schema.StringAttribute{
				Description: "The project ID. If not set, uses the provider's project_id or project_slug.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"project_slug": schema.StringAttribute{
				Description: "Project slug for API access. Use this to pass credentials at the resource level when the provider is configured before the project exists (e.g., creating a project and JWK set in the same apply). Overrides the provider-level project_slug.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"project_api_key": schema.StringAttribute{
				Description: "Project API key for API access. Use this to pass credentials at the resource level when the provider is configured before the project exists (e.g., creating a project and JWK set in the same apply). Overrides the provider-level project_api_key.",
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
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
				Description: "The JSON Web Key Set as a JSON string, including private key material. This is a sensitive value.",
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

func (r *JWKResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config JWKResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(helpers.ValidateProjectCredentialPair(config.ProjectSlug, config.ProjectAPIKey)...)
}

func (r *JWKResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan JWKResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Derive a request-scoped client, using resource-level credentials if provided
	client := helpers.ResolveProjectClient(r.client, plan.ProjectSlug, plan.ProjectAPIKey)

	// When resource-level credentials are provided, normalize unknown project_id
	// to null. This prevents failures when project_id references a resource being
	// created in the same apply (e.g., project_id = ory_project.main.id).
	projectIDAttr := plan.ProjectID
	if helpers.HasResourceLevelCredentials(plan.ProjectSlug, plan.ProjectAPIKey) && projectIDAttr.IsUnknown() {
		projectIDAttr = types.StringNull()
	}

	projectClient, projectID, diags := r.projectClient(ctx, client, projectIDAttr)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := ory.CreateJsonWebKeySet{
		Alg: plan.Algorithm.ValueString(),
		Kid: plan.KeyID.ValueString(),
		Use: plan.Use.ValueString(),
	}

	jwks, err := projectClient.CreateJsonWebKeySet(ctx, plan.SetID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating JSON Web Key Set",
			"Could not create JWK set: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(plan.SetID.ValueString())
	if projectID != "" {
		plan.ProjectID = types.StringValue(projectID)
	} else if plan.ProjectID.IsUnknown() {
		// When using resource-level credentials without project_id,
		// explicitly set to null so Terraform doesn't complain about unknown values after apply.
		plan.ProjectID = types.StringNull()
	}

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

	// Derive a request-scoped client, using resource-level credentials if provided
	client := helpers.ResolveProjectClient(r.client, state.ProjectSlug, state.ProjectAPIKey)

	projectClient, projectID, diags := r.projectClient(ctx, client, state.ProjectID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	jwks, err := projectClient.GetJsonWebKeySet(ctx, state.SetID.ValueString())
	if err != nil {
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

	if projectID != "" {
		state.ProjectID = types.StringValue(projectID)
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

	// Derive a request-scoped client, using resource-level credentials if provided
	client := helpers.ResolveProjectClient(r.client, state.ProjectSlug, state.ProjectAPIKey)

	projectClient, _, diags := r.projectClient(ctx, client, state.ProjectID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := projectClient.DeleteJsonWebKeySet(ctx, state.SetID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting JSON Web Key Set",
			"Could not delete JWK set: "+err.Error(),
		)
		return
	}
}

func (r *JWKResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: project_id/set_id or just set_id (uses provider's project_id)
	id := req.ID
	var projectID, setID string

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		projectID = parts[0]
		setID = parts[1]
		if projectID == "" || setID == "" {
			resp.Diagnostics.AddError(
				"Invalid Import ID",
				fmt.Sprintf("Expected format \"project_id/set_id\" or \"set_id\", got %q.", id),
			)
			return
		}
	} else {
		setID = id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("set_id"), setID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), setID)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}

// projectClient returns a client scoped to the correct project along with the
// resolved project ID. When a resource-level project_id is set, the slug is
// resolved via the console API and a project-scoped client is returned. When
// only the provider's project_slug+project_api_key are configured (no
// project_id anywhere), the provider's default client is returned as-is so
// existing configurations are not broken.
func (r *JWKResource) projectClient(ctx context.Context, c *client.OryClient, tfProjectID types.String) (*client.OryClient, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Use explicit resource-level project_id first.
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() && tfProjectID.ValueString() != "" {
		pid := tfProjectID.ValueString()
		pc, err := c.ProjectClientForProject(ctx, pid)
		if err != nil {
			diags.AddError(
				fmt.Sprintf("Error Resolving Project %q", pid),
				fmt.Sprintf("Could not resolve project_id %q to a project slug: %s. "+
					"Ensure the project_id is valid and that workspace_api_key is configured on the provider.", pid, err.Error()),
			)
			return nil, "", diags
		}
		return pc, pid, diags
	}

	// Fall back to provider-level project_id.
	if pid := c.ProjectID(); pid != "" {
		pc, err := c.ProjectClientForProject(ctx, pid)
		if err != nil {
			diags.AddError(
				fmt.Sprintf("Error Resolving Provider Project %q", pid),
				fmt.Sprintf("Could not resolve provider project_id %q to a project slug: %s. "+
					"Ensure the project_id is valid and that workspace_api_key is configured on the provider.", pid, err.Error()),
			)
			return nil, "", diags
		}
		return pc, pid, diags
	}

	// No project_id at all -- the provider may still have project_slug +
	// project_api_key configured directly, so fall back to the passed client.
	return c, "", diags
}
