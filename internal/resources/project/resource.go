package project

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ProjectResource{}
	_ resource.ResourceWithConfigure   = &ProjectResource{}
	_ resource.ResourceWithImportState = &ProjectResource{}
)

// NewResource returns a new Project resource.
func NewResource() resource.Resource {
	return &ProjectResource{}
}

// ProjectResource defines the resource implementation.
type ProjectResource struct {
	client *client.OryClient
}

// ProjectResourceModel describes the resource data model.
type ProjectResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Environment types.String `tfsdk:"environment"`
	HomeRegion  types.String `tfsdk:"home_region"`
	Slug        types.String `tfsdk:"slug"`
	State       types.String `tfsdk:"state"`
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network project.",
		MarkdownDescription: `
Manages an Ory Network project.

Projects are the top-level resource in Ory Network. Each project has its own
identity service, OAuth2 server, and configuration.

## Example Usage

` + "```hcl" + `
resource "ory_project" "main" {
  name        = "My Application"
  environment = "prod"
  home_region = "eu-central"
}
` + "```" + `

## Environment Types

| Environment | Description | B2B Organizations |
|-------------|-------------|-------------------|
| ` + "`prod`" + ` | Production environment with full features | Supported |
| ` + "`stage`" + ` | Staging environment for testing | Supported |
| ` + "`dev`" + ` | Development environment with limited features | **Not supported** |

**Important:** If you plan to use ` + "`ory_organization`" + ` resources, you must use ` + "`prod`" + ` or ` + "`stage`" + ` environment.
The ` + "`dev`" + ` environment does not support B2B features.

## Home Region

| Region | Description |
|--------|-------------|
| ` + "`eu-central`" + ` | Europe (Frankfurt) - Default |
| ` + "`us-east`" + ` | US East (N. Virginia) |
| ` + "`us-west`" + ` | US West (Oregon) |
| ` + "`us`" + ` | US (legacy) |
| ` + "`asia-northeast`" + ` | Asia Pacific (Tokyo) |
| ` + "`global`" + ` | Global (multi-region) |

**Note:** Home region cannot be changed after project creation.

## Import

Projects can be imported using their ID:

` + "```shell" + `
terraform import ory_project.main <project-id>
` + "```" + `

After import, you can reference the computed outputs:

` + "```hcl" + `
output "project_slug" {
  value = ory_project.main.slug
}

output "project_state" {
  value = ory_project.main.state
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "The unique identifier of the project.",
				MarkdownDescription: "The unique identifier of the project (UUID format).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description:         "The name of the project.",
				MarkdownDescription: "The display name of the project. This is shown in the Ory Console.",
				Required:            true,
			},
			"environment": schema.StringAttribute{
				Description:         "The environment type: prod, stage, or dev. Defaults to prod. Cannot be changed after creation.",
				MarkdownDescription: "The environment type. Must be one of: `prod` (production), `stage` (staging), or `dev` (development). Defaults to `prod`. **Cannot be changed after creation** - changing this will force a new resource. Note: `dev` environment does not support B2B Organizations.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("prod"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"home_region": schema.StringAttribute{
				Description:         "The home region of the project. Defaults to eu-central. Cannot be changed after creation.",
				MarkdownDescription: "The home region where the project data is stored. Must be one of: `eu-central` (Europe), `us-east`, `us-west`, `us`, `asia-northeast`, or `global`. Defaults to `eu-central`. **Cannot be changed after creation** - changing this will force a new resource.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("eu-central"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				Description:         "The project slug (e.g., 'vibrant-moore-abc123'). Auto-generated by Ory.",
				MarkdownDescription: "The project slug (e.g., `vibrant-moore-abc123`). This is auto-generated by Ory and used in API URLs. Use this value for `ORY_PROJECT_SLUG` or `project_slug` in provider configuration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description:         "The project state (e.g., 'running').",
				MarkdownDescription: "The project state. Typically `running` for active projects.",
				Computed:            true,
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, httpResp, err := r.client.CreateProject(ctx, plan.Name.ValueString(), plan.Environment.ValueString(), plan.HomeRegion.ValueString())
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 402 {
			resp.Diagnostics.AddError(
				"Payment Required",
				"Your Ory Network plan does not allow creating additional projects. "+
					"Please upgrade your plan at https://console.ory.sh.",
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Creating Project",
			"Could not create project: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(project.GetId())
	plan.Name = types.StringValue(project.GetName())
	plan.Environment = types.StringValue(project.GetEnvironment())
	plan.HomeRegion = types.StringValue(project.GetHomeRegion())
	plan.Slug = types.StringValue(project.GetSlug())
	plan.State = types.StringValue(project.GetState())

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.client.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Project",
			"Could not read project ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.Name = types.StringValue(project.GetName())
	state.Environment = types.StringValue(project.GetEnvironment())
	state.HomeRegion = types.StringValue(project.GetHomeRegion())
	state.Slug = types.StringValue(project.GetSlug())
	state.State = types.StringValue(project.GetState())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProjectResourceModel
	var state ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update project name if changed
	if !plan.Name.Equal(state.Name) {
		patches := []ory.JsonPatch{
			{Op: "replace", Path: "/name", Value: plan.Name.ValueString()},
		}
		_, err := r.client.PatchProject(ctx, state.ID.ValueString(), patches)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Project",
				"Could not update project name: "+err.Error(),
			)
			return
		}
	}

	// Read back the current state
	project, err := r.client.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Project",
			"Could not read project after update: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.Name = types.StringValue(project.GetName())
	plan.Slug = types.StringValue(project.GetSlug())
	plan.State = types.StringValue(project.GetState())

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteProject(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Project",
			"Could not delete project: "+err.Error(),
		)
		return
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
