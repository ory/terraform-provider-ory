package socialprovider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
)

var (
	_ resource.Resource                = &SocialProviderResource{}
	_ resource.ResourceWithConfigure   = &SocialProviderResource{}
	_ resource.ResourceWithImportState = &SocialProviderResource{}
)

func NewResource() resource.Resource {
	return &SocialProviderResource{}
}

type SocialProviderResource struct {
	client *client.OryClient
}

type SocialProviderResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ProjectID    types.String `tfsdk:"project_id"`
	ProviderID   types.String `tfsdk:"provider_id"`
	ProviderType types.String `tfsdk:"provider_type"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	IssuerURL    types.String `tfsdk:"issuer_url"`
	Scope        types.List   `tfsdk:"scope"`
	MapperURL    types.String `tfsdk:"mapper_url"`
	AuthURL      types.String `tfsdk:"auth_url"`
	TokenURL     types.String `tfsdk:"token_url"`
	Tenant       types.String `tfsdk:"tenant"`
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
				Description: "OAuth2 client secret from the provider.",
				Required:    true,
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

// defaultMapperURL returns the default Jsonnet mapper for common providers.
// This is a simple mapper that extracts email and subject from the claims.
// The base64-encoded Jsonnet maps claims to identity traits.
const defaultMapperURL = "base64://bG9jYWwgY2xhaW1zID0gc3RkLmV4dFZhcignY2xhaW1zJyk7CnsKICBpZGVudGl0eTogewogICAgdHJhaXRzOiB7CiAgICAgIGVtYWlsOiBjbGFpbXMuZW1haWwsCiAgICB9LAogIH0sCn0="

func (r *SocialProviderResource) buildProviderConfig(ctx context.Context, plan *SocialProviderResourceModel) map[string]interface{} {
	config := map[string]interface{}{
		"id":            plan.ProviderID.ValueString(),
		"provider":      plan.ProviderType.ValueString(),
		"client_id":     plan.ClientID.ValueString(),
		"client_secret": plan.ClientSecret.ValueString(),
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

	return config
}

func (r *SocialProviderResource) getProviders(ctx context.Context, projectID string) ([]map[string]interface{}, error) {
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}

	if project.Services.Identity == nil {
		// No identity service configured yet - this is valid, return empty list
		return []map[string]interface{}{}, nil
	}

	configMap := project.Services.Identity.Config
	if configMap == nil {
		// No config yet - return empty list
		return []map[string]interface{}{}, nil
	}

	// Navigate through the config structure - return empty list if any level is missing
	selfservice, ok := configMap["selfservice"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	methods, ok := selfservice["methods"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	oidc, ok := methods["oidc"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	oidcConfig, ok := oidc["config"].(map[string]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	providers, ok := oidcConfig["providers"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	result := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		if pm, ok := p.(map[string]interface{}); ok {
			result = append(result, pm)
		}
	}
	return result, nil
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
	} else {
		// When adding the first provider, we need to initialize the entire OIDC config structure
		if len(providers) == 0 {
			// Initialize OIDC config with the provider in one operation
			patches = append(patches, ory.JsonPatch{
				Op:   "add",
				Path: "/services/identity/config/selfservice/methods/oidc",
				Value: map[string]interface{}{
					"enabled": true,
					"config": map[string]interface{}{
						"providers": []interface{}{providerConfig},
					},
				},
			})
		} else {
			// Add new provider to existing list
			patches = append(patches, ory.JsonPatch{
				Op:    "add",
				Path:  "/services/identity/config/selfservice/methods/oidc/config/providers/-",
				Value: providerConfig,
			})
		}
	}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Social Provider", err.Error())
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

	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Social Provider",
			fmt.Sprintf("Failed to get providers for project %s: %v", projectID, err))
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

	index := r.findProviderIndex(providers, providerID)
	if index < 0 {
		// Provider not found - it may have been deleted outside of Terraform
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

	// Always ensure ID and ProjectID are set in state
	state.ID = types.StringValue(providerID)
	state.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SocialProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SocialProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
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
