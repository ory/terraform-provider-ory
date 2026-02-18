package projectconfig

import (
	"context"
	"fmt"
	"regexp"

	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ resource.Resource                = &ProjectConfigResource{}
	_ resource.ResourceWithConfigure   = &ProjectConfigResource{}
	_ resource.ResourceWithImportState = &ProjectConfigResource{}
)

func NewResource() resource.Resource {
	return &ProjectConfigResource{}
}

type ProjectConfigResource struct {
	client *client.OryClient
}

type ProjectConfigResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`

	// Keto/Permissions Namespaces
	KetoNamespaces types.List `tfsdk:"keto_namespaces"`

	// CORS (Public)
	CorsEnabled types.Bool `tfsdk:"cors_enabled"`
	CorsOrigins types.List `tfsdk:"cors_origins"`

	// CORS (Admin)
	CorsAdminEnabled types.Bool `tfsdk:"cors_admin_enabled"`
	CorsAdminOrigins types.List `tfsdk:"cors_admin_origins"`

	// Session
	SessionLifespan         types.String `tfsdk:"session_lifespan"`
	SessionCookieSameSite   types.String `tfsdk:"session_cookie_same_site"`
	SessionCookiePersistent types.Bool   `tfsdk:"session_cookie_persistent"`

	// OAuth2/Hydra
	OAuth2AccessTokenLifespan  types.String `tfsdk:"oauth2_access_token_lifespan"`
	OAuth2RefreshTokenLifespan types.String `tfsdk:"oauth2_refresh_token_lifespan"`

	// URLs
	DefaultReturnURL  types.String `tfsdk:"default_return_url"`
	AllowedReturnURLs types.List   `tfsdk:"allowed_return_urls"`
	LoginUIURL        types.String `tfsdk:"login_ui_url"`
	RegistrationUIURL types.String `tfsdk:"registration_ui_url"`
	RecoveryUIURL     types.String `tfsdk:"recovery_ui_url"`
	VerificationUIURL types.String `tfsdk:"verification_ui_url"`
	SettingsUIURL     types.String `tfsdk:"settings_ui_url"`
	ErrorUIURL        types.String `tfsdk:"error_ui_url"`

	// Auth methods
	EnablePassword     types.Bool `tfsdk:"enable_password"`
	EnableCode         types.Bool `tfsdk:"enable_code"`
	EnableTOTP         types.Bool `tfsdk:"enable_totp"`
	EnableWebAuthn     types.Bool `tfsdk:"enable_webauthn"`
	EnablePasskey      types.Bool `tfsdk:"enable_passkey"`
	EnableLookupSecret types.Bool `tfsdk:"enable_lookup_secret"`

	// Password policy
	PasswordMinLength            types.Int64 `tfsdk:"password_min_length"`
	PasswordCheckHaveIBeenPwned  types.Bool  `tfsdk:"password_check_haveibeenpwned"`
	PasswordMaxBreaches          types.Int64 `tfsdk:"password_max_breaches"`
	PasswordIdentifierSimilarity types.Bool  `tfsdk:"password_identifier_similarity"`

	// Flow settings
	EnableRecovery     types.Bool `tfsdk:"enable_recovery"`
	EnableVerification types.Bool `tfsdk:"enable_verification"`
	EnableRegistration types.Bool `tfsdk:"enable_registration"`

	// SMTP Configuration
	SMTPConnectionURI types.String `tfsdk:"smtp_connection_uri"`
	SMTPFromAddress   types.String `tfsdk:"smtp_from_address"`
	SMTPFromName      types.String `tfsdk:"smtp_from_name"`
	SMTPHeaders       types.Map    `tfsdk:"smtp_headers"`

	// MFA Policy
	MFAEnforcement           types.String `tfsdk:"mfa_enforcement"`
	TOTPIssuer               types.String `tfsdk:"totp_issuer"`
	WebAuthnRPDisplayName    types.String `tfsdk:"webauthn_rp_display_name"`
	WebAuthnRPID             types.String `tfsdk:"webauthn_rp_id"`
	WebAuthnRPOrigins        types.List   `tfsdk:"webauthn_rp_origins"`
	WebAuthnPasswordless     types.Bool   `tfsdk:"webauthn_passwordless"`
	RequiredAAL              types.String `tfsdk:"required_aal"`
	SessionWhoamiRequiredAAL types.String `tfsdk:"session_whoami_required_aal"`

	// Account Experience (Branding)
	AccountExperienceFaviconURL types.String `tfsdk:"account_experience_favicon_url"`
	AccountExperienceLogoURL    types.String `tfsdk:"account_experience_logo_url"`
	AccountExperienceName       types.String `tfsdk:"account_experience_name"`
	AccountExperienceStylesheet types.String `tfsdk:"account_experience_stylesheet"`
	AccountExperienceLocale     types.String `tfsdk:"account_experience_default_locale"`

	// Session Tokenizer Templates
	SessionTokenizerTemplates types.Map `tfsdk:"session_tokenizer_templates"`

	// Courier HTTP Delivery
	CourierDeliveryStrategy  types.String `tfsdk:"courier_delivery_strategy"`
	CourierHTTPRequestConfig types.Object `tfsdk:"courier_http_request_config"`
	CourierChannels          types.List   `tfsdk:"courier_channels"`
}

// --- Nested model types for session tokenizer templates and courier HTTP ---

// SessionTokenizerTemplateModel represents a single tokenizer template entry.
type SessionTokenizerTemplateModel struct {
	TTL             types.String `tfsdk:"ttl"`
	JWKSURL         types.String `tfsdk:"jwks_url"`
	ClaimsMapperURL types.String `tfsdk:"claims_mapper_url"`
	SubjectSource   types.String `tfsdk:"subject_source"`
}

// CourierHTTPAuthModel represents the auth block for HTTP request configs.
// This is a flattened discriminated union: set type + the fields for that type.
type CourierHTTPAuthModel struct {
	Type     types.String `tfsdk:"type"`
	User     types.String `tfsdk:"user"`
	Password types.String `tfsdk:"password"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	In       types.String `tfsdk:"in"`
}

// CourierHTTPRequestConfigModel represents an HTTP request configuration.
type CourierHTTPRequestConfigModel struct {
	URL     types.String `tfsdk:"url"`
	Method  types.String `tfsdk:"method"`
	Headers types.Map    `tfsdk:"headers"`
	Body    types.String `tfsdk:"body"`
	Auth    types.Object `tfsdk:"auth"`
}

// CourierChannelModel represents a single courier channel entry.
type CourierChannelModel struct {
	ID            types.String `tfsdk:"id"`
	RequestConfig types.Object `tfsdk:"request_config"`
}

// Shared attr.Type maps for constructing types.Object / types.Map / types.List values.
var (
	tokenizerTemplateAttrTypes = map[string]attr.Type{
		"ttl":               types.StringType,
		"jwks_url":          types.StringType,
		"claims_mapper_url": types.StringType,
		"subject_source":    types.StringType,
	}

	courierHTTPAuthAttrTypes = map[string]attr.Type{
		"type":     types.StringType,
		"user":     types.StringType,
		"password": types.StringType,
		"name":     types.StringType,
		"value":    types.StringType,
		"in":       types.StringType,
	}

	courierHTTPRequestConfigAttrTypes = map[string]attr.Type{
		"url":     types.StringType,
		"method":  types.StringType,
		"headers": types.MapType{ElemType: types.StringType},
		"body":    types.StringType,
		"auth":    types.ObjectType{AttrTypes: courierHTTPAuthAttrTypes},
	}

	courierChannelAttrTypes = map[string]attr.Type{
		"id":             types.StringType,
		"request_config": types.ObjectType{AttrTypes: courierHTTPRequestConfigAttrTypes},
	}
)

