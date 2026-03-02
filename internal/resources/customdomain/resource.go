package customdomain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &CustomDomainResource{}
	_ resource.ResourceWithConfigure   = &CustomDomainResource{}
	_ resource.ResourceWithImportState = &CustomDomainResource{}
)

// NewResource returns a new CustomDomain resource.
func NewResource() resource.Resource {
	return &CustomDomainResource{}
}

// CustomDomainResource defines the resource implementation.
type CustomDomainResource struct {
	client *client.OryClient
}

const customDomainMarkdownDescription = `
Manages an Ory Network custom domain.

Custom domains allow you to expose Ory APIs on your own domain (e.g.,
` + "`auth.example.com`" + `) instead of the default Ory domain. This enables
setting session cookies on your own domain for seamless cross-subdomain
authentication.

When you configure a custom domain with a ` + "`cookie_domain`" + `, the Ory reverse
proxy rewrites session cookies to use that domain. This is the correct way to
share session cookies across subdomains — the ` + "`session.cookie.domain`" + `
setting in project config is not user-configurable on Ory Network.

~> **Important:** Custom domains require DNS configuration. After creating a
custom domain, you must add a CNAME record pointing your hostname to Ory.
Check the ` + "`verification_status`" + ` attribute to monitor DNS verification progress.

## Example Usage

` + "```hcl" + `
resource "ory_custom_domain" "auth" {
  hostname      = "auth.example.com"
  cookie_domain = "example.com"

  cors_enabled         = true
  cors_allowed_origins = ["https://app.example.com"]
  custom_ui_base_url   = "https://app.example.com/auth"
}
` + "```" + `

## Import

Custom domains can be imported using the format ` + "`project_id/custom_domain_id`" + `:

` + "```shell" + `
terraform import ory_custom_domain.auth <project-id>/<custom-domain-id>
` + "```" + `
`

