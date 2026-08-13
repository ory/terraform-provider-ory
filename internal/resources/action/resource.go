package action

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// Constants for repeated string values
const (
	defaultHTTPMethod    = "POST"
	defaultAuthMethod    = "password"
	timingBefore         = "before"
	timingAfter          = "after"
	webhookAuthBasicAuth = "basic_auth"
	webhookAuthAPIKey    = "api_key"
	// projectStateDeleted is the state Ory reports for a project that has been
	// deleted; its config no longer holds any hooks.
	projectStateDeleted = "deleted"
)

var (
	_ resource.Resource                   = &ActionResource{}
	_ resource.ResourceWithConfigure      = &ActionResource{}
	_ resource.ResourceWithImportState    = &ActionResource{}
	_ resource.ResourceWithValidateConfig = &ActionResource{}
)

// projectMutexes serializes Create, Update, and Delete operations per project.
// The Ory API stores action hooks as arrays nested in the project config. Every
// mutation is a read-modify-write (read hooks → append/replace/remove → PatchProject).
// Without serialization, concurrent operations read the same stale hooks array
// and the last write wins, silently dropping the other hooks.
// See: https://github.com/ory/terraform-provider-ory/issues/189
var projectMutexes sync.Map // map[projectID]*sync.Mutex

func projectMutex(projectID string) *sync.Mutex {
	v, _ := projectMutexes.LoadOrStore(projectID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

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

	// Webhook authentication configuration
	WebhookAuthType                       types.String `tfsdk:"webhook_auth_type"`
	WebhookAuthBasicAuthUser              types.String `tfsdk:"webhook_auth_basic_auth_user"`
	WebhookAuthBasicAuthPassword          types.String `tfsdk:"webhook_auth_basic_auth_password"`
	WebhookAuthBasicAuthPasswordWO        types.String `tfsdk:"webhook_auth_basic_auth_password_wo"`
	WebhookAuthBasicAuthPasswordWOVersion types.String `tfsdk:"webhook_auth_basic_auth_password_wo_version"`
	WebhookAuthAPIKeyName                 types.String `tfsdk:"webhook_auth_api_key_name"`
	WebhookAuthAPIKeyValue                types.String `tfsdk:"webhook_auth_api_key_value"`
	WebhookAuthAPIKeyValueWO              types.String `tfsdk:"webhook_auth_api_key_value_wo"`
	WebhookAuthAPIKeyValueWOVersion       types.String `tfsdk:"webhook_auth_api_key_value_wo_version"`
	WebhookAuthAPIKeyIn                   types.String `tfsdk:"webhook_auth_api_key_in"`
}

const actionMarkdownDescription = `
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
| ` + "`profile`" + ` | Profile/trait update (settings flow only) | "Profile" |
| ` + "`webauthn`" + ` | Hardware security keys | "WebAuthn" |
| ` + "`passkey`" + ` | Passkey authentication | "Passkey" |
| ` + "`totp`" + ` | Time-based one-time password | "TOTP" |
| ` + "`lookup_secret`" + ` | Recovery/backup codes | "Backup Codes" |
| ` + "`saml`" + ` | SAML single sign-on (SSO) | "SAML" |

**Note:** ` + "`auth_method`" + ` only applies to ` + "`timing = \"after\"`" + ` webhooks on the ` + "`login`" + `, ` + "`registration`" + `, and ` + "`settings`" + ` flows. The ` + "`recovery`" + ` and ` + "`verification`" + ` flows are not scoped by authentication method, so ` + "`auth_method`" + ` is ignored for them and should be omitted. For ` + "`timing = \"before\"`" + ` hooks, the webhook runs before any authentication method.

### Method Availability by Flow

Not every method exists on every flow. The Ory API accepts a hook written to an
unsupported flow and method pair with HTTP 200 but discards it, so the provider
warns at plan time and the apply then fails verification.

| Method | ` + "`login`" + ` | ` + "`registration`" + ` | ` + "`settings`" + ` |
|--------|---------|----------------|------------|
| ` + "`password`" + ` | yes | yes | yes |
| ` + "`oidc`" + ` | yes | yes | yes |
| ` + "`code`" + ` | yes | yes | no |
| ` + "`profile`" + ` | no | no | yes |
| ` + "`webauthn`" + ` | yes | yes | yes |
| ` + "`passkey`" + ` | yes | yes | yes |
| ` + "`totp`" + ` | yes | no | yes |
| ` + "`lookup_secret`" + ` | yes | no | yes |
| ` + "`saml`" + ` | yes | yes | yes |

` + "```hcl" + `
# Post-profile-update webhook on the settings flow
resource "ory_action" "profile_updated" {
  flow        = "settings"
  timing      = "after"
  auth_method = "profile"
  url         = "https://api.example.com/webhooks/profile-updated"
  method      = "POST"
}
` + "```" + `

## Webhook Authentication

Webhooks can be configured with authentication to secure the endpoint. Two types are supported:

### Basic Auth

` + "```hcl" + `
resource "ory_action" "secured_webhook" {
  flow        = "registration"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/welcome"
  method      = "POST"

  webhook_auth_type                = "basic_auth"
  webhook_auth_basic_auth_user     = var.webhook_user
  webhook_auth_basic_auth_password = var.webhook_password
}
` + "```" + `

### API Key

` + "```hcl" + `
resource "ory_action" "api_key_webhook" {
  flow        = "login"
  timing      = "after"
  auth_method = "password"
  url         = "https://api.example.com/webhooks/login"
  method      = "POST"

  webhook_auth_type          = "api_key"
  webhook_auth_api_key_name  = "X-API-KEY"
  webhook_auth_api_key_value = var.api_key
  webhook_auth_api_key_in    = "header"
}
` + "```" + `

| Attribute | Description |
|-----------|-------------|
| ` + "`webhook_auth_type`" + ` | Authentication type: ` + "`basic_auth`" + ` or ` + "`api_key`" + ` |
| ` + "`webhook_auth_basic_auth_user`" + ` | Username for basic auth |
| ` + "`webhook_auth_basic_auth_password`" + ` | Password for basic auth (sensitive) |
| ` + "`webhook_auth_api_key_name`" + ` | Header or cookie name for the API key |
| ` + "`webhook_auth_api_key_value`" + ` | The API key value (sensitive) |
| ` + "`webhook_auth_api_key_in`" + ` | Where to send the API key: ` + "`header`" + ` or ` + "`cookie`" + ` |

## Import

Actions use a composite ID format: ` + "`project_id:flow:timing:auth_method:method:url`" + `

` + "```shell" + `
terraform import ory_action.welcome_email "550e8400-e29b-41d4-a716-446655440000:registration:after:password:POST:https://api.example.com/webhooks/welcome"
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
`

func (r *ActionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

func (r *ActionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Action (webhook) for identity flows.",
		MarkdownDescription: actionMarkdownDescription,
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
				Description:         "Authentication method to hook into (password, oidc, code, profile, webauthn, passkey, totp, lookup_secret, saml). Defaults to 'password'. Only applies to 'after' timing on the login, registration, and settings flows; ignored for the recovery and verification flows. Not every method is available on every flow: 'profile' is settings-only, 'code' is login/registration-only, and 'totp'/'lookup_secret' are login/settings-only.",
				MarkdownDescription: "Authentication method that triggers the webhook. In the Ory Console UI, this is the \"Method\" selector. Valid values: `password` (default), `oidc` (social login), `code` (magic link/OTP), `profile` (profile/trait update, settings flow only), `webauthn`, `passkey`, `totp`, `lookup_secret`, `saml` (SAML single sign-on). Only applies to `timing = \"after\"` webhooks on the `login`, `registration`, and `settings` flows; it is ignored for the `recovery` and `verification` flows. Not every method is available on every flow — see the method availability table in the resource description.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("password"),
				Validators: []validator.String{
					stringvalidator.OneOf("password", "oidc", "code", "profile", "webauthn", "passkey", "totp", "lookup_secret", "saml"),
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
			"webhook_auth_type": schema.StringAttribute{
				Description: "Webhook authentication type: 'basic_auth' or 'api_key'.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(webhookAuthBasicAuth, webhookAuthAPIKey),
				},
			},
			"webhook_auth_basic_auth_user": schema.StringAttribute{
				Description: "Username for basic auth webhook authentication.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_api_key_name"),
						path.MatchRoot("webhook_auth_api_key_value"),
						path.MatchRoot("webhook_auth_api_key_in"),
					),
				},
			},
			"webhook_auth_basic_auth_password": schema.StringAttribute{
				Description: "Password for basic auth webhook authentication. Stored in Terraform state; for an ephemeral alternative that is never persisted, use webhook_auth_basic_auth_password_wo.",
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_api_key_name"),
						path.MatchRoot("webhook_auth_api_key_value"),
						path.MatchRoot("webhook_auth_api_key_in"),
					),
				},
			},
			"webhook_auth_basic_auth_password_wo": schema.StringAttribute{
				Description: "Write-only equivalent of webhook_auth_basic_auth_password (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the password from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change webhook_auth_basic_auth_password_wo_version to rotate it. Mutually exclusive with webhook_auth_basic_auth_password.",
				Optional:    true,
				WriteOnly:   true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_basic_auth_password"),
						path.MatchRoot("webhook_auth_api_key_name"),
						path.MatchRoot("webhook_auth_api_key_value"),
						path.MatchRoot("webhook_auth_api_key_in"),
					),
				},
			},
			"webhook_auth_basic_auth_password_wo_version": schema.StringAttribute{
				Description: "Version trigger for webhook_auth_basic_auth_password_wo. Change this value whenever the write-only password changes so Terraform sends the new value to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless webhook_auth_basic_auth_password_wo is set.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_basic_auth_password_wo")),
				},
			},
			"webhook_auth_api_key_name": schema.StringAttribute{
				Description: "Header or cookie name for API key webhook authentication.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_basic_auth_user"),
						path.MatchRoot("webhook_auth_basic_auth_password"),
					),
				},
			},
			"webhook_auth_api_key_value": schema.StringAttribute{
				Description: "API key value for API key webhook authentication. Stored in Terraform state; for an ephemeral alternative that is never persisted, use webhook_auth_api_key_value_wo.",
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_basic_auth_user"),
						path.MatchRoot("webhook_auth_basic_auth_password"),
					),
				},
			},
			"webhook_auth_api_key_value_wo": schema.StringAttribute{
				Description: "Write-only equivalent of webhook_auth_api_key_value (Terraform 1.11+ write-only argument): the value is sent to Ory but never stored in Terraform state or plan. Use this to source the API key from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own — change webhook_auth_api_key_value_wo_version to rotate it. Mutually exclusive with webhook_auth_api_key_value.",
				Optional:    true,
				WriteOnly:   true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_api_key_value"),
						path.MatchRoot("webhook_auth_basic_auth_user"),
						path.MatchRoot("webhook_auth_basic_auth_password"),
						path.MatchRoot("webhook_auth_basic_auth_password_wo"),
					),
				},
			},
			"webhook_auth_api_key_value_wo_version": schema.StringAttribute{
				Description: "Version trigger for webhook_auth_api_key_value_wo. Change this value whenever the write-only API key changes so Terraform sends the new value to Ory (write-only values are not stored in state and cannot be diffed). Has no effect unless webhook_auth_api_key_value_wo is set.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_api_key_value_wo")),
				},
			},
			"webhook_auth_api_key_in": schema.StringAttribute{
				Description: "Where to send the API key: 'header' or 'cookie'.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("webhook_auth_type")),
					stringvalidator.OneOf("header", "cookie"),
					stringvalidator.ConflictsWith(
						path.MatchRoot("webhook_auth_basic_auth_user"),
						path.MatchRoot("webhook_auth_basic_auth_password"),
					),
				},
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

