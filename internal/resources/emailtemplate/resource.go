package emailtemplate

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

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
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var validTemplateTypes = []string{
	"recovery_code_valid",
	"recovery_code_invalid",
	"recovery_valid",
	"recovery_invalid",
	"verification_code_valid",
	"verification_code_invalid",
	"verification_valid",
	"verification_invalid",
	"login_code_valid",
	"login_code_invalid",
	"registration_code_valid",
	"registration_code_invalid",
}

var (
	_ resource.Resource                = &EmailTemplateResource{}
	_ resource.ResourceWithConfigure   = &EmailTemplateResource{}
	_ resource.ResourceWithImportState = &EmailTemplateResource{}
)

func NewResource() resource.Resource {
	return &EmailTemplateResource{}
}

type EmailTemplateResource struct {
	client *client.OryClient
}

type EmailTemplateResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	TemplateType  types.String `tfsdk:"template_type"`
	Subject       types.String `tfsdk:"subject"`
	BodyHTML      types.String `tfsdk:"body_html"`
	BodyPlaintext types.String `tfsdk:"body_plaintext"`
}

func (r *EmailTemplateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_template"
}

func (r *EmailTemplateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network email template.",
		MarkdownDescription: `Manages an Ory Network email template.

## Template Types

| Template Type | UI Name | Description |
|---------------|---------|-------------|
| ` + "`registration_code_valid`" + ` | Registration via Code | Sent when user registers with a valid code |
| ` + "`registration_code_invalid`" + ` | - | Sent when registration code is invalid/expired |
| ` + "`login_code_valid`" + ` | Login via Code | Sent when user logs in with a valid code |
| ` + "`login_code_invalid`" + ` | - | Sent when login code is invalid/expired |
| ` + "`verification_code_valid`" + ` | Verification via Code (Valid) | Sent for email verification with valid code |
| ` + "`verification_code_invalid`" + ` | - | Sent when verification code is invalid/expired |
| ` + "`recovery_code_valid`" + ` | Recovery via Code (Valid) | Sent for account recovery with valid code |
| ` + "`recovery_code_invalid`" + ` | - | Sent when recovery code is invalid/expired |
| ` + "`verification_valid`" + ` | - | Legacy verification email (link-based) |
| ` + "`verification_invalid`" + ` | - | Legacy verification invalid |
| ` + "`recovery_valid`" + ` | - | Legacy recovery email (link-based) |
| ` + "`recovery_invalid`" + ` | - | Legacy recovery invalid |

**Note:** The "_invalid" templates are sent when a code has expired or is incorrect. The non-code variants (recovery_valid, verification_valid) are for legacy link-based flows.

## Example Usage

` + "```hcl" + `
resource "ory_email_template" "welcome" {
  template_type  = "registration_code_valid"
  subject        = "Welcome to {{ .Flow.OrganizationName }}!"
  body_html      = "<h1>Welcome!</h1><p>Your code is: {{ .VerificationCode }}</p>"
  body_plaintext = "Welcome! Your code is: {{ .VerificationCode }}"
}
` + "```" + `

## Import

` + "```shell" + `
terraform import ory_email_template.welcome registration_code_valid
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID (same as template_type).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID. If not set, uses provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"template_type": schema.StringAttribute{
				Description:         "Template type (e.g., recovery_code_valid, verification_valid).",
				MarkdownDescription: "The email template type. See the Template Types table above for valid values and their UI equivalents. Common values: `registration_code_valid`, `login_code_valid`, `verification_code_valid`, `recovery_code_valid`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(validTemplateTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject": schema.StringAttribute{
				Description: "Email subject template (Go template syntax).",
				Optional:    true,
				Computed:    true,
			},
			"body_html": schema.StringAttribute{
				Description: "HTML body template (Go template syntax).",
				Required:    true,
			},
			"body_plaintext": schema.StringAttribute{
				Description: "Plaintext body template (Go template syntax).",
				Required:    true,
			},
		},
	}
}

func (r *EmailTemplateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T", req.ProviderData))
		return
	}
	r.client = oryClient
}

func encodeTemplate(content string) string {
	return "base64://" + base64.StdEncoding.EncodeToString([]byte(content))
}

func (r *EmailTemplateResource) templatePath(templateType string) string {
	// Map template type to config path (e.g., "recovery_code_valid" -> "recovery_code/valid")
	parts := strings.Split(templateType, "_")
	if len(parts) >= 2 {
		validity := parts[len(parts)-1]
		if validity == "valid" || validity == "invalid" {
			prefix := strings.Join(parts[:len(parts)-1], "_")
			return fmt.Sprintf("%s/%s", prefix, validity)
		}
	}
	return templateType
}

func (r *EmailTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EmailTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	templatePath := r.templatePath(plan.TemplateType.ValueString())
	basePath := fmt.Sprintf("/services/identity/config/courier/templates/%s/email", templatePath)

	htmlEncoded := encodeTemplate(plan.BodyHTML.ValueString())
	plaintextEncoded := encodeTemplate(plan.BodyPlaintext.ValueString())

	patches := []ory.JsonPatch{
		{
			Op:   "add",
			Path: basePath + "/body",
			Value: map[string]string{
				"html":      htmlEncoded,
				"plaintext": plaintextEncoded,
			},
		},
	}

	if !plan.Subject.IsNull() && !plan.Subject.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "add",
			Path:  basePath + "/subject",
			Value: encodeTemplate(plan.Subject.ValueString()),
		})
	}

	_, err := r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Email Template", err.Error())
		return
	}

	plan.ID = plan.TemplateType
	plan.ProjectID = types.StringValue(projectID)

	// If subject was not set by user, set it to empty string (API default)
	if plan.Subject.IsNull() || plan.Subject.IsUnknown() {
		plan.Subject = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func decodeTemplate(content string) string {
	if strings.HasPrefix(content, "base64://") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(content, "base64://"))
		if err == nil {
			return string(decoded)
		}
	}
	return content
}

func (r *EmailTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EmailTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project to read template config
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Email Template", err.Error())
		return
	}

	if project.Services.Identity == nil || project.Services.Identity.Config == nil {
		// Template not configured, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	configMap := project.Services.Identity.Config
	courier, ok := configMap["courier"].(map[string]interface{})
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	templates, ok := courier["templates"].(map[string]interface{})
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	// Navigate to the specific template (e.g., "recovery_code/valid")
	templatePath := r.templatePath(state.TemplateType.ValueString())
	pathParts := strings.Split(templatePath, "/")

	current := templates
	for _, part := range pathParts {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			// Template not found
			resp.State.RemoveResource(ctx)
			return
		}
		current = next
	}

	email, ok := current["email"].(map[string]interface{})
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	// Read subject if present
	if subject, ok := email["subject"].(string); ok && subject != "" {
		state.Subject = types.StringValue(decodeTemplate(subject))
	}

	// Read body
	if body, ok := email["body"].(map[string]interface{}); ok {
		if html, ok := body["html"].(string); ok && html != "" {
			// Check if it's a URL reference (like with actions) - if so, preserve user's config
			if !strings.HasPrefix(html, "http://") && !strings.HasPrefix(html, "https://") {
				state.BodyHTML = types.StringValue(decodeTemplate(html))
			}
		}
		if plaintext, ok := body["plaintext"].(string); ok && plaintext != "" {
			if !strings.HasPrefix(plaintext, "http://") && !strings.HasPrefix(plaintext, "https://") {
				state.BodyPlaintext = types.StringValue(decodeTemplate(plaintext))
			}
		}
	}

	state.ProjectID = types.StringValue(projectID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EmailTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EmailTemplateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	templatePath := r.templatePath(plan.TemplateType.ValueString())
	basePath := fmt.Sprintf("/services/identity/config/courier/templates/%s/email", templatePath)

	htmlEncoded := encodeTemplate(plan.BodyHTML.ValueString())
	plaintextEncoded := encodeTemplate(plan.BodyPlaintext.ValueString())

	patches := []ory.JsonPatch{
		{
			Op:   "replace",
			Path: basePath + "/body",
			Value: map[string]string{
				"html":      htmlEncoded,
				"plaintext": plaintextEncoded,
			},
		},
	}

	if !plan.Subject.IsNull() && !plan.Subject.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  basePath + "/subject",
			Value: encodeTemplate(plan.Subject.ValueString()),
		})
	}

	_, err := r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Email Template", err.Error())
		return
	}

	plan.ID = plan.TemplateType
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *EmailTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EmailTemplateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	templatePath := r.templatePath(state.TemplateType.ValueString())
	basePath := fmt.Sprintf("/services/identity/config/courier/templates/%s", templatePath)

	// Try to remove the template (resets to Ory defaults)
	patches := []ory.JsonPatch{{
		Op:   "remove",
		Path: basePath,
	}}

	// Ignore errors - the path might not exist
	_, _ = r.client.PatchProject(ctx, projectID, patches)
}

func (r *EmailTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("template_type"), req.ID)...)
}