func (r *ProjectConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_config"
}

const projectConfigMarkdownDescription = `
Configures an Ory Network project's settings.

This resource manages the configuration of an Ory Network project, including authentication methods,
password policies, session settings, CORS, and more.

## Example Usage

` + "```hcl" + `
resource "ory_project_config" "main" {
  cors_enabled        = true
  cors_origins        = ["https://app.example.com"]
  password_min_length = 10
  session_lifespan    = "720h"  # 30 days
}
` + "```" + `

## Import

Import using the project ID:

` + "```shell" + `
terraform import ory_project_config.main <project-id>
` + "```" + `

### Avoiding "Forces Replacement" After Import

After importing, if Terraform shows ` + "`project_id forces replacement`" + `, ensure your configuration matches:

**Option 1: Explicit project_id**
` + "```hcl" + `
resource "ory_project_config" "main" {
  project_id = "the-exact-project-id-you-imported"
  # ... other settings
}
` + "```" + `

**Option 2: Use provider default** (recommended)
` + "```hcl" + `
provider "ory" {
  project_id = "the-exact-project-id-you-imported"
}

resource "ory_project_config" "main" {
  # project_id inherits from provider
  # ... other settings
}
` + "```" + `

## Notes

- Project config cannot be deleted - it always exists for a project
- Deleting this resource from Terraform state does not reset the project configuration
- The ` + "`project_id`" + ` attribute forces replacement if changed (you cannot move config to a different project)
`

func (r *ProjectConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Configures an Ory Network project's settings.",
		MarkdownDescription: projectConfigMarkdownDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID (same as project_id).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID to configure. If not set, uses provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Keto/Permissions Namespaces
			"keto_namespaces": schema.ListAttribute{
				Description: "List of Keto namespace names to configure for Ory Permissions. " +
					"Namespaces define the types of resources in your permission model (e.g., 'documents', 'folders'). " +
					"Each namespace name must be unique.",
				Optional:    true,
				ElementType: types.StringType,
			},

			// CORS
			"cors_enabled": schema.BoolAttribute{
				Description: "Enable CORS for the public API.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"cors_origins": schema.ListAttribute{
				Description: "Allowed CORS origins.",
				Optional:    true,
				ElementType: types.StringType,
			},

			// CORS (Admin)
			"cors_admin_enabled": schema.BoolAttribute{
				Description: "Enable CORS for the admin API.",
				Optional:    true,
			},
			"cors_admin_origins": schema.ListAttribute{
				Description: "Allowed CORS origins for the admin API.",
				Optional:    true,
				ElementType: types.StringType,
			},

			// Session
			"session_lifespan": schema.StringAttribute{
				Description: "Session duration (e.g., '24h0m0s').",
				Optional:    true,
			},
			"session_cookie_same_site": schema.StringAttribute{
				Description: "SameSite cookie attribute (Lax, Strict, None).",
				Optional:    true,
			},
			"session_cookie_persistent": schema.BoolAttribute{
				Description: "Enable persistent session cookies (survive browser close).",
				Optional:    true,
			},

			// OAuth2/Hydra
			"oauth2_access_token_lifespan": schema.StringAttribute{
				Description: "OAuth2 access token lifespan (e.g., '1h', '30m'). Requires Hydra service.",
				Optional:    true,
			},
			"oauth2_refresh_token_lifespan": schema.StringAttribute{
				Description: "OAuth2 refresh token lifespan (e.g., '720h' for 30 days). Requires Hydra service.",
				Optional:    true,
			},

			// URLs
			"default_return_url": schema.StringAttribute{
				Description: "Default URL to redirect after flows.",
				Optional:    true,
			},
			"allowed_return_urls": schema.ListAttribute{
				Description: "List of allowed return URLs.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"login_ui_url": schema.StringAttribute{
				Description: "URL for the login UI.",
				Optional:    true,
			},
			"registration_ui_url": schema.StringAttribute{
				Description: "URL for the registration UI.",
				Optional:    true,
			},
			"recovery_ui_url": schema.StringAttribute{
				Description: "URL for the password recovery UI.",
				Optional:    true,
			},
			"verification_ui_url": schema.StringAttribute{
				Description: "URL for the verification UI.",
				Optional:    true,
			},
			"settings_ui_url": schema.StringAttribute{
				Description: "URL for the account settings UI.",
				Optional:    true,
			},
			"error_ui_url": schema.StringAttribute{
				Description: "URL for the error UI.",
				Optional:    true,
			},

			// Auth methods
			"enable_password": schema.BoolAttribute{
				Description: "Enable password authentication.",
				Optional:    true,
			},
			"enable_code": schema.BoolAttribute{
				Description: "Enable code-based authentication.",
				Optional:    true,
			},
			"enable_totp": schema.BoolAttribute{
				Description: "Enable TOTP (Time-based One-Time Password).",
				Optional:    true,
			},
			"enable_webauthn": schema.BoolAttribute{
				Description: "Enable WebAuthn (hardware keys).",
				Optional:    true,
			},
			"enable_passkey": schema.BoolAttribute{
				Description: "Enable Passkey authentication.",
				Optional:    true,
			},
			"enable_lookup_secret": schema.BoolAttribute{
				Description: "Enable backup/recovery codes.",
				Optional:    true,
			},

			// Password policy
			"password_min_length": schema.Int64Attribute{
				Description: "Minimum password length.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(8),
			},
			"password_check_haveibeenpwned": schema.BoolAttribute{
				Description: "Check passwords against HaveIBeenPwned.",
				Optional:    true,
			},
			"password_max_breaches": schema.Int64Attribute{
				Description: "Maximum allowed breaches in HaveIBeenPwned.",
				Optional:    true,
			},
			"password_identifier_similarity": schema.BoolAttribute{
				Description: "Check password similarity to identifier.",
				Optional:    true,
			},

			// Flow settings
			"enable_recovery": schema.BoolAttribute{
				Description: "Enable password recovery flow.",
				Optional:    true,
			},
			"enable_verification": schema.BoolAttribute{
				Description: "Enable email verification flow.",
				Optional:    true,
			},
			"enable_registration": schema.BoolAttribute{
				Description: "Enable user registration.",
				Optional:    true,
			},

			// SMTP Configuration
			"smtp_connection_uri": schema.StringAttribute{
				Description: "SMTP connection URI for sending emails.",
				Optional:    true,
				Sensitive:   true,
			},
			"smtp_from_address": schema.StringAttribute{
				Description: "Email address to send from.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`),
						"must be a valid email address",
					),
				},
			},
			"smtp_from_name": schema.StringAttribute{
				Description: "Name to display as sender.",
				Optional:    true,
			},
			"smtp_headers": schema.MapAttribute{
				Description: "Custom headers to include in emails.",
				Optional:    true,
				ElementType: types.StringType,
			},

			// MFA Policy
			"mfa_enforcement": schema.StringAttribute{
				Description: "MFA enforcement level: 'none', 'optional', or 'required'.",
				Optional:    true,
			},
			"totp_issuer": schema.StringAttribute{
				Description: "TOTP issuer name shown in authenticator apps.",
				Optional:    true,
			},
			"webauthn_rp_display_name": schema.StringAttribute{
				Description: "WebAuthn Relying Party display name.",
				Optional:    true,
			},
			"webauthn_rp_id": schema.StringAttribute{
				Description: "WebAuthn Relying Party ID (typically your domain).",
				Optional:    true,
			},
			"webauthn_rp_origins": schema.ListAttribute{
				Description: "Allowed WebAuthn origins.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"webauthn_passwordless": schema.BoolAttribute{
				Description: "Enable passwordless WebAuthn authentication.",
				Optional:    true,
			},
			"required_aal": schema.StringAttribute{
				Description: "Required Authenticator Assurance Level for protected resources: 'aal1' or 'aal2'.",
				Optional:    true,
			},
			"session_whoami_required_aal": schema.StringAttribute{
				Description: "Required AAL for session whoami endpoint: 'aal1', 'aal2', or 'highest_available'.",
				Optional:    true,
			},

			// Account Experience (Branding)
			"account_experience_favicon_url": schema.StringAttribute{
				Description: "URL for the favicon in the hosted login UI.",
				Optional:    true,
			},
			"account_experience_logo_url": schema.StringAttribute{
				Description: "URL for the logo in the hosted login UI.",
				Optional:    true,
			},
			"account_experience_name": schema.StringAttribute{
				Description: "Application name shown in the hosted login UI.",
				Optional:    true,
			},
			"account_experience_stylesheet": schema.StringAttribute{
				Description: "Custom CSS stylesheet for the hosted login UI.",
				Optional:    true,
			},
			"account_experience_default_locale": schema.StringAttribute{
				Description: "Default locale for the hosted login UI (e.g., 'en', 'de').",
				Optional:    true,
			},

			// Session Tokenizer Templates
			"session_tokenizer_templates": schema.MapNestedAttribute{
				Description: "JWT tokenizer templates for the /sessions/whoami endpoint. " +
					"Each key is a template name, and the value configures how JWTs are generated.",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ttl": schema.StringAttribute{
							Description: "Token time-to-live duration (e.g., '1h', '30m'). Default: '1m'.",
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[0-9]+(ns|us|ms|s|m|h)$`),
									"must be a valid Go duration (e.g., '1h', '30m', '10s')",
								),
							},
						},
						"jwks_url": schema.StringAttribute{
							Description: "JWKS URL for signing tokens. Must use base64:// scheme (e.g., 'base64://eyJrZXlzIjpbXX0=').",
							Required:    true,
							Sensitive:   true,
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^base64://`),
									"must start with 'base64://'",
								),
							},
						},
						"claims_mapper_url": schema.StringAttribute{
							Description: "Jsonnet claims mapper URL. Supports base64:// and https:// schemes.",
							Optional:    true,
						},
						"subject_source": schema.StringAttribute{
							Description: "Subject source for the JWT: 'id' (default) or 'external_id'.",
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.OneOf("id", "external_id"),
							},
						},
					},
				},
			},

			// Courier HTTP Delivery
			"courier_delivery_strategy": schema.StringAttribute{
				Description: "Courier delivery strategy: 'smtp' (default) or 'http'.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("smtp", "http"),
				},
			},
			"courier_http_request_config": courierHTTPRequestConfigSchemaAttr(
				"HTTP request configuration for courier message delivery (used when courier_delivery_strategy is 'http').",
			),
			"courier_channels": schema.ListNestedAttribute{
				Description: "Per-channel courier delivery configurations (e.g., SMS via Twilio). " +
					"Each channel overrides the default delivery for a specific message channel.",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Channel identifier (e.g., 'sms').",
							Required:    true,
						},
						"request_config": courierHTTPRequestConfigSchemaAttr(
							"HTTP request configuration for this channel.",
						),
					},
				},
			},
		},
	}
}

