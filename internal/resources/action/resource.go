package action

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// Constants for repeated string values
const (
	defaultHTTPMethod = "POST"
	defaultAuthMethod = "password"
	timingBefore      = "before"
)

var (
	_ resource.Resource                = &ActionResource{}
	_ resource.ResourceWithConfigure   = &ActionResource{}
	_ resource.ResourceWithImportState = &ActionResource{}
)

func NewResource() resource.Resource {
	return &ActionResource{}
}

type ActionResource struct {
	client *client.OryClient
}

type ActionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	Flow           types.String `tfsdk:"flow"`
	Timing         types.String `tfsdk:"timing"`
	AuthMethod     types.String `tfsdk:"auth_method"`
	URL            types.String `tfsdk:"url"`
	HTTPMethod     types.String `tfsdk:"method"`
	Body           types.String `tfsdk:"body"`
	ResponseIgnore types.Bool   `tfsdk:"response_ignore"`
	ResponseParse  types.Bool   `tfsdk:"response_parse"`
	CanInterrupt   types.Bool   `tfsdk:"can_interrupt"`
}

func (r *ActionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

func (r *ActionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Action (webhook) for identity flows.",
		MarkdownDescription: `
Manages an Ory Action (webhook) for identity flows.

Actions allow you to trigger webhooks at specific points in identity flows (login, registration, etc.).

## Example Usage

` + "```hcl" + `
# Post-registration webhook for password signups
resource "ory_action" "welcome_email" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/welcome"
  method      = "POST"
}

# Post-registration webhook for social (OIDC) signups
resource "ory_action" "social_signup" {
  flow        = "registration"
  timing      = "after"
  auth_method = "oidc"
  url         = "https://api.example.com/webhooks/social-signup"
  method      = "POST"
}
` + "```" + `

## Authentication Methods

The ` + "`auth_method`" + ` attribute specifies which authentication method triggers the webhook. In the Ory Console UI, this is the "Method" selector.

| Value | Description | UI Equivalent |
|-------|-------------|---------------|
| ` + "`password`" + ` | Password-based authentication (default) | "Password" |
| ` + "`oidc`" + ` | Social/OIDC authentication (Google, GitHub, etc.) | "Social Sign-In" |
| ` + "`code`" + ` | One-time code (magic link, OTP) | "Code" |
| ` + "`webauthn`" + ` | Hardware security keys | "WebAuthn" |
| ` + "`passkey`" + ` | Passkey authentication | "Passkey" |
| ` + "`totp`" + ` | Time-based one-time password | "TOTP" |
| ` + "`lookup_secret`" + ` | Recovery/backup codes | "Backup Codes" |

**Note:** ` + "`auth_method`" + ` is only used for ` + "`timing = \"after\"`" + ` webhooks. For ` + "`timing = \"before\"`" + ` hooks, the webhook runs before any authentication method.

## Import

Actions use a composite ID format: ` + "`project_id:flow:timing:auth_method:url`" + `

` + "```shell" + `
terraform import ory_action.welcome_email "550e8400-e29b-41d4-a716-446655440000:registration:after:password:https://api.example.com/webhooks/welcome"
` + "```" + `

### Finding Import Values from Ory Console

1. **project_id**: Settings → General → Project ID
2. **flow**: The flow type shown in Actions page (login, registration, recovery, settings, verification)
3. **timing**: "Before" or "After" as shown in the action configuration
4. **auth_method**: The authentication method selected (defaults to "password" if not explicitly set)
5. **url**: The exact webhook URL - must match exactly including trailing slashes

### Common Import Issues

- **"Cannot import non-existent remote object"**: Verify all 5 components match exactly what's configured in Ory
- **URL mismatch**: Ensure the URL matches exactly, including protocol (https://) and any trailing slashes
- **auth_method not matching**: Actions created via UI default to "password" if not explicitly selected
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID.",
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
					stringplanmodifier.RequiresReplace(),
				},
			},
			"flow": schema.StringAttribute{
				Description: "Identity flow to hook into (login, registration, recovery, settings, verification).",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("login", "registration", "recovery", "settings", "verification"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"timing": schema.StringAttribute{
				Description: "When to trigger: 'before' (pre-hook) or 'after' (post-hook).",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("before", "after"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auth_method": schema.StringAttribute{
				Description:         "Authentication method to hook into (password, oidc, code, webauthn, passkey, totp, lookup_secret). Required for 'after' timing. Defaults to 'password'.",
				MarkdownDescription: "Authentication method that triggers the webhook. In the Ory Console UI, this is the \"Method\" selector. Valid values: `password` (default), `oidc` (social login), `code` (magic link/OTP), `webauthn`, `passkey`, `totp`, `lookup_secret`. Only used for `timing = \"after\"` webhooks.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("password"),
				Validators: []validator.String{
					stringvalidator.OneOf("password", "oidc", "code", "webauthn", "passkey", "totp", "lookup_secret"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				Description: "Webhook URL to call.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"method": schema.StringAttribute{
				Description: "HTTP method (default: POST).",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("POST"),
			},
			"body": schema.StringAttribute{
				Description: "Jsonnet template for the request body.",
				Optional:    true,
			},
			"response_ignore": schema.BoolAttribute{
				Description: "Run webhook async without waiting (default: false).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"response_parse": schema.BoolAttribute{
				Description: "Parse response to modify identity (default: false).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"can_interrupt": schema.BoolAttribute{
				Description: "Allow webhook to interrupt/block the flow (default: false).",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func (r *ActionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ActionResource) buildHookValue(plan *ActionResourceModel) map[string]interface{} {
	hookConfig := map[string]interface{}{
		"url":    plan.URL.ValueString(),
		"method": plan.HTTPMethod.ValueString(),
	}

	if !plan.Body.IsNull() && !plan.Body.IsUnknown() && plan.Body.ValueString() != "" {
		body := plan.Body.ValueString()
		if !strings.HasPrefix(body, "base64://") {
			encoded := base64.StdEncoding.EncodeToString([]byte(body))
			body = "base64://" + encoded
		}
		hookConfig["body"] = body
	}

	response := map[string]interface{}{}
	if !plan.ResponseIgnore.IsNull() && !plan.ResponseIgnore.IsUnknown() {
		response["ignore"] = plan.ResponseIgnore.ValueBool()
	}
	if !plan.ResponseParse.IsNull() && !plan.ResponseParse.IsUnknown() {
		response["parse"] = plan.ResponseParse.ValueBool()
	}
	if len(response) > 0 {
		hookConfig["response"] = response
	}

	if !plan.CanInterrupt.IsNull() && !plan.CanInterrupt.IsUnknown() {
		hookConfig["can_interrupt"] = plan.CanInterrupt.ValueBool()
	}

	return map[string]interface{}{
		"hook":   "web_hook",
		"config": hookConfig,
	}
}

func (r *ActionResource) getHooks(ctx context.Context, projectID, flow, timing, authMethod string) ([]map[string]interface{}, error) {
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}

	if project.Services.Identity == nil {
		return []map[string]interface{}{}, nil
	}

	configMap := project.Services.Identity.Config
	if configMap == nil {
		return []map[string]interface{}{}, nil
	}

	selfservice, ok := configMap["selfservice"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	flows, ok := selfservice["flows"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	flowConfig, ok := flows[flow].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	timingConfig, ok := flowConfig[timing].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	// For 'after' timing, hooks are nested under the auth method
	// For 'before' timing, hooks are directly under timing
	var hooks []interface{}
	if timing == "after" {
		authMethodConfig, ok := timingConfig[authMethod].(map[string]interface{})
		if !ok {
			return []map[string]interface{}{}, nil
		}
		hooks, _ = authMethodConfig["hooks"].([]interface{})
	} else {
		hooks, _ = timingConfig["hooks"].([]interface{})
	}

	result := make([]map[string]interface{}, 0, len(hooks))
	for _, h := range hooks {
		if hm, ok := h.(map[string]interface{}); ok {
			result = append(result, hm)
		}
	}
	return result, nil
}

func (r *ActionResource) findHookIndex(hooks []map[string]interface{}, url, method string) int {
	for i, hook := range hooks {
		if hook["hook"] == "web_hook" {
			config, _ := hook["config"].(map[string]interface{})
			hookURL, _ := config["url"].(string)
			hookMethod, _ := config["method"].(string)
			if hookMethod == "" {
				hookMethod = defaultHTTPMethod
			}
			// If method is empty (e.g., during import), match by URL only
			if hookURL == url && (method == "" || hookMethod == method) {
				return i
			}
		}
	}
	return -1
}

func (r *ActionResource) hookPath(flow, timing, authMethod string) string {
	if timing == "after" {
		return fmt.Sprintf("/services/identity/config/selfservice/flows/%s/%s/%s/hooks", flow, timing, authMethod)
	}
	return fmt.Sprintf("/services/identity/config/selfservice/flows/%s/%s/hooks", flow, timing)
}

func (r *ActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ActionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	flow := plan.Flow.ValueString()
	timing := plan.Timing.ValueString()
	authMethod := plan.AuthMethod.ValueString()
	url := plan.URL.ValueString()
	httpMethod := plan.HTTPMethod.ValueString()

	// Check if hook already exists
	hooks, err := r.getHooks(ctx, projectID, flow, timing, authMethod)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Hooks", err.Error())
		return
	}

	if r.findHookIndex(hooks, url, httpMethod) >= 0 {
		resp.Diagnostics.AddError("Hook Already Exists",
			fmt.Sprintf("A webhook already exists for %s/%s/%s with URL %s", flow, timing, authMethod, url))
		return
	}

	hookValue := r.buildHookValue(&plan)
	hookPath := r.hookPath(flow, timing, authMethod)

	// Append the new hook to existing hooks and replace the entire array
	// This handles the case where the hooks array might not exist
	newHooks := make([]interface{}, 0, len(hooks)+1)
	for _, h := range hooks {
		newHooks = append(newHooks, h)
	}
	newHooks = append(newHooks, hookValue)

	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  hookPath,
		Value: newHooks,
	}}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Action", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s:%s:%s:%s", projectID, flow, timing, authMethod, url))
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ActionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	flow := state.Flow.ValueString()
	timing := state.Timing.ValueString()
	authMethod := state.AuthMethod.ValueString()
	url := state.URL.ValueString()
	httpMethod := state.HTTPMethod.ValueString()

	// Retry logic for eventual consistency after create/update
	var hooks []map[string]interface{}
	var index int
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		hooks, err = r.getHooks(ctx, projectID, flow, timing, authMethod)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Action", err.Error())
			return
		}

		index = r.findHookIndex(hooks, url, httpMethod)
		if index >= 0 {
			break
		}

		// Wait before retry (exponential backoff: 1s, 2s, 4s, 8s)
		if attempt < 4 {
			select {
			case <-ctx.Done():
				resp.State.RemoveResource(ctx)
				return
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}

	if index < 0 {
		// Build a helpful error message showing what hooks exist
		var foundHooks []string
		for _, hook := range hooks {
			if hook["hook"] == "web_hook" {
				config, _ := hook["config"].(map[string]interface{})
				hookURL, _ := config["url"].(string)
				hookMethod, _ := config["method"].(string)
				if hookMethod == "" {
					hookMethod = defaultHTTPMethod
				}
				foundHooks = append(foundHooks, fmt.Sprintf("  - %s %s", hookMethod, hookURL))
			}
		}

		if len(foundHooks) > 0 {
			resp.Diagnostics.AddWarning(
				"Action Not Found - Resource Removed From State",
				fmt.Sprintf("No webhook found matching:\n  URL: %s\n  Method: %s\n  Flow: %s/%s/%s\n\n"+
					"Webhooks found at this location:\n%s\n\n"+
					"Make sure the URL matches exactly (including protocol and trailing slashes).",
					url, httpMethod, flow, timing, authMethod, strings.Join(foundHooks, "\n")))
		}
		resp.State.RemoveResource(ctx)
		return
	}

	// Read the actual values from the hook configuration
	hook := hooks[index]
	config, _ := hook["config"].(map[string]interface{})

	// Read method (default to POST if not set)
	if method, ok := config["method"].(string); ok && method != "" {
		state.HTTPMethod = types.StringValue(method)
	} else {
		state.HTTPMethod = types.StringValue(defaultHTTPMethod)
	}

	// Read body - decode from base64 if needed
	// Note: The API may return a URL reference to stored jsonnet instead of the actual content.
	// In that case, we preserve the user's configured value to avoid drift.
	if body, ok := config["body"].(string); ok && body != "" {
		if strings.HasPrefix(body, "base64://") {
			// User-provided content stored as base64 - decode it
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(body, "base64://"))
			if err == nil {
				state.Body = types.StringValue(string(decoded))
			} else {
				// Decoding failed, keep as-is
				state.Body = types.StringValue(body)
			}
		} else if strings.HasPrefix(body, "http://") || strings.HasPrefix(body, "https://") {
			// API returned a URL reference to stored jsonnet content.
			// Don't overwrite the user's configured body to avoid drift.
			// The body remains as whatever the user configured (or null if not set).
		} else {
			// Plain text body
			state.Body = types.StringValue(body)
		}
	}

	// Read response settings
	if response, ok := config["response"].(map[string]interface{}); ok {
		if ignore, ok := response["ignore"].(bool); ok {
			state.ResponseIgnore = types.BoolValue(ignore)
		} else {
			state.ResponseIgnore = types.BoolValue(false)
		}
		if parse, ok := response["parse"].(bool); ok {
			state.ResponseParse = types.BoolValue(parse)
		} else {
			state.ResponseParse = types.BoolValue(false)
		}
	} else {
		state.ResponseIgnore = types.BoolValue(false)
		state.ResponseParse = types.BoolValue(false)
	}

	// Read can_interrupt
	if canInterrupt, ok := config["can_interrupt"].(bool); ok {
		state.CanInterrupt = types.BoolValue(canInterrupt)
	} else {
		state.CanInterrupt = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ActionResourceModel
	var state ActionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	flow := plan.Flow.ValueString()
	timing := plan.Timing.ValueString()
	authMethod := plan.AuthMethod.ValueString()
	url := state.URL.ValueString() // Use old URL to find
	httpMethod := state.HTTPMethod.ValueString()

	hooks, err := r.getHooks(ctx, projectID, flow, timing, authMethod)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Hooks", err.Error())
		return
	}

	index := r.findHookIndex(hooks, url, httpMethod)
	if index < 0 {
		resp.Diagnostics.AddError("Hook Not Found",
			fmt.Sprintf("Hook not found at %s/%s/%s with URL %s", flow, timing, authMethod, url))
		return
	}

	hookValue := r.buildHookValue(&plan)
	hookPath := r.hookPath(flow, timing, authMethod)

	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  fmt.Sprintf("%s/%d", hookPath, index),
		Value: hookValue,
	}}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Action", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s:%s:%s:%s:%s", projectID, flow, timing, authMethod, plan.URL.ValueString()))
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ActionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ActionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	flow := state.Flow.ValueString()
	timing := state.Timing.ValueString()
	authMethod := state.AuthMethod.ValueString()
	url := state.URL.ValueString()
	httpMethod := state.HTTPMethod.ValueString()

	hooks, err := r.getHooks(ctx, projectID, flow, timing, authMethod)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Hooks", err.Error())
		return
	}

	index := r.findHookIndex(hooks, url, httpMethod)
	if index < 0 {
		return // Already deleted
	}

	hookPath := r.hookPath(flow, timing, authMethod)
	patches := []ory.JsonPatch{{
		Op:   "remove",
		Path: fmt.Sprintf("%s/%d", hookPath, index),
	}}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Action", err.Error())
		return
	}
}