func (r *ActionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config ActionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// auth_method only takes effect for "after" hooks on the login, registration,
	// and settings flows. It is ignored for "before" hooks (all flows) and for the
	// recovery and verification flows, which store their after-hooks in a single
	// flat array. Warn when it is set explicitly but will have no effect, so users
	// aren't surprised. Timing is only considered when known, to avoid false
	// positives for auth-scoped flows whose timing is resolved at apply time.
	if !config.AuthMethod.IsNull() && !config.AuthMethod.IsUnknown() &&
		!config.Flow.IsNull() && !config.Flow.IsUnknown() {
		flow := config.Flow.ValueString()
		timingKnown := !config.Timing.IsNull() && !config.Timing.IsUnknown()
		ignoredByFlow := !flowSupportsAuthMethod(flow)
		ignoredByTiming := timingKnown && config.Timing.ValueString() != timingAfter
		if ignoredByFlow || ignoredByTiming {
			detail := fmt.Sprintf("The %q flow does not scope its hooks by authentication method, so "+
				"auth_method is ignored. You can safely remove auth_method from this resource.", flow)
			if !ignoredByFlow {
				detail = "auth_method only applies to \"after\" hooks, so it is ignored for \"before\" hooks. " +
					"You can safely remove auth_method from this resource."
			}
			resp.Diagnostics.AddAttributeWarning(
				path.Root("auth_method"),
				"auth_method has no effect for this configuration",
				detail,
			)
		} else if authMethod := config.AuthMethod.ValueString(); timingKnown && !flowSupportsMethod(flow, authMethod) {
			// The method passes the schema enum but this flow has no key for it, so
			// the API accepts the patch with HTTP 200 and drops the hook. Warn rather
			// than error so a config the provider misjudges (for example because a
			// later Ory release adds a method key) can still be planned; the apply
			// verification catches it either way.
			resp.Diagnostics.AddAttributeWarning(
				path.Root("auth_method"),
				"auth_method is not available on this flow",
				unsupportedMethodDetail(flow, authMethod),
			)
		}
	}

	if config.WebhookAuthType.IsNull() || config.WebhookAuthType.IsUnknown() {
		return
	}

	// Per-field unknown checks: when a value is sourced from a data source (e.g. AWS Secrets
	// Manager), it is unknown at plan time and will become known at apply time. We skip the
	// required-field check only for the specific field that is unknown, allowing validation of
	// other fields that are already known. See:
	// https://developer.hashicorp.com/terraform/plugin/framework/validation
	authType := config.WebhookAuthType.ValueString()
	switch authType {
	case webhookAuthBasicAuth:
		if !config.WebhookAuthBasicAuthUser.IsUnknown() && config.WebhookAuthBasicAuthUser.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("webhook_auth_basic_auth_user"),
				"Missing Required Attribute",
				"webhook_auth_basic_auth_user is required when webhook_auth_type is \"basic_auth\".",
			)
		}
		// The password may be supplied via webhook_auth_basic_auth_password or its
		// write-only counterpart. Only error when both are known and null.
		if !config.WebhookAuthBasicAuthPassword.IsUnknown() && config.WebhookAuthBasicAuthPassword.IsNull() &&
			!config.WebhookAuthBasicAuthPasswordWO.IsUnknown() && config.WebhookAuthBasicAuthPasswordWO.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("webhook_auth_basic_auth_password"),
				"Missing Required Attribute",
				"webhook_auth_basic_auth_password (or webhook_auth_basic_auth_password_wo) is required when webhook_auth_type is \"basic_auth\".",
			)
		}
	case webhookAuthAPIKey:
		if !config.WebhookAuthAPIKeyName.IsUnknown() && config.WebhookAuthAPIKeyName.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("webhook_auth_api_key_name"),
				"Missing Required Attribute",
				"webhook_auth_api_key_name is required when webhook_auth_type is \"api_key\".",
			)
		}
		// The API key value may be supplied via webhook_auth_api_key_value or its
		// write-only counterpart. Only error when both are known and null.
		if !config.WebhookAuthAPIKeyValue.IsUnknown() && config.WebhookAuthAPIKeyValue.IsNull() &&
			!config.WebhookAuthAPIKeyValueWO.IsUnknown() && config.WebhookAuthAPIKeyValueWO.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("webhook_auth_api_key_value"),
				"Missing Required Attribute",
				"webhook_auth_api_key_value (or webhook_auth_api_key_value_wo) is required when webhook_auth_type is \"api_key\".",
			)
		}
		if !config.WebhookAuthAPIKeyIn.IsUnknown() && config.WebhookAuthAPIKeyIn.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root("webhook_auth_api_key_in"),
				"Missing Required Attribute",
				"webhook_auth_api_key_in is required when webhook_auth_type is \"api_key\".",
			)
		}
	}
}

