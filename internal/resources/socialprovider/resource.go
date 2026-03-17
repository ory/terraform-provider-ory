package socialprovider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ resource.Resource                   = &SocialProviderResource{}
	_ resource.ResourceWithConfigure      = &SocialProviderResource{}
	_ resource.ResourceWithImportState    = &SocialProviderResource{}
	_ resource.ResourceWithValidateConfig = &SocialProviderResource{}
)

func NewResource() resource.Resource {
	return &SocialProviderResource{}
}

type SocialProviderResource struct {
	client *client.OryClient
}

type SocialProviderResourceModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	ProviderID        types.String `tfsdk:"provider_id"`
	ProviderType      types.String `tfsdk:"provider_type"`
	ClientID          types.String `tfsdk:"client_id"`
	ClientSecret      types.String `tfsdk:"client_secret"`
	IssuerURL         types.String `tfsdk:"issuer_url"`
	Scope             types.List   `tfsdk:"scope"`
	MapperURL         types.String `tfsdk:"mapper_url"`
	AuthURL           types.String `tfsdk:"auth_url"`
	TokenURL          types.String `tfsdk:"token_url"`
	Tenant            types.String `tfsdk:"tenant"`
	AppleTeamID       types.String `tfsdk:"apple_team_id"`
	ApplePrivateKeyID types.String `tfsdk:"apple_private_key_id"`
	ApplePrivateKey   types.String `tfsdk:"apple_private_key"`
	AutoLink          types.Bool   `tfsdk:"auto_link"`
	BaseRedirectURI   types.String `tfsdk:"base_redirect_uri"`
}

func (r *SocialProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_social_provider"
}

func (r *SocialProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network social sign-in provider (Google, GitHub, etc.).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID (same as provider_id).",
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
			"provider_id": schema.StringAttribute{
				Description: "Unique identifier for the provider (used in callback URLs).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"provider_type": schema.StringAttribute{
				Description: "Provider type (google, github, microsoft, apple, generic, etc.).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"client_id": schema.StringAttribute{
				Description: "OAuth2 client ID from the provider.",
				Required:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "OAuth2 client secret from the provider. Required for all providers except Apple (where Ory generates the secret from apple_team_id, apple_private_key_id, and apple_private_key).",
				Optional:    true,
				Sensitive:   true,
			},
			"issuer_url": schema.StringAttribute{
				Description: "OIDC issuer URL (required for generic providers).",
				Optional:    true,
			},
			"scope": schema.ListAttribute{
				Description: "OAuth2 scopes to request.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"mapper_url": schema.StringAttribute{
				Description: "Jsonnet mapper URL for claims mapping. Can be a URL or base64-encoded Jsonnet (base64://...). If not set, a default mapper that extracts email from claims will be used.",
				Optional:    true,
			},
			"auth_url": schema.StringAttribute{
				Description: "Custom authorization URL (for non-standard providers).",
				Optional:    true,
			},
			"token_url": schema.StringAttribute{
				Description: "Custom token URL (for non-standard providers).",
				Optional:    true,
			},
			"tenant": schema.StringAttribute{
				Description: "Tenant ID (for Microsoft/Azure providers).",
				Optional:    true,
			},
			"apple_team_id": schema.StringAttribute{
				Description: "Apple Developer Team ID (e.g., \"KP76DQS54M\"). Required when provider_type is \"apple\" and client_secret is not set.",
				Optional:    true,
			},
			"apple_private_key_id": schema.StringAttribute{
				Description: "Apple private key ID from the Apple Developer portal (e.g., \"UX56C66723\"). Required when provider_type is \"apple\" and client_secret is not set.",
				Optional:    true,
			},
			"apple_private_key": schema.StringAttribute{
				Description: "Apple private key in PEM format (contents of the .p8 file). Required when provider_type is \"apple\" and client_secret is not set. Ory uses this to generate the JWT client secret automatically.",
				Optional:    true,
				Sensitive:   true,
			},
			"auto_link": schema.BoolAttribute{
				Description: "Enable automatic account linking for this provider. When true, if an identity with the same identifier (e.g., email) already exists, the social sign-in will automatically link to that identity instead of failing. Requires enable_oidc_auto_link_policy to be true in the project config (ory_project_config). This attribute is write-only — the API accepts it on create/update but does not return it on read, so Terraform preserves the value from state. On import, the value will not be populated. To disable auto-linking, explicitly set auto_link = false rather than removing the attribute.",
				Optional:    true,
			},
			"base_redirect_uri": schema.StringAttribute{
				Description: "Override the base redirect URI for OIDC callbacks (e.g., \"https://iam.example.com\"). When set, Ory constructs callback URLs using this base instead of the default project domain. This is a global OIDC config setting — if multiple social providers set different values, the last applied value wins.",
				Optional:    true,
			},
		},
	}
}

