package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &IdentityResource{}
	_ resource.ResourceWithConfigure      = &IdentityResource{}
	_ resource.ResourceWithImportState    = &IdentityResource{}
	_ resource.ResourceWithValidateConfig = &IdentityResource{}
)

// NewResource returns a new Identity resource.
func NewResource() resource.Resource {
	return &IdentityResource{}
}

// IdentityResource defines the resource implementation.
type IdentityResource struct {
	client *client.OryClient
}

// IdentityResourceModel describes the resource data model.
type IdentityResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectSlug    types.String `tfsdk:"project_slug"`
	ProjectAPIKey  types.String `tfsdk:"project_api_key"`
	SchemaID       types.String `tfsdk:"schema_id"`
	Traits         types.String `tfsdk:"traits"`
	State          types.String `tfsdk:"state"`
	Password       types.String `tfsdk:"password"`
	MetadataPublic types.String `tfsdk:"metadata_public"`
	MetadataAdmin  types.String `tfsdk:"metadata_admin"`
}

func (r *IdentityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity"
}

const identityMarkdownDescription = `
Manages an Ory Network identity (user).

Identities represent users in your application. Each identity has traits
(profile data) defined by an identity schema.

## Required Provider Configuration

This resource requires the following provider configuration:

` + "```hcl" + `
provider "ory" {
  project_api_key = var.ory_project_api_key  # or ORY_PROJECT_API_KEY env var
  project_slug    = var.ory_project_slug     # or ORY_PROJECT_SLUG env var
}
` + "```" + `

Or via environment variables:

` + "```bash" + `
export ORY_PROJECT_API_KEY="ory_pat_..."
export ORY_PROJECT_SLUG="your-project-slug"
` + "```" + `

## Schema ID

The ` + "`schema_id`" + ` attribute specifies which identity schema defines the structure of the identity's traits:

- **Use a preset schema**: ` + "`preset://email`" + ` (most common default for new projects) or ` + "`preset://username`" + `
- **Reference a Terraform-managed schema**: ` + "`ory_identity_schema.customer.schema_id`" + `
- **Use a custom ID**: e.g., ` + "`customer_v1`" + ` if created via Console or API

~> **Note:** The ` + "`schema_id`" + ` must match a schema that exists in your project. Check your project's identity schemas in the Ory Console, or use ` + "`GET /projects/<id>`" + ` and inspect ` + "`services.identity.config.identity.schemas`" + `.

## Example Usage

` + "```hcl" + `
# Identity using the preset email schema
resource "ory_identity" "user" {
  schema_id = "preset://email"

  traits = jsonencode({
    email = "user@example.com"
  })
}

# Identity with password
resource "ory_identity" "user_with_password" {
  schema_id = "preset://email"

  traits = jsonencode({
    email = "user@example.com"
  })

  password = var.user_password  # Use a variable, never hardcode

  metadata_public = jsonencode({
    role = "user"
  })
}

# Identity using a custom Terraform-managed schema
resource "ory_identity" "customer" {
  schema_id = ory_identity_schema.customer.schema_id

  traits = jsonencode({
    email      = "customer@example.com"
    first_name = "Jane"
    last_name  = "Doe"
  })
}
` + "```" + `

## Import

Identities can be imported using their ID:

` + "```shell" + `
terraform import ory_identity.user <identity-id>
` + "```" + `

**Note**: If the identity is deleted outside of Terraform (e.g., via UI or API),
the next ` + "`terraform plan`" + ` will detect this and remove it from state.
`

