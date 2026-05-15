package emailtemplate

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	pathpkg "path"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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

const emailTemplateMarkdownDescription = `Manages an Ory Network email template.

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
`

func (r *EmailTemplateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Network email template.",
		MarkdownDescription: emailTemplateMarkdownDescription,
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

// isHTTPSURL reports whether `s` parses as an https:// URL with a host.
// We deliberately only accept https — http (and other schemes like ftp,
// file, gopher, base64) must never be passed to the network fetcher, both
// to keep SSRF surface area minimal and to match how the Ory backoffice
// returns template content URLs.
func isHTTPSURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// urlContentFetcher fetches template content from a URL returned by the API.
// It is a package-level variable so tests can stub it out. The default
// implementation delegates to client.FetchSafeHTTPS which rejects non-HTTPS
// schemes, refuses private/loopback hosts (with DNS-rebind protection at
// connect time), limits redirects, and caps the response body at 1 MiB.
var urlContentFetcher = func(ctx context.Context, rawURL string) (string, error) {
	body, err := client.FetchSafeHTTPS(ctx, "email template", rawURL)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// urlHashMatchesContent reports whether the SHA-512 hex hash embedded in the
// URL's filename matches the SHA-512 of the supplied content. The Ory backoffice
// stores template values at storage URLs whose filename is the lowercase hex
// SHA-512 of the raw content plus a known extension (.txt/.html). When the
// hashes match we know the API value equals `content` without an extra fetch.
func urlHashMatchesContent(rawURL, content string) bool {
	hash, ok := hashFromURL(rawURL)
	if !ok {
		return false
	}
	sum := sha512.Sum512([]byte(content))
	return strings.EqualFold(hash, hex.EncodeToString(sum[:]))
}

// hashFromURL extracts the hex hash portion from a storage URL filename.
// Returns ("", false) if the URL is not in the expected shape.
func hashFromURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	base := pathpkg.Base(u.Path)
	ext := pathpkg.Ext(base)
	hash := strings.TrimSuffix(base, ext)
	if len(hash) != 128 {
		return "", false
	}
	for _, c := range hash {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return "", false
		}
	}
	return hash, true
}

// resolveStoredTemplate returns the canonical decoded template value for a
// field returned by the API. Behavior:
//   - If the API returned an empty value, return "" so an out-of-band reset
//     (Console clearing the field) overwrites stale state instead of silently
//     hiding the drift.
//   - If the API returned a literal value (e.g. `base64://...` or plain text),
//     decode and return it.
//   - If the API returned an HTTPS storage URL and its hash matches the
//     current state value, return the state value unchanged (no drift).
//   - Otherwise the content has drifted: fetch the URL safely and return the
//     body. On fetch failure we degrade gracefully back to the previous state
//     value so a transient network error doesn't crash the read.
func resolveStoredTemplate(ctx context.Context, apiValue, stateValue string) string {
	if apiValue == "" {
		return ""
	}
	// `base64://...` parses as a URL (scheme + opaque host) but it is a literal
	// payload, not a fetchable resource. Decode it directly.
	if strings.HasPrefix(apiValue, "base64://") {
		return decodeTemplate(apiValue)
	}
	if !isHTTPSURL(apiValue) {
		return apiValue
	}
	if stateValue != "" && urlHashMatchesContent(apiValue, stateValue) {
		return stateValue
	}
	fetched, err := urlContentFetcher(ctx, apiValue)
	if err != nil {
		tflog.Warn(ctx, "Falling back to previous email template state value: fetch failed",
			map[string]interface{}{"url": apiValue, "error": err.Error()})
		return stateValue
	}
	return fetched
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

	// Prefer the cached project from the last PatchProject response. The
	// Ory API has eventual consistency: GetProject called right after a patch
	// can return a stale revision (with the URL of the pre-patch content),
	// and the URL fetch would then write the wrong template body back into
	// state. The cache is only populated after our own writes, so falling
	// back to GetProject still picks up real drift made via the Console.
	var project *ory.Project
	if cached := r.client.GetCachedProject(projectID); cached != nil {
		project = cached
	} else {
		var err error
		project, err = r.client.GetProject(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Email Template", err.Error())
			return
		}
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

	// Read subject. The API returns either the literal value or a storage URL
	// whose filename is sha512(content); resolveStoredTemplate compares hashes
	// to detect drift without an extra fetch when the value is unchanged.
	//
	// We capture the previous state values, then overwrite them unconditionally
	// — a missing or empty API field must clear state (Console reset is a real
	// drift case), and the hash-match fast-path inside resolveStoredTemplate
	// uses the captured previous value when the API returned a URL.
	prevSubject := state.Subject.ValueString()
	prevHTML := state.BodyHTML.ValueString()
	prevPlaintext := state.BodyPlaintext.ValueString()

	apiSubject, _ := email["subject"].(string)
	state.Subject = types.StringValue(resolveStoredTemplate(ctx, apiSubject, prevSubject))

	body, _ := email["body"].(map[string]interface{})
	apiHTML, _ := body["html"].(string)
	apiPlaintext, _ := body["plaintext"].(string)
	state.BodyHTML = types.StringValue(resolveStoredTemplate(ctx, apiHTML, prevHTML))
	state.BodyPlaintext = types.StringValue(resolveStoredTemplate(ctx, apiPlaintext, prevPlaintext))

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

	// `add` acts as upsert for object members per RFC 6902, so it succeeds
	// whether the path already exists (drift recovery) or not (first apply
	// after the template was reset via Console).
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

	patches := []ory.JsonPatch{{
		Op:   "remove",
		Path: basePath,
	}}

	if _, err := r.client.PatchProject(ctx, projectID, patches); err != nil {
		// A 404-ish "path does not exist" is fine — the template is already
		// gone (e.g. never had subject/body customized). Anything else
		// (including a 409/revision conflict) must surface so the user is not
		// left thinking the resource was cleaned up when it wasn't.
		//
		// The Ory SDK's GenericOpenAPIError.Error() only contains the HTTP
		// status, so the "does not exist" detail lives in .Body(). Inspect
		// both before deciding to tolerate the error.
		if !isPathAlreadyGoneError(err) {
			resp.Diagnostics.AddError("Error Deleting Email Template", err.Error())
			return
		}
		tflog.Debug(ctx, "Email template path already absent during delete",
			map[string]interface{}{"path": basePath, "error": err.Error()})
	}
}

// isPathAlreadyGoneError reports whether a PatchProject error indicates the
// target path is already absent (404, or a JSON body that mentions the path
// does not exist / pointer reference not found). Anything else — auth,
// validation, conflict — should bubble up to the user.
func isPathAlreadyGoneError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *ory.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		body := strings.ToLower(string(apiErr.Body()))
		if strings.Contains(body, "does not exist") ||
			strings.Contains(body, "not found") ||
			strings.Contains(body, "no such path") ||
			strings.Contains(body, "unable to find a value") {
			return true
		}
	}
	// Fall back to the error string; both `Error()` on the SDK error and any
	// wrapper produced by retryWithBackoff include the HTTP status.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "does not exist")
}

func (r *EmailTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("template_type"), req.ID)...)
}