func (r *SocialProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SocialProviderResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config SocialProviderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Ensure provider_type is specified and known, since it is required to build the API payload.
	if config.ProviderType.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("provider_type"),
			"Missing Required Attribute",
			"provider_type must be specified.",
		)
		return
	}
	if config.ProviderType.IsUnknown() {
		// Cannot validate attribute combinations when provider_type is unknown.
		return
	}

	providerType := config.ProviderType.ValueString()

	// When any attribute value is unknown (e.g. sourced from a data source like
	// AWS Secrets Manager), we cannot validate cross-field requirements yet.
	// Unknown values will become known at apply time, so we defer validation.
	// See: https://developer.hashicorp.com/terraform/plugin/framework/validation
	if config.ClientSecret.IsUnknown() ||
		config.AppleTeamID.IsUnknown() ||
		config.ApplePrivateKeyID.IsUnknown() ||
		config.ApplePrivateKey.IsUnknown() {
		return
	}

	hasClientSecret := !config.ClientSecret.IsNull()
	hasAppleTeamID := !config.AppleTeamID.IsNull()
	hasApplePrivateKeyID := !config.ApplePrivateKeyID.IsNull()
	hasApplePrivateKey := !config.ApplePrivateKey.IsNull()
	hasAnyAppleField := hasAppleTeamID || hasApplePrivateKeyID || hasApplePrivateKey
	hasAllAppleFields := hasAppleTeamID && hasApplePrivateKeyID && hasApplePrivateKey

	// Validate that known values are not empty strings
	if hasClientSecret && config.ClientSecret.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret"), "Invalid Attribute Value", "client_secret must not be an empty string.")
	}
	if hasAppleTeamID && config.AppleTeamID.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("apple_team_id"), "Invalid Attribute Value", "apple_team_id must not be an empty string.")
	}
	if hasApplePrivateKeyID && config.ApplePrivateKeyID.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("apple_private_key_id"), "Invalid Attribute Value", "apple_private_key_id must not be an empty string.")
	}
	if hasApplePrivateKey && config.ApplePrivateKey.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("apple_private_key"), "Invalid Attribute Value", "apple_private_key must not be an empty string.")
	}
	if !config.BaseRedirectURI.IsNull() && !config.BaseRedirectURI.IsUnknown() && config.BaseRedirectURI.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("base_redirect_uri"), "Invalid Attribute Value", "base_redirect_uri must not be an empty string.")
	}

	if providerType == "apple" {
		// Apple: needs either client_secret OR all three Apple-specific fields
		if !hasClientSecret && !hasAllAppleFields {
			resp.Diagnostics.AddAttributeError(
				path.Root("client_secret"),
				"Missing Apple Provider Configuration",
				"Apple providers require either client_secret or all three Apple-specific attributes: apple_team_id, apple_private_key_id, and apple_private_key.",
			)
		}
		if hasAnyAppleField && !hasAllAppleFields {
			resp.Diagnostics.AddAttributeError(
				path.Root("apple_team_id"),
				"Incomplete Apple Provider Configuration",
				"When using Apple-specific attributes, all three must be set: apple_team_id, apple_private_key_id, and apple_private_key.",
			)
		}
	} else {
		// Non-Apple: client_secret is required
		if !hasClientSecret {
			resp.Diagnostics.AddAttributeError(
				path.Root("client_secret"),
				"Missing Required Attribute",
				fmt.Sprintf("client_secret is required for provider_type %q.", providerType),
			)
		}
		// Non-Apple: Apple-specific fields should not be set
		if hasAnyAppleField {
			resp.Diagnostics.AddAttributeError(
				path.Root("provider_type"),
				"Invalid Attribute Combination",
				"apple_team_id, apple_private_key_id, and apple_private_key are only valid when provider_type is \"apple\".",
			)
		}
	}
}