// courierHTTPAuthSchemaAttrs returns the schema attributes for the courier HTTP auth block.
func courierHTTPAuthSchemaAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Description: "Authentication type: 'basic_auth' or 'api_key'.",
			Required:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("basic_auth", "api_key"),
			},
		},
		"user": schema.StringAttribute{
			Description: "Username for basic_auth.",
			Optional:    true,
		},
		"password": schema.StringAttribute{
			Description: "Password for basic_auth.",
			Optional:    true,
			Sensitive:   true,
		},
		"name": schema.StringAttribute{
			Description: "Header/cookie/query parameter name for api_key auth.",
			Optional:    true,
		},
		"value": schema.StringAttribute{
			Description: "API key value for api_key auth.",
			Optional:    true,
			Sensitive:   true,
		},
		"in": schema.StringAttribute{
			Description: "Where to send the API key: 'header', 'cookie', or 'query'.",
			Optional:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("header", "cookie", "query"),
			},
		},
	}
}

// courierHTTPRequestConfigSchemaAttr returns a SingleNestedAttribute for HTTP request config.
func courierHTTPRequestConfigSchemaAttr(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: description,
		Optional:    true,
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "Target URL for the HTTP request.",
				Required:    true,
			},
			"method": schema.StringAttribute{
				Description: "HTTP method (e.g., 'POST', 'PUT').",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("POST", "PUT", "PATCH", "GET"),
				},
			},
			"headers": schema.MapAttribute{
				Description: "Additional HTTP headers to include.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"body": schema.StringAttribute{
				Description: "Request body template. Supports base64:// scheme for Jsonnet templates.",
				Optional:    true,
			},
			"auth": schema.SingleNestedAttribute{
				Description: "Authentication configuration for the HTTP request.",
				Optional:    true,
				Attributes:  courierHTTPAuthSchemaAttrs(),
			},
		},
	}
}