// buildHookValue builds the Ory web_hook config map. basicAuthPassword and
// apiKeyValue carry the resolved secret values (the write-only _wo value when
// set, otherwise the stateful attribute), read from req.Config by
// resolveAuthSecrets. They are used only to build the API payload and are never
// written back into the model, so they never reach Terraform state.
func (r *ActionResource) buildHookValue(plan *ActionResourceModel, basicAuthPassword, apiKeyValue types.String) map[string]interface{} {
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

	// Build auth config if webhook_auth_type is set and all required fields are present
	if !plan.WebhookAuthType.IsNull() && !plan.WebhookAuthType.IsUnknown() {
		authType := plan.WebhookAuthType.ValueString()
		authCfg := map[string]interface{}{}
		hasValidAuthConfig := false

		switch authType {
		case webhookAuthBasicAuth:
			if !plan.WebhookAuthBasicAuthUser.IsNull() && !plan.WebhookAuthBasicAuthUser.IsUnknown() &&
				!basicAuthPassword.IsNull() && !basicAuthPassword.IsUnknown() {
				authCfg["user"] = plan.WebhookAuthBasicAuthUser.ValueString()
				authCfg["password"] = basicAuthPassword.ValueString()
				hasValidAuthConfig = true
			}
		case webhookAuthAPIKey:
			if !plan.WebhookAuthAPIKeyName.IsNull() && !plan.WebhookAuthAPIKeyName.IsUnknown() &&
				!apiKeyValue.IsNull() && !apiKeyValue.IsUnknown() &&
				!plan.WebhookAuthAPIKeyIn.IsNull() && !plan.WebhookAuthAPIKeyIn.IsUnknown() {
				authCfg["name"] = plan.WebhookAuthAPIKeyName.ValueString()
				authCfg["value"] = apiKeyValue.ValueString()
				authCfg["in"] = plan.WebhookAuthAPIKeyIn.ValueString()
				hasValidAuthConfig = true
			}
		}

		if hasValidAuthConfig {
			hookConfig["auth"] = map[string]interface{}{
				"type":   authType,
				"config": authCfg,
			}
		}
	}

	return map[string]interface{}{
		"hook":   "web_hook",
		"config": hookConfig,
	}
}