func (r *IdentityResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Network identity (user).",
		MarkdownDescription: identityMarkdownDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the identity.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_slug": schema.StringAttribute{
				Description: "Project slug for API access. Use this to pass credentials at the resource level when the provider is configured before the project exists (e.g., creating a project and identity in the same apply). Overrides the provider-level project_slug.",
				Optional:    true,
			},
			"project_api_key": schema.StringAttribute{
				Description: "Project API key for API access. Use this to pass credentials at the resource level when the provider is configured before the project exists (e.g., creating a project and identity in the same apply). Overrides the provider-level project_api_key.",
				Optional:    true,
				Sensitive:   true,
			},
			"schema_id": schema.StringAttribute{
				Description: "Identity schema ID. Must match a schema configured in your project (e.g., 'preset://email', a custom schema ID). Check your project's identity schemas in the Ory Console or API.",
				Required:    true,
			},
			"traits": schema.StringAttribute{
				Description: "Identity traits as JSON string. The structure depends on your identity schema.",
				Required:    true,
			},
			"state": schema.StringAttribute{
				Description: "Identity state: active or inactive.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("active"),
			},
			"password": schema.StringAttribute{
				Description: "Password for the identity. Write-only, not returned on read.",
				Optional:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"metadata_public": schema.StringAttribute{
				Description: "Public metadata as JSON string. Visible to the identity.",
				Optional:    true,
			},
			"metadata_admin": schema.StringAttribute{
				Description: "Admin metadata as JSON string. Only visible to admins.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *IdentityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// resolveClient returns a request-scoped client, optionally using resource-level
// project credentials. The provider-configured client on the struct is never
// mutated, which avoids data races when Terraform runs multiple instances of the
// same resource type concurrently (e.g., for_each across projects).
func (r *IdentityResource) resolveClient(slug, apiKey types.String) *client.OryClient {
	if !slug.IsNull() && !slug.IsUnknown() && !apiKey.IsNull() && !apiKey.IsUnknown() {
		return r.client.WithProjectCredentials(slug.ValueString(), apiKey.ValueString())
	}
	return r.client
}

func (r *IdentityResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config IdentityResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation if either value is unknown (will be resolved at apply time)
	if config.ProjectSlug.IsUnknown() || config.ProjectAPIKey.IsUnknown() {
		return
	}

	slugSet := !config.ProjectSlug.IsNull()
	keySet := !config.ProjectAPIKey.IsNull()

	if slugSet != keySet {
		if !slugSet {
			resp.Diagnostics.AddAttributeError(
				path.Root("project_slug"),
				"Missing Required Attribute",
				"project_slug is required when project_api_key is set. Both must be specified together.",
			)
		}
		if !keySet {
			resp.Diagnostics.AddAttributeError(
				path.Root("project_api_key"),
				"Missing Required Attribute",
				"project_api_key is required when project_slug is set. Both must be specified together.",
			)
		}
	}
}

func (r *IdentityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IdentityResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Derive a request-scoped client, using resource-level credentials if provided
	client := r.resolveClient(plan.ProjectSlug, plan.ProjectAPIKey)

	cfg := client.Config()
	if !helpers.ResolveProjectCreds(cfg.ProjectSlug, cfg.ProjectAPIKey, &resp.Diagnostics) {
		return
	}

	var traits map[string]interface{}
	if err := json.Unmarshal([]byte(plan.Traits.ValueString()), &traits); err != nil {
		resp.Diagnostics.AddError(
			"Invalid Traits JSON",
			"Could not parse traits as JSON: "+err.Error(),
		)
		return
	}

	body := ory.CreateIdentityBody{
		SchemaId: plan.SchemaID.ValueString(),
		Traits:   traits,
		State:    ory.PtrString(plan.State.ValueString()),
	}

	if !plan.Password.IsNull() && !plan.Password.IsUnknown() {
		body.Credentials = &ory.IdentityWithCredentials{
			Password: &ory.IdentityWithCredentialsPassword{
				Config: &ory.IdentityWithCredentialsPasswordConfig{
					Password: ory.PtrString(plan.Password.ValueString()),
				},
			},
		}
	}

	if !plan.MetadataPublic.IsNull() && !plan.MetadataPublic.IsUnknown() {
		var metadataPublic interface{}
		if err := json.Unmarshal([]byte(plan.MetadataPublic.ValueString()), &metadataPublic); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Metadata Public JSON",
				"Could not parse metadata_public as JSON: "+err.Error(),
			)
			return
		}
		body.MetadataPublic = metadataPublic
	}

	if !plan.MetadataAdmin.IsNull() && !plan.MetadataAdmin.IsUnknown() {
		var metadataAdmin interface{}
		if err := json.Unmarshal([]byte(plan.MetadataAdmin.ValueString()), &metadataAdmin); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Metadata Admin JSON",
				"Could not parse metadata_admin as JSON: "+err.Error(),
			)
			return
		}
		body.MetadataAdmin = metadataAdmin
	}

	identity, err := client.CreateIdentity(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Identity",
			"Could not create identity: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(identity.GetId())
	plan.SchemaID = types.StringValue(identity.GetSchemaId())
	plan.State = types.StringValue(identity.GetState())

	if identity.Traits != nil {
		traitsJSON, err := json.Marshal(identity.Traits)
		if err == nil {
			plan.Traits = types.StringValue(string(traitsJSON))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *IdentityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IdentityResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Derive a request-scoped client, using resource-level credentials if provided
	client := r.resolveClient(state.ProjectSlug, state.ProjectAPIKey)

	identity, err := client.GetIdentity(ctx, state.ID.ValueString())
	if err != nil {
		// Check if it's a 404 (identity deleted outside Terraform)
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(strings.ToLower(errStr), "not found") {
			// Identity was deleted outside Terraform, remove from state
			resp.Diagnostics.AddWarning(
				"Identity Not Found",
				fmt.Sprintf("Identity %s was not found (possibly deleted outside Terraform). Removing from state.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Identity",
			"Could not read identity ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.SchemaID = types.StringValue(identity.GetSchemaId())
	state.State = types.StringValue(identity.GetState())

	if identity.Traits != nil {
		traitsJSON, err := json.Marshal(identity.Traits)
		if err == nil {
			state.Traits = types.StringValue(string(traitsJSON))
		}
	}

	if identity.MetadataPublic != nil {
		metadataJSON, err := json.Marshal(identity.MetadataPublic)
		if err == nil {
			state.MetadataPublic = types.StringValue(string(metadataJSON))
		}
	}

	if identity.MetadataAdmin != nil {
		metadataJSON, err := json.Marshal(identity.MetadataAdmin)
		if err == nil {
			state.MetadataAdmin = types.StringValue(string(metadataJSON))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IdentityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IdentityResourceModel
	var state IdentityResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Derive a request-scoped client, using resource-level credentials if provided
	client := r.resolveClient(plan.ProjectSlug, plan.ProjectAPIKey)

	var traits map[string]interface{}
	if err := json.Unmarshal([]byte(plan.Traits.ValueString()), &traits); err != nil {
		resp.Diagnostics.AddError(
			"Invalid Traits JSON",
			"Could not parse traits as JSON: "+err.Error(),
		)
		return
	}

	body := ory.UpdateIdentityBody{
		SchemaId: plan.SchemaID.ValueString(),
		Traits:   traits,
		State:    plan.State.ValueString(),
	}

	if !plan.MetadataPublic.IsNull() && !plan.MetadataPublic.IsUnknown() {
		var metadataPublic interface{}
		if err := json.Unmarshal([]byte(plan.MetadataPublic.ValueString()), &metadataPublic); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Metadata Public JSON",
				"Could not parse metadata_public as JSON: "+err.Error(),
			)
			return
		}
		body.MetadataPublic = metadataPublic
	}

	if !plan.MetadataAdmin.IsNull() && !plan.MetadataAdmin.IsUnknown() {
		var metadataAdmin interface{}
		if err := json.Unmarshal([]byte(plan.MetadataAdmin.ValueString()), &metadataAdmin); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Metadata Admin JSON",
				"Could not parse metadata_admin as JSON: "+err.Error(),
			)
			return
		}
		body.MetadataAdmin = metadataAdmin
	}

	identity, err := client.UpdateIdentity(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Identity",
			"Could not update identity: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.SchemaID = types.StringValue(identity.GetSchemaId())
	plan.State = types.StringValue(identity.GetState())

	if identity.Traits != nil {
		traitsJSON, err := json.Marshal(identity.Traits)
		if err == nil {
			plan.Traits = types.StringValue(string(traitsJSON))
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *IdentityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IdentityResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Derive a request-scoped client, using resource-level credentials if provided
	client := r.resolveClient(state.ProjectSlug, state.ProjectAPIKey)

	err := client.DeleteIdentity(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Identity",
			"Could not delete identity: "+err.Error(),
		)
		return
	}
}

func (r *IdentityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