// defaultMapperURL returns the default Jsonnet mapper for common providers.
// This is a simple mapper that extracts email and subject from the claims.
// The base64-encoded Jsonnet maps claims to identity traits.
const defaultMapperURL = "base64://bG9jYWwgY2xhaW1zID0gc3RkLmV4dFZhcignY2xhaW1zJyk7CnsKICBpZGVudGl0eTogewogICAgdHJhaXRzOiB7CiAgICAgIGVtYWlsOiBjbGFpbXMuZW1haWwsCiAgICB9LAogIH0sCn0="

func (r *SocialProviderResource) buildProviderConfig(ctx context.Context, plan *SocialProviderResourceModel) map[string]interface{} {
	config := map[string]interface{}{
		"id":        plan.ProviderID.ValueString(),
		"provider":  plan.ProviderType.ValueString(),
		"client_id": plan.ClientID.ValueString(),
	}

	if !plan.ClientSecret.IsNull() && !plan.ClientSecret.IsUnknown() && plan.ClientSecret.ValueString() != "" {
		config["client_secret"] = plan.ClientSecret.ValueString()
	}
	if !plan.IssuerURL.IsNull() && !plan.IssuerURL.IsUnknown() {
		config["issuer_url"] = plan.IssuerURL.ValueString()
	}
	if !plan.Scope.IsNull() && !plan.Scope.IsUnknown() {
		var scope []string
		plan.Scope.ElementsAs(ctx, &scope, false)
		config["scope"] = scope
	}
	// mapper_url is required by the Ory API - use default if not provided
	if !plan.MapperURL.IsNull() && !plan.MapperURL.IsUnknown() && plan.MapperURL.ValueString() != "" {
		config["mapper_url"] = plan.MapperURL.ValueString()
	} else {
		config["mapper_url"] = defaultMapperURL
	}
	if !plan.AuthURL.IsNull() && !plan.AuthURL.IsUnknown() {
		config["auth_url"] = plan.AuthURL.ValueString()
	}
	if !plan.TokenURL.IsNull() && !plan.TokenURL.IsUnknown() {
		config["token_url"] = plan.TokenURL.ValueString()
	}
	if !plan.Tenant.IsNull() && !plan.Tenant.IsUnknown() {
		config["microsoft_tenant"] = plan.Tenant.ValueString()
	}

	// Auto-link — only send when explicitly set
	if !plan.AutoLink.IsNull() && !plan.AutoLink.IsUnknown() {
		config["auto_link"] = plan.AutoLink.ValueBool()
	}

	// Apple-specific fields — skip empty strings to avoid sending blank credentials
	if !plan.AppleTeamID.IsNull() && !plan.AppleTeamID.IsUnknown() && plan.AppleTeamID.ValueString() != "" {
		config["apple_team_id"] = plan.AppleTeamID.ValueString()
	}
	if !plan.ApplePrivateKeyID.IsNull() && !plan.ApplePrivateKeyID.IsUnknown() && plan.ApplePrivateKeyID.ValueString() != "" {
		config["apple_private_key_id"] = plan.ApplePrivateKeyID.ValueString()
	}
	if !plan.ApplePrivateKey.IsNull() && !plan.ApplePrivateKey.IsUnknown() && plan.ApplePrivateKey.ValueString() != "" {
		config["apple_private_key"] = plan.ApplePrivateKey.ValueString()
	}

	return config
}