// resolveAuthSecrets returns the effective basic-auth password and api-key value
// for an apply, preferring the write-only (_wo) value when the user supplied one.
// Write-only attribute values are nullified in the plan and state, so they must
// be read from the configuration (req.Config) at Create and Update time.
func (r *ActionResource) resolveAuthSecrets(ctx context.Context, config tfsdk.Config, plan *ActionResourceModel, diags *diag.Diagnostics) (basicAuthPassword, apiKeyValue types.String) {
	basicAuthPassword = plan.WebhookAuthBasicAuthPassword
	apiKeyValue = plan.WebhookAuthAPIKeyValue

	var passwordWO, valueWO types.String
	diags.Append(config.GetAttribute(ctx, path.Root("webhook_auth_basic_auth_password_wo"), &passwordWO)...)
	diags.Append(config.GetAttribute(ctx, path.Root("webhook_auth_api_key_value_wo"), &valueWO)...)

	if !passwordWO.IsNull() && !passwordWO.IsUnknown() {
		basicAuthPassword = passwordWO
	}
	if !valueWO.IsNull() && !valueWO.IsUnknown() {
		apiKeyValue = valueWO
	}
	return basicAuthPassword, apiKeyValue
}

// copyHooks returns a deep copy of the hooks slice via a JSON round-trip so
// callers cannot accidentally mutate the cached project state shared with
// other resources in the same Terraform run.
func copyHooks(hooks []map[string]interface{}) []map[string]interface{} {
	b, err := json.Marshal(hooks)
	if err != nil {
		// Should never happen — the data was decoded from JSON.
		return hooks
	}
	var cp []map[string]interface{}
	if err := json.Unmarshal(b, &cp); err != nil {
		return hooks
	}
	return cp
}