func (r *ProjectConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ProjectConfigResource) buildPatches(ctx context.Context, plan *ProjectConfigResourceModel) []ory.JsonPatch {
	var patches []ory.JsonPatch

	// Keto/Permissions Namespaces
	if !plan.KetoNamespaces.IsNull() && !plan.KetoNamespaces.IsUnknown() {
		var namespaceNames []string
		plan.KetoNamespaces.ElementsAs(ctx, &namespaceNames, false)
		// Convert to the format expected by Ory API: [{name: "...", id: N}, ...]
		namespaces := make([]map[string]interface{}, len(namespaceNames))
		for i, name := range namespaceNames {
			namespaces[i] = map[string]interface{}{
				"name": name,
				"id":   i + 1, // IDs are 1-indexed
			}
		}
		patches = append(patches, ory.JsonPatch{
			Op:    "add",
			Path:  "/services/permission/config/namespaces",
			Value: namespaces,
		})
	}

	// CORS
	if !plan.CorsEnabled.IsNull() && !plan.CorsEnabled.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_public/enabled",
			Value: plan.CorsEnabled.ValueBool(),
		})
	}
	if !plan.CorsOrigins.IsNull() && !plan.CorsOrigins.IsUnknown() {
		var origins []string
		plan.CorsOrigins.ElementsAs(ctx, &origins, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_public/origins",
			Value: origins,
		})
	}

	// Admin CORS
	if !plan.CorsAdminEnabled.IsNull() && !plan.CorsAdminEnabled.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_admin/enabled",
			Value: plan.CorsAdminEnabled.ValueBool(),
		})
	}
	if !plan.CorsAdminOrigins.IsNull() && !plan.CorsAdminOrigins.IsUnknown() {
		var origins []string
		plan.CorsAdminOrigins.ElementsAs(ctx, &origins, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_admin/origins",
			Value: origins,
		})
	}

	// Session
	if !plan.SessionLifespan.IsNull() && !plan.SessionLifespan.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/session/lifespan",
			Value: plan.SessionLifespan.ValueString(),
		})
	}
	if !plan.SessionCookieSameSite.IsNull() && !plan.SessionCookieSameSite.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/session/cookie/same_site",
			Value: plan.SessionCookieSameSite.ValueString(),
		})
	}
	if !plan.SessionCookiePersistent.IsNull() && !plan.SessionCookiePersistent.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/session/cookie/persistent",
			Value: plan.SessionCookiePersistent.ValueBool(),
		})
	}

	// OAuth2/Hydra token lifespans
	if !plan.OAuth2AccessTokenLifespan.IsNull() && !plan.OAuth2AccessTokenLifespan.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/oauth2/config/ttl/access_token",
			Value: plan.OAuth2AccessTokenLifespan.ValueString(),
		})
	}
	if !plan.OAuth2RefreshTokenLifespan.IsNull() && !plan.OAuth2RefreshTokenLifespan.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/oauth2/config/ttl/refresh_token",
			Value: plan.OAuth2RefreshTokenLifespan.ValueString(),
		})
	}

	// URLs
	if !plan.DefaultReturnURL.IsNull() && !plan.DefaultReturnURL.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/default_browser_return_url",
			Value: plan.DefaultReturnURL.ValueString(),
		})
	}
	if !plan.AllowedReturnURLs.IsNull() && !plan.AllowedReturnURLs.IsUnknown() {
		var urls []string
		plan.AllowedReturnURLs.ElementsAs(ctx, &urls, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/allowed_return_urls",
			Value: urls,
		})
	}

	urlMappings := map[*types.String]string{
		&plan.LoginUIURL:        "/services/identity/config/selfservice/flows/login/ui_url",
		&plan.RegistrationUIURL: "/services/identity/config/selfservice/flows/registration/ui_url",
		&plan.RecoveryUIURL:     "/services/identity/config/selfservice/flows/recovery/ui_url",
		&plan.VerificationUIURL: "/services/identity/config/selfservice/flows/verification/ui_url",
		&plan.SettingsUIURL:     "/services/identity/config/selfservice/flows/settings/ui_url",
		&plan.ErrorUIURL:        "/services/identity/config/selfservice/flows/error/ui_url",
	}
	for field, path := range urlMappings {
		if !field.IsNull() && !field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  path,
				Value: field.ValueString(),
			})
		}
	}

	// Auth methods
	methodMappings := map[*types.Bool]string{
		&plan.EnablePassword:     "/services/identity/config/selfservice/methods/password/enabled",
		&plan.EnableCode:         "/services/identity/config/selfservice/methods/code/enabled",
		&plan.EnableTOTP:         "/services/identity/config/selfservice/methods/totp/enabled",
		&plan.EnableWebAuthn:     "/services/identity/config/selfservice/methods/webauthn/enabled",
		&plan.EnablePasskey:      "/services/identity/config/selfservice/methods/passkey/enabled",
		&plan.EnableLookupSecret: "/services/identity/config/selfservice/methods/lookup_secret/enabled",
	}
	for field, path := range methodMappings {
		if !field.IsNull() && !field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  path,
				Value: field.ValueBool(),
			})
		}
	}

	// Password policy
	if !plan.PasswordMinLength.IsNull() && !plan.PasswordMinLength.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/password/config/min_password_length",
			Value: plan.PasswordMinLength.ValueInt64(),
		})
	}
	if !plan.PasswordCheckHaveIBeenPwned.IsNull() && !plan.PasswordCheckHaveIBeenPwned.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/password/config/haveibeenpwned_enabled",
			Value: plan.PasswordCheckHaveIBeenPwned.ValueBool(),
		})
	}
	if !plan.PasswordMaxBreaches.IsNull() && !plan.PasswordMaxBreaches.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/password/config/max_breaches",
			Value: plan.PasswordMaxBreaches.ValueInt64(),
		})
	}
	if !plan.PasswordIdentifierSimilarity.IsNull() && !plan.PasswordIdentifierSimilarity.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/password/config/identifier_similarity_check_enabled",
			Value: plan.PasswordIdentifierSimilarity.ValueBool(),
		})
	}

	// Flow settings
	flowMappings := map[*types.Bool]string{
		&plan.EnableRecovery:     "/services/identity/config/selfservice/flows/recovery/enabled",
		&plan.EnableVerification: "/services/identity/config/selfservice/flows/verification/enabled",
		&plan.EnableRegistration: "/services/identity/config/selfservice/flows/registration/enabled",
	}
	for field, path := range flowMappings {
		if !field.IsNull() && !field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  path,
				Value: field.ValueBool(),
			})
		}
	}

	// SMTP Configuration
	if !plan.SMTPConnectionURI.IsNull() && !plan.SMTPConnectionURI.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/smtp/connection_uri",
			Value: plan.SMTPConnectionURI.ValueString(),
		})
	}
	if !plan.SMTPFromAddress.IsNull() && !plan.SMTPFromAddress.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/smtp/from_address",
			Value: plan.SMTPFromAddress.ValueString(),
		})
	}
	if !plan.SMTPFromName.IsNull() && !plan.SMTPFromName.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/smtp/from_name",
			Value: plan.SMTPFromName.ValueString(),
		})
	}
	if !plan.SMTPHeaders.IsNull() && !plan.SMTPHeaders.IsUnknown() {
		var headers map[string]string
		plan.SMTPHeaders.ElementsAs(ctx, &headers, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/smtp/headers",
			Value: headers,
		})
	}

	// MFA Policy
	if !plan.MFAEnforcement.IsNull() && !plan.MFAEnforcement.IsUnknown() {
		// MFA enforcement is typically handled through required_aal
		// "none" = aal1, "required" = aal2
		enforcement := plan.MFAEnforcement.ValueString()
		if enforcement == "required" {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  "/services/identity/config/selfservice/flows/settings/required_aal",
				Value: "aal2",
			})
		}
	}
	if !plan.TOTPIssuer.IsNull() && !plan.TOTPIssuer.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/totp/config/issuer",
			Value: plan.TOTPIssuer.ValueString(),
		})
	}
	if !plan.WebAuthnRPDisplayName.IsNull() && !plan.WebAuthnRPDisplayName.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/webauthn/config/rp/display_name",
			Value: plan.WebAuthnRPDisplayName.ValueString(),
		})
	}
	if !plan.WebAuthnRPID.IsNull() && !plan.WebAuthnRPID.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/webauthn/config/rp/id",
			Value: plan.WebAuthnRPID.ValueString(),
		})
	}
	if !plan.WebAuthnRPOrigins.IsNull() && !plan.WebAuthnRPOrigins.IsUnknown() {
		var origins []string
		plan.WebAuthnRPOrigins.ElementsAs(ctx, &origins, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/webauthn/config/rp/origins",
			Value: origins,
		})
	}
	if !plan.WebAuthnPasswordless.IsNull() && !plan.WebAuthnPasswordless.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/methods/webauthn/config/passwordless",
			Value: plan.WebAuthnPasswordless.ValueBool(),
		})
	}
	if !plan.RequiredAAL.IsNull() && !plan.RequiredAAL.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/selfservice/flows/settings/required_aal",
			Value: plan.RequiredAAL.ValueString(),
		})
	}
	if !plan.SessionWhoamiRequiredAAL.IsNull() && !plan.SessionWhoamiRequiredAAL.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/session/whoami/required_aal",
			Value: plan.SessionWhoamiRequiredAAL.ValueString(),
		})
	}

	// Account Experience (Branding)
	if !plan.AccountExperienceFaviconURL.IsNull() && !plan.AccountExperienceFaviconURL.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/account_experience/config/favicon_url",
			Value: plan.AccountExperienceFaviconURL.ValueString(),
		})
	}
	if !plan.AccountExperienceLogoURL.IsNull() && !plan.AccountExperienceLogoURL.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/account_experience/config/logo_url",
			Value: plan.AccountExperienceLogoURL.ValueString(),
		})
	}
	if !plan.AccountExperienceName.IsNull() && !plan.AccountExperienceName.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/account_experience/config/name",
			Value: plan.AccountExperienceName.ValueString(),
		})
	}
	if !plan.AccountExperienceStylesheet.IsNull() && !plan.AccountExperienceStylesheet.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/account_experience/config/stylesheet",
			Value: plan.AccountExperienceStylesheet.ValueString(),
		})
	}
	if !plan.AccountExperienceLocale.IsNull() && !plan.AccountExperienceLocale.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/account_experience/config/default_locale",
			Value: plan.AccountExperienceLocale.ValueString(),
		})
	}

	// Session Tokenizer Templates
	if !plan.SessionTokenizerTemplates.IsNull() && !plan.SessionTokenizerTemplates.IsUnknown() {
		var templates map[string]SessionTokenizerTemplateModel
		plan.SessionTokenizerTemplates.ElementsAs(ctx, &templates, false)
		templatesMap := make(map[string]interface{}, len(templates))
		for name, tmpl := range templates {
			entry := map[string]interface{}{}
			if !tmpl.TTL.IsNull() && !tmpl.TTL.IsUnknown() {
				entry["ttl"] = tmpl.TTL.ValueString()
			}
			if !tmpl.JWKSURL.IsNull() && !tmpl.JWKSURL.IsUnknown() {
				entry["jwks_url"] = tmpl.JWKSURL.ValueString()
			}
			if !tmpl.ClaimsMapperURL.IsNull() && !tmpl.ClaimsMapperURL.IsUnknown() {
				entry["claims_mapper_url"] = tmpl.ClaimsMapperURL.ValueString()
			}
			if !tmpl.SubjectSource.IsNull() && !tmpl.SubjectSource.IsUnknown() {
				entry["subject_source"] = tmpl.SubjectSource.ValueString()
			}
			templatesMap[name] = entry
		}
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/session/whoami/tokenizer/templates",
			Value: templatesMap,
		})
	}

	// Courier Delivery Strategy
	if !plan.CourierDeliveryStrategy.IsNull() && !plan.CourierDeliveryStrategy.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/delivery_strategy",
			Value: plan.CourierDeliveryStrategy.ValueString(),
		})
	}

	// Courier HTTP Request Config
	if !plan.CourierHTTPRequestConfig.IsNull() && !plan.CourierHTTPRequestConfig.IsUnknown() {
		var reqConfig CourierHTTPRequestConfigModel
		plan.CourierHTTPRequestConfig.As(ctx, &reqConfig, basetypes.ObjectAsOptions{})
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/http/request_config",
			Value: buildHTTPRequestConfigMap(ctx, &reqConfig),
		})
	}

	// Courier Channels
	if !plan.CourierChannels.IsNull() && !plan.CourierChannels.IsUnknown() {
		var channels []CourierChannelModel
		plan.CourierChannels.ElementsAs(ctx, &channels, false)
		channelsList := make([]map[string]interface{}, 0, len(channels))
		for _, ch := range channels {
			chMap := map[string]interface{}{
				"id": ch.ID.ValueString(),
			}
			if !ch.RequestConfig.IsNull() && !ch.RequestConfig.IsUnknown() {
				var reqConfig CourierHTTPRequestConfigModel
				ch.RequestConfig.As(ctx, &reqConfig, basetypes.ObjectAsOptions{})
				chMap["request_config"] = buildHTTPRequestConfigMap(ctx, &reqConfig)
			}
			channelsList = append(channelsList, chMap)
		}
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/channels",
			Value: channelsList,
		})
	}

	return patches
}