// extractOIDCConfigFromProject navigates to the OIDC method config map.
func extractOIDCConfigFromProject(project *ory.Project) map[string]interface{} {
	if project.Services.Identity == nil {
		return nil
	}
	configMap := project.Services.Identity.Config
	if configMap == nil {
		return nil
	}
	selfservice, ok := configMap["selfservice"].(map[string]interface{})
	if !ok {
		return nil
	}
	methods, ok := selfservice["methods"].(map[string]interface{})
	if !ok {
		return nil
	}
	oidc, ok := methods["oidc"].(map[string]interface{})
	if !ok {
		return nil
	}
	oidcConfig, ok := oidc["config"].(map[string]interface{})
	if !ok {
		return nil
	}
	return oidcConfig
}

func extractProvidersFromProject(project *ory.Project) []map[string]interface{} {
	oidcConfig := extractOIDCConfigFromProject(project)
	if oidcConfig == nil {
		return []map[string]interface{}{}
	}
	providers, ok := oidcConfig["providers"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}
	result := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		if pm, ok := p.(map[string]interface{}); ok {
			result = append(result, pm)
		}
	}
	return result
}

func (r *SocialProviderResource) getProviders(ctx context.Context, projectID string) ([]map[string]interface{}, error) {
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}
	return extractProvidersFromProject(project), nil
}

func (r *SocialProviderResource) findProviderIndex(providers []map[string]interface{}, providerID string) int {
	for i, p := range providers {
		if p["id"] == providerID {
			return i
		}
	}
	return -1
}