func (r *ActionResource) getHooks(ctx context.Context, projectID, flow, timing, authMethod string) ([]map[string]interface{}, error) {
	// Prefer cached project state from a previous PatchProject call in this
	// Terraform run. This avoids reading stale data from the eventually-consistent
	// GetProject API, which is critical when the mutex serializes back-to-back
	// mutations — the second operation must see the first operation's result.
	if cached := r.client.GetCachedProject(projectID); cached != nil {
		return copyHooks(r.getHooksFromProject(cached, flow, timing, authMethod)), nil
	}
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		// A purged project has no hooks; treat it as empty so Delete/Read can
		// converge instead of erroring during teardown.
		if client.IsNotFound(err) {
			return []map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}
	return r.getHooksFromProject(project, flow, timing, authMethod), nil
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

// authMethodsForFlow returns the authentication methods a flow's "after" hooks
// can be scoped by, in the order the Ory Console UI lists them. A flow with no
// method-scoped after-hooks (recovery, verification) returns nil.
//
// The sets are the method keys under
// services.identity.config.selfservice.flows.<flow>.after in a live project
// config, which is also how the Console UI builds its "Method" picker. Verified
// against the Console API on 2026-08-13: "profile" exists only for settings,
// "code" only for login and registration, and "totp"/"lookup_secret" only for
// login and settings.
//
// Writing a hook to a method a flow does not support returns HTTP 200 but the
// API drops the key, so nothing is stored. See
// https://github.com/ory/terraform-provider-ory/issues/328
func authMethodsForFlow(flow string) []string {
	switch flow {
	case "login":
		return []string{"password", "oidc", "code", "webauthn", "passkey", "totp", "lookup_secret", "saml"}
	case "registration":
		return []string{"password", "oidc", "code", "webauthn", "passkey", "saml"}
	case "settings":
		return []string{"password", "oidc", "profile", "webauthn", "passkey", "totp", "lookup_secret", "saml"}
	default:
		return nil
	}
}

// flowSupportsAuthMethod reports whether a flow's "after" hooks are scoped by
// authentication method (e.g. .../after/password/hooks). Only the login,
// registration, and settings flows have method-scoped after-hooks in the Ory
// Kratos config. The recovery and verification flows store their after-hooks in
// a single flat array at .../after/hooks with no auth-method level, so PATCHing
// to .../after/<auth_method>/hooks for them returns 200 but silently drops the
// hook. See https://github.com/ory/terraform-provider-ory/issues/241
func flowSupportsAuthMethod(flow string) bool {
	return authMethodsForFlow(flow) != nil
}

// flowSupportsMethod reports whether the given flow's after-hooks accept the
// given authentication method.
func flowSupportsMethod(flow, authMethod string) bool {
	return slices.Contains(authMethodsForFlow(flow), authMethod)
}

// unsupportedMethodDetail explains that a flow has no key for an authentication
// method and lists the methods it does accept.
func unsupportedMethodDetail(flow, authMethod string) string {
	return fmt.Sprintf("The %q flow does not support auth_method %q. Supported methods for this flow: %s. "+
		"Ory accepts a hook written to an unsupported method with HTTP 200 but discards it, so the hook is "+
		"never stored.", flow, authMethod, strings.Join(authMethodsForFlow(flow), ", "))
}

// verifyFailureDetail describes a hook that is absent from the PatchProject
// response. An unsupported flow and method pair is the most likely cause, so
// name it instead of leaving the user with a bare "not found".
func verifyFailureDetail(flow, timing, authMethod, url string) string {
	detail := fmt.Sprintf("Hook not found in PatchProject response for %s/%s/%s with URL %s", flow, timing, authMethod, url)
	if timing == timingAfter && flowSupportsAuthMethod(flow) && !flowSupportsMethod(flow, authMethod) {
		detail += "\n\n" + unsupportedMethodDetail(flow, authMethod)
	}
	return detail
}

func (r *ActionResource) hookPath(flow, timing, authMethod string) string {
	if timing == timingAfter && flowSupportsAuthMethod(flow) {
		return fmt.Sprintf("/services/identity/config/selfservice/flows/%s/%s/%s/hooks", flow, timing, authMethod)
	}
	return fmt.Sprintf("/services/identity/config/selfservice/flows/%s/%s/hooks", flow, timing)
}

func (r *ActionResource) getHooksFromProject(project *ory.Project, flow, timing, authMethod string) []map[string]interface{} {
	// A deleted project still returns 200 from GetProject (soft delete) but its
	// config carries no hooks; report none so Delete/Read converge cleanly.
	if project.GetState() == projectStateDeleted {
		return []map[string]interface{}{}
	}

	if project.Services.Identity == nil {
		return []map[string]interface{}{}
	}

	configMap := project.Services.Identity.Config
	if configMap == nil {
		return []map[string]interface{}{}
	}

	selfservice, ok := configMap["selfservice"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	flows, ok := selfservice["flows"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	flowConfig, ok := flows[flow].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	timingConfig, ok := flowConfig[timing].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var hooks []interface{}
	if timing == timingAfter && flowSupportsAuthMethod(flow) {
		authMethodConfig, ok := timingConfig[authMethod].(map[string]interface{})
		if !ok {
			return []map[string]interface{}{}
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
	return result
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

	// Serialize mutations to prevent concurrent read-modify-write races.
	mu := projectMutex(projectID)
	mu.Lock()
	defer mu.Unlock()

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

	basicAuthPassword, apiKeyValue := r.resolveAuthSecrets(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	hookValue := r.buildHookValue(&plan, basicAuthPassword, apiKeyValue)
	hookPath := r.hookPath(flow, timing, authMethod)

	// Append the new hook to existing hooks and replace the entire array
	// This handles the case where the hooks array might not exist
	newHooks := make([]interface{}, 0, len(hooks))
	for _, h := range hooks {
		newHooks = append(newHooks, h)
	}
	newHooks = append(newHooks, hookValue)

	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  hookPath,
		Value: newHooks,
	}}

	result, err := r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Action", err.Error())
		return
	}

	project := result.GetProject()
	hooks = r.getHooksFromProject(&project, flow, timing, authMethod)
	if r.findHookIndex(hooks, url, httpMethod) < 0 {
		resp.Diagnostics.AddError("Error Verifying Action",
			verifyFailureDetail(flow, timing, authMethod, url))
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

	var hooks []map[string]interface{}
	var index = -1

	if cached := r.client.GetCachedProject(projectID); cached != nil {
		hooks = r.getHooksFromProject(cached, flow, timing, authMethod)
		index = r.findHookIndex(hooks, url, httpMethod)
	}

	if index < 0 {
		var err error
		for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
			hooks, err = r.getHooks(ctx, projectID, flow, timing, authMethod)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading Action", err.Error())
				return
			}

			index = r.findHookIndex(hooks, url, httpMethod)
			if index >= 0 {
				break
			}

			if attempt < helpers.ReadRetryMaxAttempts-1 {
				select {
				case <-ctx.Done():
					resp.State.RemoveResource(ctx)
					return
				case <-time.After(time.Duration(1<<attempt) * time.Second):
				}
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

	// Read auth config - explicitly null all fields first, then set from remote state
	if auth, ok := config["auth"].(map[string]interface{}); ok {
		if authType, ok := auth["type"].(string); ok && authType != "" {
			state.WebhookAuthType = types.StringValue(authType)

			authCfg, _ := auth["config"].(map[string]interface{})
			if authCfg == nil {
				authCfg = map[string]interface{}{}
			}

			switch authType {
			case webhookAuthBasicAuth:
				if user, ok := authCfg["user"].(string); ok && user != "" {
					state.WebhookAuthBasicAuthUser = types.StringValue(user)
				} else {
					state.WebhookAuthBasicAuthUser = types.StringNull()
				}
				// Password is sensitive - the API may not return it.
				// Preserve the existing state value to avoid drift when omitted.
				// Only refresh when already tracked in state: when supplied via the
				// write-only webhook_auth_basic_auth_password_wo it is null in state
				// and must stay null so the secret never lands in state.
				if !state.WebhookAuthBasicAuthPassword.IsNull() {
					if password, ok := authCfg["password"].(string); ok && password != "" {
						state.WebhookAuthBasicAuthPassword = types.StringValue(password)
					}
				}
				// Clear unrelated auth-type fields
				state.WebhookAuthAPIKeyName = types.StringNull()
				state.WebhookAuthAPIKeyValue = types.StringNull()
				state.WebhookAuthAPIKeyIn = types.StringNull()
			case webhookAuthAPIKey:
				if name, ok := authCfg["name"].(string); ok && name != "" {
					state.WebhookAuthAPIKeyName = types.StringValue(name)
				} else {
					state.WebhookAuthAPIKeyName = types.StringNull()
				}
				// Value is sensitive - the API may not return it.
				// Preserve the existing state value to avoid drift when omitted.
				// Only refresh when already tracked in state: when supplied via the
				// write-only webhook_auth_api_key_value_wo it is null in state and
				// must stay null so the secret never lands in state.
				if !state.WebhookAuthAPIKeyValue.IsNull() {
					if value, ok := authCfg["value"].(string); ok && value != "" {
						state.WebhookAuthAPIKeyValue = types.StringValue(value)
					}
				}
				if in, ok := authCfg["in"].(string); ok && in != "" {
					state.WebhookAuthAPIKeyIn = types.StringValue(in)
				} else {
					state.WebhookAuthAPIKeyIn = types.StringNull()
				}
				// Clear unrelated auth-type fields
				state.WebhookAuthBasicAuthUser = types.StringNull()
				state.WebhookAuthBasicAuthPassword = types.StringNull()
			default:
				state.WebhookAuthType = types.StringNull()
				state.WebhookAuthBasicAuthUser = types.StringNull()
				state.WebhookAuthBasicAuthPassword = types.StringNull()
				state.WebhookAuthAPIKeyName = types.StringNull()
				state.WebhookAuthAPIKeyValue = types.StringNull()
				state.WebhookAuthAPIKeyIn = types.StringNull()
			}
		} else {
			state.WebhookAuthType = types.StringNull()
			state.WebhookAuthBasicAuthUser = types.StringNull()
			state.WebhookAuthBasicAuthPassword = types.StringNull()
			state.WebhookAuthAPIKeyName = types.StringNull()
			state.WebhookAuthAPIKeyValue = types.StringNull()
			state.WebhookAuthAPIKeyIn = types.StringNull()
		}
	} else {
		// No auth block - clear all auth fields so removed auth is detected as drift
		state.WebhookAuthType = types.StringNull()
		state.WebhookAuthBasicAuthUser = types.StringNull()
		state.WebhookAuthBasicAuthPassword = types.StringNull()
		state.WebhookAuthAPIKeyName = types.StringNull()
		state.WebhookAuthAPIKeyValue = types.StringNull()
		state.WebhookAuthAPIKeyIn = types.StringNull()
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

	// Serialize mutations to prevent concurrent read-modify-write races.
	mu := projectMutex(projectID)
	mu.Lock()
	defer mu.Unlock()

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

	basicAuthPassword, apiKeyValue := r.resolveAuthSecrets(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	hookValue := r.buildHookValue(&plan, basicAuthPassword, apiKeyValue)
	hookPath := r.hookPath(flow, timing, authMethod)

	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  fmt.Sprintf("%s/%d", hookPath, index),
		Value: hookValue,
	}}

	result, err := r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Action", err.Error())
		return
	}

	project := result.GetProject()
	newURL := plan.URL.ValueString()
	newMethod := plan.HTTPMethod.ValueString()
	updatedHooks := r.getHooksFromProject(&project, flow, timing, authMethod)
	if r.findHookIndex(updatedHooks, newURL, newMethod) < 0 {
		resp.Diagnostics.AddError("Error Verifying Action Update",
			verifyFailureDetail(flow, timing, authMethod, newURL))
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

	// Serialize mutations to prevent concurrent read-modify-write races.
	mu := projectMutex(projectID)
	mu.Lock()
	defer mu.Unlock()

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

// isImportHTTPMethod reports whether s is one of the HTTP methods accepted in
// the optional method segment of an import ID.
func isImportHTTPMethod(s string) bool {
	switch s {
	case "POST", "GET", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// actionImportID holds the fields parsed from an ory_action import ID.
type actionImportID struct {
	projectID  string
	flow       string
	timing     string
	authMethod string
	httpMethod string
	url        string
}

// parseActionImportID parses an ory_action import ID. On success it returns the
// parsed fields and an empty detail; on failure it returns a non-empty
// diagnostic detail describing the problem (suitable as the detail of an
// "Invalid Import ID" diagnostic).
//
// Import ID layouts (colon-separated, the url is always the tail):
//   - For "after" timing:  project_id:flow:after:auth_method:method:url
//   - For "before" timing: project_id:flow:before:method:url
//
// The method segment is optional and defaults to POST (legacy formats).
//
// The url itself contains colons (https://...), so the ID cannot be parsed by
// counting segments (issue #280). The fixed segments are parsed from the left,
// and the optional method segment is recognized by matching it against the
// known HTTP verbs.
func parseActionImportID(id string) (actionImportID, string) {
	const importFormatHelp = "Import ID must be in one of these formats:\n" +
		"  - For 'after' timing: project_id:flow:after:auth_method:method:url\n" +
		"  - For 'before' timing: project_id:flow:before:method:url\n" +
		"The method segment is optional and defaults to POST.\n\n" +
		"Examples:\n" +
		"  550e8400-...:registration:after:password:POST:https://api.example.com/webhook\n" +
		"  550e8400-...:login:before:PATCH:https://api.example.com/validate"

	parts := strings.SplitN(id, ":", 4)
	if len(parts) != 4 {
		return actionImportID{}, importFormatHelp
	}
	parsed := actionImportID{
		projectID:  parts[0],
		flow:       parts[1],
		timing:     parts[2],
		authMethod: defaultAuthMethod,
		httpMethod: defaultHTTPMethod,
	}
	rest := parts[3]

	if parsed.timing != timingBefore && parsed.timing != timingAfter {
		return actionImportID{}, fmt.Sprintf("Invalid timing '%s'. Must be 'before' or 'after'.\n\n%s", parsed.timing, importFormatHelp)
	}

	if parsed.timing == timingAfter {
		// The auth_method segment is required for "after" timing. "_", "none",
		// and "" are accepted as placeholders for flows that ignore it.
		segment, tail, found := strings.Cut(rest, ":")
		if !found {
			return actionImportID{}, "Missing url after auth_method for 'after' timing.\n\n" + importFormatHelp
		}
		if segment != "_" && segment != "none" && segment != "" {
			parsed.authMethod = segment
		}
		rest = tail
	}

	// Optional method segment. A leading segment before the first ':' is only a
	// method if it matches a known HTTP verb; otherwise the ':' belongs to the
	// url's own scheme separator (https://...) and the method was omitted.
	// Detect the scheme case by the "//" authority prefix rather than scanning
	// the whole tail for "://", which would false-positive on urls with an
	// embedded scheme in a query param (e.g. ?redirect=https://other).
	if segment, tail, found := strings.Cut(rest, ":"); found {
		if isImportHTTPMethod(strings.ToUpper(segment)) {
			parsed.httpMethod = strings.ToUpper(segment)
			rest = tail
		} else if !strings.HasPrefix(tail, "//") {
			// A url still follows, so this segment was meant to be the method.
			return actionImportID{}, fmt.Sprintf("Invalid HTTP method '%s'. Must be one of: POST, GET, PUT, PATCH, DELETE", segment)
		}
	}

	parsed.url = rest
	if parsed.url == "" {
		return actionImportID{}, "Missing url.\n\n" + importFormatHelp
	}
	return parsed, ""
}

func (r *ActionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, detail := parseActionImportID(req.ID)
	if detail != "" {
		resp.Diagnostics.AddError("Invalid Import ID", detail)
		return
	}

	// Construct the full ID for state
	fullID := fmt.Sprintf("%s:%s:%s:%s:%s", parsed.projectID, parsed.flow, parsed.timing, parsed.authMethod, parsed.url)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fullID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parsed.projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("flow"), parsed.flow)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("timing"), parsed.timing)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("auth_method"), parsed.authMethod)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("url"), parsed.url)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("method"), parsed.httpMethod)...)
}