// CustomDomainResourceModel describes the resource data model.
type CustomDomainResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ProjectID          types.String `tfsdk:"project_id"`
	Hostname           types.String `tfsdk:"hostname"`
	CookieDomain       types.String `tfsdk:"cookie_domain"`
	CorsAllowedOrigins types.List   `tfsdk:"cors_allowed_origins"`
	CorsEnabled        types.Bool   `tfsdk:"cors_enabled"`
	CustomUIBaseURL    types.String `tfsdk:"custom_ui_base_url"`
	VerificationStatus types.String `tfsdk:"verification_status"`
	VerificationErrors types.List   `tfsdk:"verification_errors"`
	SSLStatus          types.String `tfsdk:"ssl_status"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
}

func (r *CustomDomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_domain"
}

func (r *CustomDomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Network custom domain.",
		MarkdownDescription: customDomainMarkdownDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the custom domain.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
			"hostname": schema.StringAttribute{
				Description: "The custom hostname where the API will be exposed (e.g., 'auth.example.com').",
				Required:    true,
			},
			"cookie_domain": schema.StringAttribute{
				Description: "The domain where session cookies will be set. Must be a parent domain of the hostname (e.g., 'example.com' for hostname 'auth.example.com').",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cors_allowed_origins": schema.ListAttribute{
				Description: "CORS allowed origins for the custom hostname (max 50).",
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"cors_enabled": schema.BoolAttribute{
				Description: "Whether CORS is enabled for the custom hostname.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"custom_ui_base_url": schema.StringAttribute{
				Description: "The base URL where the custom user interface is exposed (e.g., 'https://app.example.com/auth').",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"verification_status": schema.StringAttribute{
				Description: "DNS verification status of the custom domain (e.g., 'pending', 'active').",
				Computed:    true,
			},
			"verification_errors": schema.ListAttribute{
				Description: "DNS verification errors, if any.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"ssl_status": schema.StringAttribute{
				Description: "SSL certificate status of the custom domain.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the custom domain was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the custom domain was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *CustomDomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CustomDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CustomDomainResourceModel

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

	body := ory.CreateCustomDomainBody{}
	body.SetHostname(plan.Hostname.ValueString())

	if !plan.CookieDomain.IsNull() && !plan.CookieDomain.IsUnknown() {
		body.SetCookieDomain(plan.CookieDomain.ValueString())
	}
	if !plan.CustomUIBaseURL.IsNull() && !plan.CustomUIBaseURL.IsUnknown() {
		body.SetCustomUiBaseUrl(plan.CustomUIBaseURL.ValueString())
	}
	if !plan.CorsEnabled.IsNull() && !plan.CorsEnabled.IsUnknown() {
		body.SetCorsEnabled(plan.CorsEnabled.ValueBool())
	}
	if !plan.CorsAllowedOrigins.IsNull() && !plan.CorsAllowedOrigins.IsUnknown() {
		var origins []string
		resp.Diagnostics.Append(plan.CorsAllowedOrigins.ElementsAs(ctx, &origins, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.SetCorsAllowedOrigins(origins)
	}

	domain, err := r.client.CreateCustomDomain(ctx, projectID, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Custom Domain",
			"Could not create custom domain: "+err.Error(),
		)
		return
	}

	// Re-read from List to get full object (Create response may omit computed fields like ssl_status).
	domainID := domain.GetId()
	fullDomain, err := r.client.GetCustomDomain(ctx, projectID, domainID)
	if err == nil {
		domain = fullDomain
	} else {
		resp.Diagnostics.AddWarning(
			"Partial Custom Domain State After Create",
			fmt.Sprintf("Custom domain %s was created, but a follow-up read failed: %s. Some computed fields may be incomplete until the next refresh.", domainID, err),
		)
	}

	r.mapDomainToState(ctx, domain, &plan, projectID, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *CustomDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CustomDomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(state.ProjectID)

	domain, err := r.client.GetCustomDomain(ctx, projectID, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrCustomDomainNotFound) {
			resp.Diagnostics.AddWarning(
				"Custom Domain Not Found",
				fmt.Sprintf("Custom domain %s was not found (possibly deleted outside Terraform). Removing from state.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Custom Domain",
			"Could not read custom domain ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	r.mapDomainToState(ctx, domain, &state, projectID, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CustomDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state CustomDomainResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(plan.ProjectID)
	if projectID == "" {
		projectID = r.resolveProjectID(state.ProjectID)
	}

	body := ory.SetCustomDomainBody{}
	body.SetHostname(plan.Hostname.ValueString())

	if !plan.CookieDomain.IsNull() && !plan.CookieDomain.IsUnknown() {
		body.SetCookieDomain(plan.CookieDomain.ValueString())
	}
	if !plan.CustomUIBaseURL.IsNull() && !plan.CustomUIBaseURL.IsUnknown() {
		body.SetCustomUiBaseUrl(plan.CustomUIBaseURL.ValueString())
	}
	if !plan.CorsEnabled.IsNull() && !plan.CorsEnabled.IsUnknown() {
		body.SetCorsEnabled(plan.CorsEnabled.ValueBool())
	}
	if !plan.CorsAllowedOrigins.IsNull() && !plan.CorsAllowedOrigins.IsUnknown() {
		var origins []string
		resp.Diagnostics.Append(plan.CorsAllowedOrigins.ElementsAs(ctx, &origins, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.SetCorsAllowedOrigins(origins)
	}

	domain, err := r.client.UpdateCustomDomain(ctx, projectID, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Custom Domain",
			"Could not update custom domain: "+err.Error(),
		)
		return
	}

	// Re-read from List to get full object (Update response may omit computed fields like ssl_status).
	fullDomain, err := r.client.GetCustomDomain(ctx, projectID, state.ID.ValueString())
	if err == nil {
		domain = fullDomain
	} else {
		resp.Diagnostics.AddWarning(
			"Partial Custom Domain State After Update",
			fmt.Sprintf("Custom domain %s was updated, but a follow-up read failed: %s. Some computed fields may be incomplete until the next refresh.", state.ID.ValueString(), err),
		)
	}

	r.mapDomainToState(ctx, domain, &plan, projectID, &resp.Diagnostics)
	plan.CreatedAt = state.CreatedAt // Preserve from state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *CustomDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CustomDomainResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(state.ProjectID)

	err := r.client.DeleteCustomDomain(ctx, projectID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Custom Domain",
			"Could not delete custom domain: "+err.Error(),
		)
		return
	}
}

func (r *CustomDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	var projectID, domainID string

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		projectID = parts[0]
		domainID = parts[1]
	} else {
		domainID = id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), domainID)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}

func (r *CustomDomainResource) resolveProjectID(tfProjectID types.String) string {
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() {
		return tfProjectID.ValueString()
	}
	return r.client.ProjectID()
}

// mapDomainToState maps an API CustomDomain response to the Terraform state model.
func (r *CustomDomainResource) mapDomainToState(ctx context.Context, domain *ory.CustomDomain, state *CustomDomainResourceModel, projectID string, diags *diag.Diagnostics) {
	state.ID = types.StringValue(domain.GetId())
	state.ProjectID = types.StringValue(projectID)
	state.Hostname = types.StringValue(domain.GetHostname())
	state.CookieDomain = types.StringValue(domain.GetCookieDomain())
	state.CorsEnabled = types.BoolValue(domain.GetCorsEnabled())
	state.CustomUIBaseURL = types.StringValue(domain.GetCustomUiBaseUrl())
	state.VerificationStatus = types.StringValue(domain.GetVerificationStatus())
	state.SSLStatus = types.StringValue(domain.GetSslStatus())

	if domain.HasCreatedAt() {
		state.CreatedAt = types.StringValue(domain.GetCreatedAt().String())
	}
	if domain.HasUpdatedAt() {
		state.UpdatedAt = types.StringValue(domain.GetUpdatedAt().String())
	}

	// Map list fields
	origins := domain.GetCorsAllowedOrigins()
	originValues, d := types.ListValueFrom(ctx, types.StringType, origins)
	diags.Append(d...)
	state.CorsAllowedOrigins = originValues

	verErrors := domain.GetVerificationErrors()
	errorValues, d := types.ListValueFrom(ctx, types.StringType, verErrors)
	diags.Append(d...)
	state.VerificationErrors = errorValues
}