func (r *SocialProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SocialProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	providerConfig := r.buildProviderConfig(ctx, &plan)

	// Get current providers
	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Providers", err.Error())
		return
	}

	var patches []ory.JsonPatch

	existingIndex := r.findProviderIndex(providers, plan.ProviderID.ValueString())
	if existingIndex >= 0 {
		// Replace existing
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  fmt.Sprintf("/services/identity/config/selfservice/methods/oidc/config/providers/%d", existingIndex),
			Value: providerConfig,
		})
		if !plan.BaseRedirectURI.IsNull() && !plan.BaseRedirectURI.IsUnknown() && plan.BaseRedirectURI.ValueString() != "" {
			patches = append(patches, ory.JsonPatch{
				Op:    "add",
				Path:  "/services/identity/config/selfservice/methods/oidc/config/base_redirect_uri",
				Value: plan.BaseRedirectURI.ValueString(),
			})
		}
	} else {
		// When adding the first provider, we need to initialize the entire OIDC config structure
		if len(providers) == 0 {
			oidcConfig := map[string]interface{}{
				"providers": []interface{}{providerConfig},
			}
			if !plan.BaseRedirectURI.IsNull() && !plan.BaseRedirectURI.IsUnknown() && plan.BaseRedirectURI.ValueString() != "" {
				oidcConfig["base_redirect_uri"] = plan.BaseRedirectURI.ValueString()
			}
			patches = append(patches, ory.JsonPatch{
				Op:   "add",
				Path: "/services/identity/config/selfservice/methods/oidc",
				Value: map[string]interface{}{
					"enabled": true,
					"config":  oidcConfig,
				},
			})
		} else {
			// Add new provider to existing list
			patches = append(patches, ory.JsonPatch{
				Op:    "add",
				Path:  "/services/identity/config/selfservice/methods/oidc/config/providers/-",
				Value: providerConfig,
			})
			// Set base_redirect_uri as a separate patch when OIDC config already exists
			if !plan.BaseRedirectURI.IsNull() && !plan.BaseRedirectURI.IsUnknown() && plan.BaseRedirectURI.ValueString() != "" {
				patches = append(patches, ory.JsonPatch{
					Op:    "add",
					Path:  "/services/identity/config/selfservice/methods/oidc/config/base_redirect_uri",
					Value: plan.BaseRedirectURI.ValueString(),
				})
			}
		}
	}

	result, err := r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Social Provider", err.Error())
		return
	}

	project := result.GetProject()
	updatedProviders := extractProvidersFromProject(&project)
	if r.findProviderIndex(updatedProviders, plan.ProviderID.ValueString()) < 0 {
		resp.Diagnostics.AddError("Error Verifying Social Provider",
			fmt.Sprintf("Provider %q was not found in the PatchProject response", plan.ProviderID.ValueString()))
		return
	}

	plan.ID = plan.ProviderID
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *SocialProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SocialProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	// Validate we have a project ID
	if projectID == "" {
		resp.Diagnostics.AddError(
			"Missing Project ID",
			"Could not determine project ID. Set project_id in the resource or configure ORY_PROJECT_ID environment variable.",
		)
		return
	}

	providerID := state.ProviderID.ValueString()
	if providerID == "" {
		resp.Diagnostics.AddError(
			"Missing Provider ID",
			"provider_id is empty in state. This is a bug - please report it.",
		)
		return
	}

	var providers []map[string]interface{}
	var index = -1
	var lastProject *ory.Project // retained so base_redirect_uri can reuse it

	if cached := r.client.GetCachedProject(projectID); cached != nil {
		lastProject = cached
		providers = extractProvidersFromProject(cached)
		index = r.findProviderIndex(providers, providerID)
	}

	if index < 0 {
		for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
			project, err := r.client.GetProject(ctx, projectID)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading Social Provider",
					fmt.Sprintf("Failed to get providers for project %s: %v", projectID, err))
				return
			}
			lastProject = project
			providers = extractProvidersFromProject(project)

			index = r.findProviderIndex(providers, providerID)
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
		// Provider not found after retries - it may have been deleted outside of Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	provider := providers[index]
	state.ProviderType = types.StringValue(fmt.Sprintf("%v", provider["provider"]))
	state.ClientID = types.StringValue(fmt.Sprintf("%v", provider["client_id"]))
	// Don't read back client_secret for security - it's sensitive

	if issuer, ok := provider["issuer_url"].(string); ok {
		state.IssuerURL = types.StringValue(issuer)
	}

	// Read scope array
	if scope, ok := provider["scope"].([]interface{}); ok && len(scope) > 0 {
		scopeStrings := make([]string, 0, len(scope))
		for _, s := range scope {
			if str, ok := s.(string); ok {
				scopeStrings = append(scopeStrings, str)
			}
		}
		scopeList, diags := types.ListValueFrom(ctx, types.StringType, scopeStrings)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			state.Scope = scopeList
		}
	}

	// Read mapper_url only if user explicitly configured it
	// When user doesn't set mapper_url, we use a default which gets transformed to a GCS URL by the API
	// We only read it back if it was already in state (user configured it)
	if !state.MapperURL.IsNull() && !state.MapperURL.IsUnknown() {
		if mapper, ok := provider["mapper_url"].(string); ok && mapper != "" {
			state.MapperURL = types.StringValue(mapper)
		}
	}

	// Read auth_url for custom providers
	if authURL, ok := provider["auth_url"].(string); ok && authURL != "" {
		state.AuthURL = types.StringValue(authURL)
	}

	// Read token_url for custom providers
	if tokenURL, ok := provider["token_url"].(string); ok && tokenURL != "" {
		state.TokenURL = types.StringValue(tokenURL)
	}

	// Read microsoft_tenant for Azure providers
	if tenant, ok := provider["microsoft_tenant"].(string); ok && tenant != "" {
		state.Tenant = types.StringValue(tenant)
	}

	// auto_link is write-only — the API does not return it in responses.
	// Preserve the existing state value (same approach as client_secret).

	// Read Apple-specific fields, clearing stale state when not returned by the API
	if teamID, ok := provider["apple_team_id"].(string); ok && teamID != "" {
		state.AppleTeamID = types.StringValue(teamID)
	} else {
		state.AppleTeamID = types.StringNull()
	}
	if keyID, ok := provider["apple_private_key_id"].(string); ok && keyID != "" {
		state.ApplePrivateKeyID = types.StringValue(keyID)
	} else {
		state.ApplePrivateKeyID = types.StringNull()
	}
	// Don't read back apple_private_key for security - it's sensitive

	// Read base_redirect_uri — a global OIDC config field, not per-provider.
	// Only track it when the user has configured it in state, to avoid cross-resource
	// conflicts from this global setting being managed by multiple providers.
	// Reuse lastProject (already fetched above) to avoid an extra API call.
	if !state.BaseRedirectURI.IsNull() && !state.BaseRedirectURI.IsUnknown() {
		if oidcConfig := extractOIDCConfigFromProject(lastProject); oidcConfig != nil {
			if uri, ok := oidcConfig["base_redirect_uri"].(string); ok && uri != "" {
				state.BaseRedirectURI = types.StringValue(uri)
			} else {
				state.BaseRedirectURI = types.StringNull()
			}
		} else {
			state.BaseRedirectURI = types.StringNull()
		}
	}

	// Always ensure ID and ProjectID are set in state
	state.ID = types.StringValue(providerID)
	state.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SocialProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SocialProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Providers", err.Error())
		return
	}

	index := r.findProviderIndex(providers, plan.ProviderID.ValueString())
	if index < 0 {
		resp.Diagnostics.AddError("Provider Not Found",
			fmt.Sprintf("Provider '%s' not found", plan.ProviderID.ValueString()))
		return
	}

	providerConfig := r.buildProviderConfig(ctx, &plan)
	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  fmt.Sprintf("/services/identity/config/selfservice/methods/oidc/config/providers/%d", index),
		Value: providerConfig,
	}}

	// Handle base_redirect_uri changes.
	// Skip patching when the planned value is unknown — Terraform hasn't resolved it yet
	// and acting on it could send a spurious remove op.
	if !plan.BaseRedirectURI.IsUnknown() {
		planURI := plan.BaseRedirectURI.ValueString()
		stateURI := ""
		if !state.BaseRedirectURI.IsUnknown() && !state.BaseRedirectURI.IsNull() {
			stateURI = state.BaseRedirectURI.ValueString()
		}
		if !plan.BaseRedirectURI.IsNull() && planURI != "" {
			// Only patch when value actually changed to avoid unnecessary global config churn.
			if planURI != stateURI {
				patches = append(patches, ory.JsonPatch{
					Op:    "add",
					Path:  "/services/identity/config/selfservice/methods/oidc/config/base_redirect_uri",
					Value: planURI,
				})
			}
		} else if !state.BaseRedirectURI.IsNull() && stateURI != "" {
			// User removed base_redirect_uri from config — remove it from the API too.
			patches = append(patches, ory.JsonPatch{
				Op:   "remove",
				Path: "/services/identity/config/selfservice/methods/oidc/config/base_redirect_uri",
			})
		}
	}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Social Provider", err.Error())
		return
	}

	plan.ID = plan.ProviderID
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *SocialProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SocialProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting Providers", err.Error())
		return
	}

	index := r.findProviderIndex(providers, state.ProviderID.ValueString())
	if index < 0 {
		return // Already deleted
	}

	var patches []ory.JsonPatch

	// If this is the last provider, we need to reset the entire OIDC config
	// to avoid leaving an invalid state with an empty providers array
	if len(providers) == 1 {
		// Reset the entire OIDC method configuration
		patches = append(patches, ory.JsonPatch{
			Op:   "replace",
			Path: "/services/identity/config/selfservice/methods/oidc",
			Value: map[string]interface{}{
				"enabled": false,
				"config": map[string]interface{}{
					"providers": []interface{}{},
				},
			},
		})
	} else {
		// Remove the specific provider by index
		patches = append(patches, ory.JsonPatch{
			Op:   "remove",
			Path: fmt.Sprintf("/services/identity/config/selfservice/methods/oidc/config/providers/%d", index),
		})
	}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Social Provider", err.Error())
		return
	}
}

func (r *SocialProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("provider_id"), req.ID)...)
}