// buildHTTPRequestConfigMap converts a CourierHTTPRequestConfigModel to a map for JSON Patch.
func buildHTTPRequestConfigMap(ctx context.Context, cfg *CourierHTTPRequestConfigModel) map[string]interface{} {
	result := map[string]interface{}{
		"url":    cfg.URL.ValueString(),
		"method": cfg.Method.ValueString(),
	}
	if !cfg.Body.IsNull() && !cfg.Body.IsUnknown() {
		result["body"] = cfg.Body.ValueString()
	}
	if !cfg.Headers.IsNull() && !cfg.Headers.IsUnknown() {
		var headers map[string]string
		cfg.Headers.ElementsAs(ctx, &headers, false)
		result["headers"] = headers
	}
	if !cfg.Auth.IsNull() && !cfg.Auth.IsUnknown() {
		var auth CourierHTTPAuthModel
		cfg.Auth.As(ctx, &auth, basetypes.ObjectAsOptions{})
		result["auth"] = buildAuthConfigMap(&auth)
	}
	return result
}

// buildAuthConfigMap converts a flattened CourierHTTPAuthModel to the nested API format:
// {"type": "...", "config": {...}}
func buildAuthConfigMap(auth *CourierHTTPAuthModel) map[string]interface{} {
	authType := auth.Type.ValueString()
	config := map[string]interface{}{}
	switch authType {
	case "basic_auth":
		if !auth.User.IsNull() && !auth.User.IsUnknown() {
			config["user"] = auth.User.ValueString()
		}
		if !auth.Password.IsNull() && !auth.Password.IsUnknown() {
			config["password"] = auth.Password.ValueString()
		}
	case "api_key":
		if !auth.Name.IsNull() && !auth.Name.IsUnknown() {
			config["name"] = auth.Name.ValueString()
		}
		if !auth.Value.IsNull() && !auth.Value.IsUnknown() {
			config["value"] = auth.Value.ValueString()
		}
		if !auth.In.IsNull() && !auth.In.IsUnknown() {
			config["in"] = auth.In.ValueString()
		}
	}
	return map[string]interface{}{
		"type":   authType,
		"config": config,
	}
}

func (r *ProjectConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProjectConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	patches := r.buildPatches(ctx, &plan)

	// Debug: Log the patches being built (only at Debug level to avoid exposing sensitive paths)
	tflog.Debug(ctx, "Building project config patches", map[string]interface{}{
		"project_id":  projectID,
		"patch_count": len(patches),
	})

	if len(patches) > 0 {
		_, err := r.client.PatchProject(ctx, projectID, patches)
		if err != nil {
			resp.Diagnostics.AddError("Error Applying Project Config", err.Error())
			return
		}
		tflog.Debug(ctx, "Successfully applied project config patches", map[string]interface{}{
			"project_id":  projectID,
			"patch_count": len(patches),
		})
	}

	plan.ID = types.StringValue(projectID)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProjectConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var project *ory.Project
	if cached := r.client.GetCachedProject(projectID); cached != nil {
		project = cached
	} else {
		var err error
		project, err = r.client.GetProject(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Project Config",
				"Could not read project "+projectID+": "+err.Error())
			return
		}
	}

	r.readProjectConfig(ctx, project, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// getNestedValue safely traverses nested maps to extract a value.
func getNestedValue(config map[string]interface{}, keys ...string) interface{} {
	current := interface{}(config)
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[key]
		if !ok {
			return nil
		}
	}
	return current
}

