package organization

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &OrganizationResource{}
	_ resource.ResourceWithConfigure   = &OrganizationResource{}
	_ resource.ResourceWithImportState = &OrganizationResource{}
)

// NewResource returns a new Organization resource.
func NewResource() resource.Resource {
	return &OrganizationResource{}
}

// OrganizationResource defines the resource implementation.
type OrganizationResource struct {
	client *client.OryClient
}

// OrganizationResourceModel describes the resource data model.
type OrganizationResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Label     types.String `tfsdk:"label"`
	Domains   types.List   `tfsdk:"domains"`
	ProjectID types.String `tfsdk:"project_id"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *OrganizationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *OrganizationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network organization for multi-tenancy.",
		MarkdownDescription: `
Manages an Ory Network organization.

Organizations represent tenants in a multi-tenant application. They can have
associated SSO domains and contain users (identities).

~> **Important:** Organizations require:
- An Ory Network **Growth plan or higher** with B2B features enabled
- Project environment set to ` + "`prod`" + ` (Production) or ` + "`stage`" + ` (Staging)
- Organizations are NOT available in ` + "`dev`" + ` (Development) environments

## Example Usage

` + "```hcl" + `
# Ensure your project is in prod or stage environment
resource "ory_project" "main" {
  name        = "My B2B App"
  environment = "prod"  # or "stage" - NOT "dev"
}

resource "ory_organization" "acme" {
  label   = "Acme Corporation"
  domains = ["acme.com", "acme.io"]
}
` + "```" + `

## Import

Organizations can be imported using the organization ID:

` + "```shell" + `
terraform import ory_organization.acme <organization-id>
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the organization.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"label": schema.StringAttribute{
				Description: "Human-readable organization name.",
				Required:    true,
			},
			"domains": schema.ListAttribute{
				Description: "List of SSO domains for this organization.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"project_id": schema.StringAttribute{
				Description: "The project ID. If not set, uses the provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the organization was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *OrganizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OrganizationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(plan.ProjectID)
	if projectID == "" {
		resp.Diagnostics.AddError(
			"Missing Project ID",
			"project_id must be set either in the resource or provider configuration.",
		)
		return
	}

	// Check project environment - organizations only work in prod/stage, not dev
	env, err := r.client.GetProjectEnvironment(ctx, projectID)
	if err == nil && env == "dev" {
		resp.Diagnostics.AddError(
			"Invalid Project Environment",
			"Organizations are not available in development (dev) projects. "+
				"Please use a project with environment set to 'prod' or 'stage'. "+
				"You can create a new project with: ory_project { environment = \"prod\" }",
		)
		return
	}

	var domains []string
	if !plan.Domains.IsNull() && !plan.Domains.IsUnknown() {
		resp.Diagnostics.Append(plan.Domains.ElementsAs(ctx, &domains, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	org, err := r.client.CreateOrganization(ctx, projectID, plan.Label.ValueString(), domains)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Organization",
			"Could not create organization: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(org.GetId())
	plan.ProjectID = types.StringValue(projectID)
	plan.Label = types.StringValue(org.GetLabel())
	plan.CreatedAt = types.StringValue(org.CreatedAt.String())

	// Preserve null state for domains if it was null in the plan
	// This prevents "inconsistent result after apply" errors when the API
	// returns an empty array but the user didn't specify domains
	if plan.Domains.IsNull() {
		// Keep it null - user didn't specify domains
	} else if len(org.Domains) > 0 {
		domainList, diags := types.ListValueFrom(ctx, types.StringType, org.Domains)
		resp.Diagnostics.Append(diags...)
		plan.Domains = domainList
	} else {
		// User specified domains (possibly empty list), API returned empty
		plan.Domains = types.ListValueMust(types.StringType, []attr.Value{})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *OrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OrganizationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(state.ProjectID)

	org, err := r.client.GetOrganization(ctx, projectID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Organization",
			"Could not read organization ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Label = types.StringValue(org.GetLabel())
	state.ProjectID = types.StringValue(projectID)
	state.CreatedAt = types.StringValue(org.CreatedAt.String())

	// Preserve null state for domains if it was null in the existing state
	// This prevents drift when the API returns an empty array
	if state.Domains.IsNull() && len(org.Domains) == 0 {
		// Keep it null - matches existing state
	} else if len(org.Domains) > 0 {
		domainList, diags := types.ListValueFrom(ctx, types.StringType, org.Domains)
		resp.Diagnostics.Append(diags...)
		state.Domains = domainList
	} else {
		// State had domains but API returned empty
		state.Domains = types.ListValueMust(types.StringType, []attr.Value{})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OrganizationResourceModel
	var state OrganizationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use state's project_id if plan doesn't have one (e.g., after import)
	projectID := r.resolveProjectID(plan.ProjectID)
	if projectID == "" {
		projectID = r.resolveProjectID(state.ProjectID)
	}

	var domains []string
	if !plan.Domains.IsNull() && !plan.Domains.IsUnknown() {
		resp.Diagnostics.Append(plan.Domains.ElementsAs(ctx, &domains, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	org, err := r.client.UpdateOrganization(ctx, projectID, state.ID.ValueString(), plan.Label.ValueString(), domains)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Organization",
			"Could not update organization: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.ProjectID = types.StringValue(projectID)
	plan.Label = types.StringValue(org.GetLabel())
	// Preserve created_at from state - the API may return a slightly different timestamp format
	plan.CreatedAt = state.CreatedAt

	// Preserve null state for domains if it was null in the plan
	if plan.Domains.IsNull() {
		// Keep it null - user didn't specify domains
	} else if len(org.Domains) > 0 {
		domainList, diags := types.ListValueFrom(ctx, types.StringType, org.Domains)
		resp.Diagnostics.Append(diags...)
		plan.Domains = domainList
	} else {
		// User specified domains (possibly empty list), API returned empty
		plan.Domains = types.ListValueMust(types.StringType, []attr.Value{})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *OrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OrganizationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(state.ProjectID)

	err := r.client.DeleteOrganization(ctx, projectID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Organization",
			"Could not delete organization: "+err.Error(),
		)
		return
	}
}

func (r *OrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: project_id/org_id or just org_id (uses provider's project_id)
	id := req.ID
	var projectID, orgID string

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		projectID = parts[0]
		orgID = parts[1]
	} else {
		orgID = id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), orgID)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}

func (r *OrganizationResource) resolveProjectID(tfProjectID types.String) string {
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() {
		return tfProjectID.ValueString()
	}
	return r.client.ProjectID()
}
