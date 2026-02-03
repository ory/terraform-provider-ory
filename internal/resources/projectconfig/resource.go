package projectconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
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

	// CORS
	CorsEnabled types.Bool `tfsdk:"cors_enabled"`
	CorsOrigins types.List `tfsdk:"cors_origins"`

	// Session
	SessionLifespan       types.String `tfsdk:"session_lifespan"`
	SessionCookieSameSite types.String `tfsdk:"session_cookie_same_site"`

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
}

func (r *ProjectConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_config"
}

func (r *ProjectConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures an Ory Network project's settings.",
		MarkdownDescription: `
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
`,
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

			// Session
			"session_lifespan": schema.StringAttribute{
				Description: "Session duration (e.g., '24h0m0s').",
				Optional:    true,
			},
			"session_cookie_same_site": schema.StringAttribute{
				Description: "SameSite cookie attribute (Lax, Strict, None).",
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
				Description: "SMTP connection URI (e.g., 'smtp://user:pass@smtp.example.com:587/').",
				Optional:    true,
				Sensitive:   true,
			},
			"smtp_from_address": schema.StringAttribute{
				Description: "Email address to send from.",
				Optional:    true,
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

	return patches
}

func (r *ProjectConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProjectConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	patches := r.buildPatches(ctx, &plan)

	// Debug: Log the patches being built
	patchesJSON, _ := json.Marshal(patches)
	tflog.Warn(ctx, fmt.Sprintf("Building project config patches: project_id=%s patch_count=%d patches=%s",
		projectID, len(patches), string(patchesJSON)))

	if len(patches) > 0 {
		_, err := r.client.PatchProject(ctx, projectID, patches)
		if err != nil {
			resp.Diagnostics.AddError("Error Applying Project Config", err.Error())
			return
		}
		tflog.Warn(ctx, fmt.Sprintf("Successfully applied project config patches: project_id=%s patch_count=%d",
			projectID, len(patches)))
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

	// Config exists as long as project exists - nothing to read back
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ProjectConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProjectConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
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