// getNestedString extracts a string from nested maps, returning ("", false) if not found.
func getNestedString(config map[string]interface{}, keys ...string) (string, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// getNestedBool extracts a bool from nested maps, returning (false, false) if not found.
func getNestedBool(config map[string]interface{}, keys ...string) (bool, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// getNestedFloat extracts a number from nested maps (JSON numbers are float64).
func getNestedFloat(config map[string]interface{}, keys ...string) (float64, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

// readProjectConfig reads the project configuration from the API response into the Terraform state.
// Only updates attributes that are already set in state (non-null) to avoid importing defaults.
func (r *ProjectConfigResource) readProjectConfig(ctx context.Context, project *ory.Project, state *ProjectConfigResourceModel) {
	// CORS (Public)
	if project.CorsPublic != nil {
		if !state.CorsEnabled.IsNull() {
			if project.CorsPublic.Enabled != nil {
				state.CorsEnabled = types.BoolValue(*project.CorsPublic.Enabled)
			}
		}
		if !state.CorsOrigins.IsNull() {
			if len(project.CorsPublic.Origins) > 0 {
				originsList, diags := types.ListValueFrom(ctx, types.StringType, project.CorsPublic.Origins)
				if !diags.HasError() {
					state.CorsOrigins = originsList
				}
			}
		}
	}

	// CORS (Admin)
	if project.CorsAdmin != nil {
		if !state.CorsAdminEnabled.IsNull() {
			if project.CorsAdmin.Enabled != nil {
				state.CorsAdminEnabled = types.BoolValue(*project.CorsAdmin.Enabled)
			}
		}
		if !state.CorsAdminOrigins.IsNull() {
			if len(project.CorsAdmin.Origins) > 0 {
				originsList, diags := types.ListValueFrom(ctx, types.StringType, project.CorsAdmin.Origins)
				if !diags.HasError() {
					state.CorsAdminOrigins = originsList
				}
			}
		}
	}

	// Identity service config
	if project.Services.Identity != nil {
		identityConfig := project.Services.Identity.Config

		// Session
		if !state.SessionLifespan.IsNull() {
			if v, ok := getNestedString(identityConfig, "session", "lifespan"); ok {
				state.SessionLifespan = types.StringValue(v)
			}
		}
		if !state.SessionCookieSameSite.IsNull() {
			if v, ok := getNestedString(identityConfig, "session", "cookie", "same_site"); ok {
				state.SessionCookieSameSite = types.StringValue(v)
			}
		}
		if !state.SessionCookiePersistent.IsNull() {
			if v, ok := getNestedBool(identityConfig, "session", "cookie", "persistent"); ok {
				state.SessionCookiePersistent = types.BoolValue(v)
			}
		}

		// URLs
		if !state.DefaultReturnURL.IsNull() {
			if v, ok := getNestedString(identityConfig, "selfservice", "default_browser_return_url"); ok {
				state.DefaultReturnURL = types.StringValue(v)
			}
		}
		if !state.AllowedReturnURLs.IsNull() {
			if v := getNestedValue(identityConfig, "selfservice", "allowed_return_urls"); v != nil {
				if urls, ok := v.([]interface{}); ok && len(urls) > 0 {
					strs := make([]string, 0, len(urls))
					for _, u := range urls {
						if s, ok := u.(string); ok {
							strs = append(strs, s)
						}
					}
					urlsList, diags := types.ListValueFrom(ctx, types.StringType, strs)
					if !diags.HasError() {
						state.AllowedReturnURLs = urlsList
					}
				}
			}
		}

		urlReadMappings := map[*types.String][]string{
			&state.LoginUIURL:        {"selfservice", "flows", "login", "ui_url"},
			&state.RegistrationUIURL: {"selfservice", "flows", "registration", "ui_url"},
			&state.RecoveryUIURL:     {"selfservice", "flows", "recovery", "ui_url"},
			&state.VerificationUIURL: {"selfservice", "flows", "verification", "ui_url"},
			&state.SettingsUIURL:     {"selfservice", "flows", "settings", "ui_url"},
			&state.ErrorUIURL:        {"selfservice", "flows", "error", "ui_url"},
		}
		for field, keys := range urlReadMappings {
			if !field.IsNull() {
				if v, ok := getNestedString(identityConfig, keys...); ok {
					*field = types.StringValue(v)
				}
			}
		}

		// Auth methods
		methodReadMappings := map[*types.Bool][]string{
			&state.EnablePassword:     {"selfservice", "methods", "password", "enabled"},
			&state.EnableCode:         {"selfservice", "methods", "code", "enabled"},
			&state.EnableTOTP:         {"selfservice", "methods", "totp", "enabled"},
			&state.EnableWebAuthn:     {"selfservice", "methods", "webauthn", "enabled"},
			&state.EnablePasskey:      {"selfservice", "methods", "passkey", "enabled"},
			&state.EnableLookupSecret: {"selfservice", "methods", "lookup_secret", "enabled"},
		}
		for field, keys := range methodReadMappings {
			if !field.IsNull() {
				if v, ok := getNestedBool(identityConfig, keys...); ok {
					*field = types.BoolValue(v)
				}
			}
		}

		// Password policy
		if !state.PasswordMinLength.IsNull() {
			if v, ok := getNestedFloat(identityConfig, "selfservice", "methods", "password", "config", "min_password_length"); ok {
				state.PasswordMinLength = types.Int64Value(int64(v))
			}
		}
		if !state.PasswordCheckHaveIBeenPwned.IsNull() {
			if v, ok := getNestedBool(identityConfig, "selfservice", "methods", "password", "config", "haveibeenpwned_enabled"); ok {
				state.PasswordCheckHaveIBeenPwned = types.BoolValue(v)
			}
		}
		if !state.PasswordMaxBreaches.IsNull() {
			if v, ok := getNestedFloat(identityConfig, "selfservice", "methods", "password", "config", "max_breaches"); ok {
				state.PasswordMaxBreaches = types.Int64Value(int64(v))
			}
		}
		if !state.PasswordIdentifierSimilarity.IsNull() {
			if v, ok := getNestedBool(identityConfig, "selfservice", "methods", "password", "config", "identifier_similarity_check_enabled"); ok {
				state.PasswordIdentifierSimilarity = types.BoolValue(v)
			}
		}

		// Flow settings
		flowReadMappings := map[*types.Bool][]string{
			&state.EnableRecovery:     {"selfservice", "flows", "recovery", "enabled"},
			&state.EnableVerification: {"selfservice", "flows", "verification", "enabled"},
			&state.EnableRegistration: {"selfservice", "flows", "registration", "enabled"},
		}
		for field, keys := range flowReadMappings {
			if !field.IsNull() {
				if v, ok := getNestedBool(identityConfig, keys...); ok {
					*field = types.BoolValue(v)
				}
			}
		}

		// SMTP (skip smtp_connection_uri — it's sensitive and may not be returned)
		if !state.SMTPFromAddress.IsNull() {
			if v, ok := getNestedString(identityConfig, "courier", "smtp", "from_address"); ok {
				state.SMTPFromAddress = types.StringValue(v)
			}
		}
		if !state.SMTPFromName.IsNull() {
			if v, ok := getNestedString(identityConfig, "courier", "smtp", "from_name"); ok {
				state.SMTPFromName = types.StringValue(v)
			}
		}

		// MFA / WebAuthn
		if !state.TOTPIssuer.IsNull() {
			if v, ok := getNestedString(identityConfig, "selfservice", "methods", "totp", "config", "issuer"); ok {
				state.TOTPIssuer = types.StringValue(v)
			}
		}
		if !state.WebAuthnRPDisplayName.IsNull() {
			if v, ok := getNestedString(identityConfig, "selfservice", "methods", "webauthn", "config", "rp", "display_name"); ok {
				state.WebAuthnRPDisplayName = types.StringValue(v)
			}
		}
		if !state.WebAuthnRPID.IsNull() {
			if v, ok := getNestedString(identityConfig, "selfservice", "methods", "webauthn", "config", "rp", "id"); ok {
				state.WebAuthnRPID = types.StringValue(v)
			}
		}
		if !state.WebAuthnRPOrigins.IsNull() {
			if v := getNestedValue(identityConfig, "selfservice", "methods", "webauthn", "config", "rp", "origins"); v != nil {
				if origins, ok := v.([]interface{}); ok && len(origins) > 0 {
					strs := make([]string, 0, len(origins))
					for _, o := range origins {
						if s, ok := o.(string); ok {
							strs = append(strs, s)
						}
					}
					originsList, diags := types.ListValueFrom(ctx, types.StringType, strs)
					if !diags.HasError() {
						state.WebAuthnRPOrigins = originsList
					}
				}
			}
		}
		if !state.WebAuthnPasswordless.IsNull() {
			if v, ok := getNestedBool(identityConfig, "selfservice", "methods", "webauthn", "config", "passwordless"); ok {
				state.WebAuthnPasswordless = types.BoolValue(v)
			}
		}
		if !state.RequiredAAL.IsNull() {
			if v, ok := getNestedString(identityConfig, "selfservice", "flows", "settings", "required_aal"); ok {
				state.RequiredAAL = types.StringValue(v)
			}
		}
		if !state.SessionWhoamiRequiredAAL.IsNull() {
			if v, ok := getNestedString(identityConfig, "session", "whoami", "required_aal"); ok {
				state.SessionWhoamiRequiredAAL = types.StringValue(v)
			}
		}

		// Session Tokenizer Templates
		if !state.SessionTokenizerTemplates.IsNull() {
			if v := getNestedValue(identityConfig, "session", "whoami", "tokenizer", "templates"); v != nil {
				if templatesRaw, ok := v.(map[string]interface{}); ok && len(templatesRaw) > 0 {
					templateObjects := make(map[string]attr.Value, len(templatesRaw))
					for name, tmplRaw := range templatesRaw {
						tmplMap, ok := tmplRaw.(map[string]interface{})
						if !ok {
							continue
						}
						attrs := map[string]attr.Value{
							"ttl":               types.StringNull(),
							"jwks_url":          types.StringNull(),
							"claims_mapper_url": types.StringNull(),
							"subject_source":    types.StringNull(),
						}
						if s, ok := tmplMap["ttl"].(string); ok && s != "" {
							// API normalizes durations (e.g. "1h" → "1h0m0s"), preserve state value
							attrs["ttl"] = preserveTokenizerField(state, name, "ttl", s)
						}
						if _, ok := tmplMap["jwks_url"].(string); ok {
							// jwks_url is sensitive — preserve state value to avoid diff
							attrs["jwks_url"] = preserveTokenizerField(state, name, "jwks_url", "")
						}
						if s, ok := tmplMap["claims_mapper_url"].(string); ok && s != "" {
							if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
								// API may return GCS URL instead of base64 — preserve state value
								attrs["claims_mapper_url"] = preserveTokenizerField(state, name, "claims_mapper_url", s)
							} else {
								attrs["claims_mapper_url"] = types.StringValue(s)
							}
						}
						if s, ok := tmplMap["subject_source"].(string); ok && s != "" {
							attrs["subject_source"] = types.StringValue(s)
						}
						objVal, diags := types.ObjectValue(tokenizerTemplateAttrTypes, attrs)
						if !diags.HasError() {
							templateObjects[name] = objVal
						}
					}
					mapVal, diags := types.MapValue(types.ObjectType{AttrTypes: tokenizerTemplateAttrTypes}, templateObjects)
					if !diags.HasError() {
						state.SessionTokenizerTemplates = mapVal
					}
				}
			}
		}

		// Courier Delivery Strategy
		if !state.CourierDeliveryStrategy.IsNull() {
			if v, ok := getNestedString(identityConfig, "courier", "delivery_strategy"); ok {
				state.CourierDeliveryStrategy = types.StringValue(v)
			}
		}

		// Courier HTTP Request Config
		if !state.CourierHTTPRequestConfig.IsNull() {
			if v := getNestedValue(identityConfig, "courier", "http", "request_config"); v != nil {
				if reqCfgRaw, ok := v.(map[string]interface{}); ok {
					objVal := readHTTPRequestConfigObject(ctx, reqCfgRaw, state.CourierHTTPRequestConfig)
					if !objVal.IsNull() {
						state.CourierHTTPRequestConfig = objVal
					}
				}
			}
		}

		// Courier Channels
		if !state.CourierChannels.IsNull() {
			if v := getNestedValue(identityConfig, "courier", "channels"); v != nil {
				if channelsRaw, ok := v.([]interface{}); ok && len(channelsRaw) > 0 {
					channelObjects := make([]attr.Value, 0, len(channelsRaw))
					for _, chRaw := range channelsRaw {
						chMap, ok := chRaw.(map[string]interface{})
						if !ok {
							continue
						}
						attrs := map[string]attr.Value{
							"id":             types.StringNull(),
							"request_config": types.ObjectNull(courierHTTPRequestConfigAttrTypes),
						}
						if id, ok := chMap["id"].(string); ok {
							attrs["id"] = types.StringValue(id)
						}
						if rc, ok := chMap["request_config"].(map[string]interface{}); ok {
							// Find matching state channel to preserve sensitive fields
							stateRC := findChannelRequestConfig(state.CourierChannels, attrs["id"])
							attrs["request_config"] = readHTTPRequestConfigObject(ctx, rc, stateRC)
						}
						objVal, diags := types.ObjectValue(courierChannelAttrTypes, attrs)
						if !diags.HasError() {
							channelObjects = append(channelObjects, objVal)
						}
					}
					listVal, diags := types.ListValue(types.ObjectType{AttrTypes: courierChannelAttrTypes}, channelObjects)
					if !diags.HasError() {
						state.CourierChannels = listVal
					}
				}
			}
		}
	}

	// OAuth2 service config
	if project.Services.Oauth2 != nil {
		oauth2Config := project.Services.Oauth2.Config

		if !state.OAuth2AccessTokenLifespan.IsNull() {
			if v, ok := getNestedString(oauth2Config, "ttl", "access_token"); ok {
				state.OAuth2AccessTokenLifespan = types.StringValue(v)
			}
		}
		if !state.OAuth2RefreshTokenLifespan.IsNull() {
			if v, ok := getNestedString(oauth2Config, "ttl", "refresh_token"); ok {
				state.OAuth2RefreshTokenLifespan = types.StringValue(v)
			}
		}
	}

	// Permission/Keto service config
	if project.Services.Permission != nil && !state.KetoNamespaces.IsNull() {
		permConfig := project.Services.Permission.Config
		if v := getNestedValue(permConfig, "namespaces"); v != nil {
			if nsList, ok := v.([]interface{}); ok && len(nsList) > 0 {
				names := make([]string, 0, len(nsList))
				for _, ns := range nsList {
					if nsMap, ok := ns.(map[string]interface{}); ok {
						if name, ok := nsMap["name"].(string); ok {
							names = append(names, name)
						}
					}
				}
				if len(names) > 0 {
					namesList, diags := types.ListValueFrom(ctx, types.StringType, names)
					if !diags.HasError() {
						state.KetoNamespaces = namesList
					}
				}
			}
		}
	}

	// Account Experience config
	if project.Services.AccountExperience != nil {
		aeConfig := project.Services.AccountExperience.Config

		aeStringMappings := map[*types.String]string{
			&state.AccountExperienceFaviconURL: "favicon_url",
			&state.AccountExperienceLogoURL:    "logo_url",
			&state.AccountExperienceName:       "name",
			&state.AccountExperienceStylesheet: "stylesheet",
			&state.AccountExperienceLocale:     "default_locale",
		}
		for field, key := range aeStringMappings {
			if !field.IsNull() {
				if v, ok := getNestedString(aeConfig, key); ok && v != "" {
					*field = types.StringValue(v)
				}
			}
		}
	}
}

// preserveTokenizerField returns the value of a field from the existing state for a given template name.
// Used for sensitive fields (jwks_url), fields where the API normalizes values (ttl),
// and fields where the API returns GCS URLs instead of base64 (claims_mapper_url).
// If no state value exists, falls back to the provided API value (or StringNull if empty).
func preserveTokenizerField(state *ProjectConfigResourceModel, templateName, fieldName, apiValue string) basetypes.StringValue {
	if state.SessionTokenizerTemplates.IsNull() || state.SessionTokenizerTemplates.IsUnknown() {
		if apiValue != "" {
			return types.StringValue(apiValue)
		}
		return types.StringNull()
	}
	elems := state.SessionTokenizerTemplates.Elements()
	if tmplVal, ok := elems[templateName]; ok {
		if objVal, ok := tmplVal.(types.Object); ok && !objVal.IsNull() {
			attrs := objVal.Attributes()
			if v, ok := attrs[fieldName]; ok {
				if s, ok := v.(types.String); ok && !s.IsNull() {
					return s
				}
			}
		}
	}
	if apiValue != "" {
		return types.StringValue(apiValue)
	}
	return types.StringNull()
}

// readHTTPRequestConfigObject reads an HTTP request config from the API response map
// into a types.Object. Preserves sensitive auth fields from the existing state object.
func readHTTPRequestConfigObject(_ context.Context, raw map[string]interface{}, stateObj basetypes.ObjectValue) basetypes.ObjectValue {
	attrs := map[string]attr.Value{
		"url":     types.StringNull(),
		"method":  types.StringNull(),
		"headers": types.MapNull(types.StringType),
		"body":    types.StringNull(),
		"auth":    types.ObjectNull(courierHTTPAuthAttrTypes),
	}

	if s, ok := raw["url"].(string); ok {
		attrs["url"] = types.StringValue(s)
	}
	if s, ok := raw["method"].(string); ok {
		attrs["method"] = types.StringValue(strings.ToUpper(s))
	}
	if s, ok := raw["body"].(string); ok && s != "" {
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
			// API may return GCS URL instead of base64 content — preserve state value
			attrs["body"] = getStateRequestConfigField(stateObj, "body")
		} else {
			attrs["body"] = types.StringValue(s)
		}
	}
	if hdrs, ok := raw["headers"].(map[string]interface{}); ok && len(hdrs) > 0 {
		strHdrs := make(map[string]attr.Value, len(hdrs))
		for k, v := range hdrs {
			if s, ok := v.(string); ok {
				strHdrs[k] = types.StringValue(s)
			}
		}
		mapVal, diags := types.MapValue(types.StringType, strHdrs)
		if !diags.HasError() {
			attrs["headers"] = mapVal
		}
	}
	if authRaw, ok := raw["auth"].(map[string]interface{}); ok {
		attrs["auth"] = readAuthObject(authRaw, stateObj)
	}

	objVal, diags := types.ObjectValue(courierHTTPRequestConfigAttrTypes, attrs)
	if diags.HasError() {
		return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
	}
	return objVal
}

// readAuthObject reads an auth config from the API response (nested {"type","config":{...}} format)
// and returns a flat types.Object. Preserves sensitive fields (password, value) from state.
func readAuthObject(raw map[string]interface{}, parentStateObj basetypes.ObjectValue) basetypes.ObjectValue {
	attrs := map[string]attr.Value{
		"type":     types.StringNull(),
		"user":     types.StringNull(),
		"password": types.StringNull(),
		"name":     types.StringNull(),
		"value":    types.StringNull(),
		"in":       types.StringNull(),
	}

	authType, _ := raw["type"].(string)
	if authType == "" {
		return types.ObjectNull(courierHTTPAuthAttrTypes)
	}
	attrs["type"] = types.StringValue(authType)

	config, _ := raw["config"].(map[string]interface{})
	if config == nil {
		config = map[string]interface{}{}
	}

	switch authType {
	case "basic_auth":
		if s, ok := config["user"].(string); ok {
			attrs["user"] = types.StringValue(s)
		}
		// password is sensitive — preserve from state
		attrs["password"] = getStateAuthField(parentStateObj, "password")
	case "api_key":
		if s, ok := config["name"].(string); ok {
			attrs["name"] = types.StringValue(s)
		}
		if s, ok := config["in"].(string); ok {
			attrs["in"] = types.StringValue(s)
		}
		// value is sensitive — preserve from state
		attrs["value"] = getStateAuthField(parentStateObj, "value")
	}

	objVal, diags := types.ObjectValue(courierHTTPAuthAttrTypes, attrs)
	if diags.HasError() {
		return types.ObjectNull(courierHTTPAuthAttrTypes)
	}
	return objVal
}

// getStateRequestConfigField extracts a field from the existing state's request_config object.
func getStateRequestConfigField(stateObj basetypes.ObjectValue, field string) basetypes.StringValue {
	if stateObj.IsNull() || stateObj.IsUnknown() {
		return types.StringNull()
	}
	attrs := stateObj.Attributes()
	if v, ok := attrs[field]; ok {
		if s, ok := v.(types.String); ok && !s.IsNull() {
			return s
		}
	}
	return types.StringNull()
}

// getStateAuthField extracts a sensitive auth field from the existing state's request_config.auth object.
func getStateAuthField(parentStateObj basetypes.ObjectValue, field string) basetypes.StringValue {
	if parentStateObj.IsNull() || parentStateObj.IsUnknown() {
		return types.StringNull()
	}
	parentAttrs := parentStateObj.Attributes()
	authVal, ok := parentAttrs["auth"]
	if !ok {
		return types.StringNull()
	}
	authObj, ok := authVal.(types.Object)
	if !ok || authObj.IsNull() || authObj.IsUnknown() {
		return types.StringNull()
	}
	authAttrs := authObj.Attributes()
	if v, ok := authAttrs[field]; ok {
		if s, ok := v.(types.String); ok && !s.IsNull() {
			return s
		}
	}
	return types.StringNull()
}

// findChannelRequestConfig finds the request_config object for a channel by ID from the state's courier_channels list.
func findChannelRequestConfig(stateChannels basetypes.ListValue, channelID attr.Value) basetypes.ObjectValue {
	if stateChannels.IsNull() || stateChannels.IsUnknown() {
		return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
	}
	idStr, ok := channelID.(types.String)
	if !ok || idStr.IsNull() {
		return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
	}
	for _, elem := range stateChannels.Elements() {
		chObj, ok := elem.(types.Object)
		if !ok || chObj.IsNull() {
			continue
		}
		chAttrs := chObj.Attributes()
		if chID, ok := chAttrs["id"].(types.String); ok && chID.ValueString() == idStr.ValueString() {
			if rc, ok := chAttrs["request_config"].(types.Object); ok {
				return rc
			}
		}
	}
	return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
}

func (r *ProjectConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProjectConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	patches := r.buildPatches(ctx, &plan)
	if len(patches) > 0 {
		_, err := r.client.PatchProject(ctx, projectID, patches)
		if err != nil {
			resp.Diagnostics.AddError("Error Updating Project Config", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(projectID)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Config cannot be deleted - it just exists. We leave the config as-is.
}

func (r *ProjectConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID := req.ID

	// Set both id and project_id from the import ID
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)

	// Add a warning to help users understand how import works for this resource
	resp.Diagnostics.AddWarning(
		"Project Config Import - Read Your Existing Config",
		"The project config has been imported with project_id: "+projectID+".\n\n"+
			"IMPORTANT: After import, you must ensure your Terraform configuration matches the imported project:\n\n"+
			"Option 1 - Set project_id explicitly:\n"+
			"  resource \"ory_project_config\" \"main\" {\n"+
			"    project_id = \""+projectID+"\"\n"+
			"    # ... your config\n"+
			"  }\n\n"+
			"Option 2 - Use provider default:\n"+
			"  provider \"ory\" {\n"+
			"    project_id = \""+projectID+"\"\n"+
			"  }\n\n"+
			"  resource \"ory_project_config\" \"main\" {\n"+
			"    # project_id inherits from provider\n"+
			"  }\n\n"+
			"If you see 'project_id forces replacement', the project_id in your config doesn't match the imported project.")
}