func (r *ActionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format includes HTTP method to support non-POST webhooks:
	// - For "after" timing: project_id:flow:after:auth_method:method:url (6 parts)
	// - For "before" timing: project_id:flow:before:method:url (5 parts)
	//
	// Legacy formats without method are still supported (defaults to POST):
	// - For "after" timing: project_id:flow:after:auth_method:url (5 parts)
	// - For "before" timing: project_id:flow:before:url (4 parts)
	parts := strings.SplitN(req.ID, ":", 6)

	var projectID, flow, timing, authMethod, httpMethod, url string

	switch len(parts) {
	case 4:
		// Legacy 4-part format: project_id:flow:before:url (for "before" timing only)
		projectID = parts[0]
		flow = parts[1]
		timing = parts[2]
		url = parts[3]

		if timing != timingBefore {
			resp.Diagnostics.AddError("Invalid Import ID",
				"4-part import format (project_id:flow:timing:url) is only valid for 'before' timing.\n"+
					"For 'after' timing, use: project_id:flow:after:auth_method:method:url")
			return
		}
		authMethod = defaultAuthMethod // Default, not used for "before" timing
		httpMethod = defaultHTTPMethod // Default
	case 5:
		// Could be:
		// - Legacy 5-part for "after": project_id:flow:after:auth_method:url (no method, defaults to POST)
		// - New 5-part for "before": project_id:flow:before:method:url
		projectID = parts[0]
		flow = parts[1]
		timing = parts[2]

		if timing == timingBefore {
			// New format: project_id:flow:before:method:url
			httpMethod = parts[3]
			url = parts[4]
			authMethod = defaultAuthMethod // Default, not used for "before" timing

			// Validate HTTP method
			if httpMethod != "POST" && httpMethod != "GET" && httpMethod != "PUT" && httpMethod != "PATCH" && httpMethod != "DELETE" {
				resp.Diagnostics.AddError("Invalid Import ID",
					fmt.Sprintf("Invalid HTTP method '%s'. Must be one of: POST, GET, PUT, PATCH, DELETE", httpMethod))
				return
			}
		} else {
			// Legacy format: project_id:flow:after:auth_method:url (no method)
			authMethod = parts[3]
			url = parts[4]
			httpMethod = defaultHTTPMethod // Default

			// Allow "_" or "none" as placeholder for auth_method
			if authMethod == "_" || authMethod == "none" || authMethod == "" {
				authMethod = defaultAuthMethod
			}
		}
	case 6:
		// New 6-part format for "after": project_id:flow:after:auth_method:method:url
		projectID = parts[0]
		flow = parts[1]
		timing = parts[2]
		authMethod = parts[3]
		httpMethod = parts[4]
		url = parts[5]

		if timing == timingBefore {
			resp.Diagnostics.AddError("Invalid Import ID",
				"6-part import format is only valid for 'after' timing.\n"+
					"For 'before' timing, use: project_id:flow:before:method:url")
			return
		}

		// Validate HTTP method
		if httpMethod != "POST" && httpMethod != "GET" && httpMethod != "PUT" && httpMethod != "PATCH" && httpMethod != "DELETE" {
			resp.Diagnostics.AddError("Invalid Import ID",
				fmt.Sprintf("Invalid HTTP method '%s'. Must be one of: POST, GET, PUT, PATCH, DELETE", httpMethod))
			return
		}

		// Allow "_" or "none" as placeholder for auth_method
		if authMethod == "_" || authMethod == "none" || authMethod == "" {
			authMethod = defaultAuthMethod
		}
	default:
		resp.Diagnostics.AddError("Invalid Import ID",
			"Import ID must be in one of these formats:\n"+
				"  - For 'after' timing: project_id:flow:after:auth_method:method:url\n"+
				"  - For 'before' timing: project_id:flow:before:method:url\n\n"+
				"Examples:\n"+
				"  550e8400-...:registration:after:password:POST:https://api.example.com/webhook\n"+
				"  550e8400-...:login:before:PATCH:https://api.example.com/validate")
		return
	}

	// Construct the full ID for state
	fullID := fmt.Sprintf("%s:%s:%s:%s:%s", projectID, flow, timing, authMethod, url)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fullID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("flow"), flow)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("timing"), timing)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("auth_method"), authMethod)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("url"), url)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("method"), httpMethod)...)
}
