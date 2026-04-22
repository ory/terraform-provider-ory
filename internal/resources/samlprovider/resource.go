package samlprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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

var (
	_ resource.Resource                   = &SAMLProviderResource{}
	_ resource.ResourceWithConfigure      = &SAMLProviderResource{}
	_ resource.ResourceWithImportState    = &SAMLProviderResource{}
	_ resource.ResourceWithValidateConfig = &SAMLProviderResource{}
)

// projectMutexes serializes Create, Update, and Delete operations per project.
// The Ory API stores SAML providers as an array in the project config. All
// mutations use a read-modify-write pattern (read providers → build patch →
// PatchProject). Without serialization, concurrent operations read the same
// stale state and the last write wins, silently discarding the others.
// See: https://github.com/ory/terraform-provider-ory/issues/165
var projectMutexes sync.Map // map[projectID]*sync.Mutex

func projectMutex(projectID string) *sync.Mutex {
	v, _ := projectMutexes.LoadOrStore(projectID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func NewResource() resource.Resource {
	return &SAMLProviderResource{}
}

type SAMLProviderResource struct {
	client *client.OryClient
}

type SAMLProviderResourceModel struct {
	ID                        types.String `tfsdk:"id"`
	ProjectID                 types.String `tfsdk:"project_id"`
	ProviderID                types.String `tfsdk:"provider_id"`
	Label                     types.String `tfsdk:"label"`
	MapperURL                 types.String `tfsdk:"mapper_url"`
	RawIDPMetadataXML         types.String `tfsdk:"raw_idp_metadata_xml"`
	OrganizationID            types.String `tfsdk:"organization_id"`
	AudienceOverrideBaseURL   types.String `tfsdk:"audience_override_base_url"`
	ProxySAMLAudienceOverride types.String `tfsdk:"proxy_saml_audience_override"`
}

func (r *SAMLProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_saml_provider"
}

func (r *SAMLProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network SAML identity provider (Ory Polis).",
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
				Description: "Unique identifier for the SAML provider (used in ACS callback URLs).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"label": schema.StringAttribute{
				Description: "Human-readable label for the provider, displayed on the login button (e.g., \"Sign in with Corporate SSO\").",
				Optional:    true,
			},
			"mapper_url": schema.StringAttribute{
				Description: "Jsonnet mapper URL for mapping SAML attributes to identity traits. Accepts a URL (http/https) or a base64-encoded Jsonnet template prefixed with `base64://`. If not set, a default mapper that extracts email from claims will be used.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(mapperURLRegex, "must start with base64://, http://, or https://"),
				},
			},
			"raw_idp_metadata_xml": schema.StringAttribute{
				Description: "SAML IDP metadata XML. Accepts a URL (http/https) or a base64-encoded XML prefixed with `base64://`. This is required by the SAML identity provider to establish trust.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(mapperURLRegex, "must start with base64://, http://, or https://"),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "Organization ID to associate this SAML provider with (for B2B SSO).",
				Optional:    true,
			},
			"audience_override_base_url": schema.StringAttribute{
				Description: "Override the base URL used when computing the SAML SP audience (EntityID). Useful when running Ory behind a custom domain.",
				Optional:    true,
			},
			"proxy_saml_audience_override": schema.StringAttribute{
				Description: "Customer-controlled override of the SAML Audience (EntityID) sent to the identity provider.",
				Optional:    true,
			},
		},
	}
}

func (r *SAMLProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SAMLProviderResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config SAMLProviderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.ProviderID.IsNull() && !config.ProviderID.IsUnknown() && config.ProviderID.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("provider_id"), "Invalid Attribute Value", "provider_id must not be an empty string.")
	}
	if !config.RawIDPMetadataXML.IsNull() && !config.RawIDPMetadataXML.IsUnknown() && config.RawIDPMetadataXML.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("raw_idp_metadata_xml"), "Invalid Attribute Value", "raw_idp_metadata_xml must not be an empty string.")
	}
}

// defaultMapperURL returns the default Jsonnet mapper. Ory's SAML backend
// (Polis/Jackson) surfaces claims under the same `claims` variable used by
// OIDC mappers, so this matches the social_provider default.
const defaultMapperURL = "base64://bG9jYWwgY2xhaW1zID0gc3RkLmV4dFZhcignY2xhaW1zJyk7CnsKICBpZGVudGl0eTogewogICAgdHJhaXRzOiB7CiAgICAgIGVtYWlsOiBjbGFpbXMuZW1haWwsCiAgICB9LAogIH0sCn0="

func (r *SAMLProviderResource) buildProviderConfig(plan *SAMLProviderResourceModel) map[string]interface{} {
	config := map[string]interface{}{
		"id":                   plan.ProviderID.ValueString(),
		"raw_idp_metadata_xml": plan.RawIDPMetadataXML.ValueString(),
	}

	if !plan.Label.IsNull() && !plan.Label.IsUnknown() && plan.Label.ValueString() != "" {
		config["label"] = plan.Label.ValueString()
	}
	if !plan.MapperURL.IsNull() && !plan.MapperURL.IsUnknown() && plan.MapperURL.ValueString() != "" {
		config["mapper_url"] = plan.MapperURL.ValueString()
	} else {
		config["mapper_url"] = defaultMapperURL
	}
	if !plan.OrganizationID.IsNull() && !plan.OrganizationID.IsUnknown() && plan.OrganizationID.ValueString() != "" {
		config["organization_id"] = plan.OrganizationID.ValueString()
	}
	if !plan.AudienceOverrideBaseURL.IsNull() && !plan.AudienceOverrideBaseURL.IsUnknown() && plan.AudienceOverrideBaseURL.ValueString() != "" {
		config["audience_override_base_url"] = plan.AudienceOverrideBaseURL.ValueString()
	}
	if !plan.ProxySAMLAudienceOverride.IsNull() && !plan.ProxySAMLAudienceOverride.IsUnknown() && plan.ProxySAMLAudienceOverride.ValueString() != "" {
		config["proxy_saml_audience_override"] = plan.ProxySAMLAudienceOverride.ValueString()
	}

	return config
}

// extractSAMLConfigFromProject navigates to the SAML method config map.
func extractSAMLConfigFromProject(project *ory.Project) map[string]interface{} {
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
	saml, ok := methods["saml"].(map[string]interface{})
	if !ok {
		return nil
	}
	samlConfig, ok := saml["config"].(map[string]interface{})
	if !ok {
		return nil
	}
	return samlConfig
}

func extractProvidersFromProject(project *ory.Project) []map[string]interface{} {
	samlConfig := extractSAMLConfigFromProject(project)
	if samlConfig == nil {
		return []map[string]interface{}{}
	}
	providers, ok := samlConfig["providers"].([]interface{})
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

// copyProviders returns a deep copy of the provider slice via a JSON
// round-trip so callers cannot accidentally mutate the cached project state.
func copyProviders(providers []map[string]interface{}) []map[string]interface{} {
	b, err := json.Marshal(providers)
	if err != nil {
		return providers
	}
	var cp []map[string]interface{}
	if err := json.Unmarshal(b, &cp); err != nil {
		return providers
	}
	return cp
}

func (r *SAMLProviderResource) getProviders(ctx context.Context, projectID string) ([]map[string]interface{}, error) {
	if cached := r.client.GetCachedProject(projectID); cached != nil {
		return copyProviders(extractProvidersFromProject(cached)), nil
	}
	project, err := r.client.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectID, err)
	}
	return extractProvidersFromProject(project), nil
}

func (r *SAMLProviderResource) findProviderIndex(providers []map[string]interface{}, providerID string) int {
	for i, p := range providers {
		if p["id"] == providerID {
			return i
		}
	}
	return -1
}

func (r *SAMLProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SAMLProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	providerConfig := r.buildProviderConfig(&plan)

	mu := projectMutex(projectID)
	mu.Lock()
	defer mu.Unlock()

	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting SAML Providers", err.Error())
		return
	}

	var patches []ory.JsonPatch
	existingIndex := r.findProviderIndex(providers, plan.ProviderID.ValueString())

	switch {
	case existingIndex >= 0:
		// Replace existing
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  fmt.Sprintf("/services/identity/config/selfservice/methods/saml/config/providers/%d", existingIndex),
			Value: providerConfig,
		})
	case len(providers) == 0:
		// Initialize the full SAML method configuration when adding the first provider.
		// Using `add` (not `replace`) mirrors ory_social_provider's first-provider flow
		// so the JSON Patch semantics are identical whether or not the saml method stub
		// already exists in the project config.
		patches = append(patches, ory.JsonPatch{
			Op:   "add",
			Path: "/services/identity/config/selfservice/methods/saml",
			Value: map[string]interface{}{
				"enabled": true,
				"config": map[string]interface{}{
					"providers": []interface{}{providerConfig},
				},
			},
		})
	default:
		patches = append(patches, ory.JsonPatch{
			Op:    "add",
			Path:  "/services/identity/config/selfservice/methods/saml/config/providers/-",
			Value: providerConfig,
		})
	}

	result, err := r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating SAML Provider", apiErrorDetail(err))
		return
	}

	project := result.GetProject()
	updatedProviders := extractProvidersFromProject(&project)
	if r.findProviderIndex(updatedProviders, plan.ProviderID.ValueString()) < 0 {
		resp.Diagnostics.AddError("Error Verifying SAML Provider",
			fmt.Sprintf("Provider %q was not found in the PatchProject response", plan.ProviderID.ValueString()))
		return
	}

	plan.ID = plan.ProviderID
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *SAMLProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SAMLProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}
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
	index := -1

	if cached := r.client.GetCachedProject(projectID); cached != nil {
		providers = extractProvidersFromProject(cached)
		index = r.findProviderIndex(providers, providerID)
	}

	if index < 0 {
		for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
			project, err := r.client.GetProject(ctx, projectID)
			if err != nil {
				resp.Diagnostics.AddError("Error Reading SAML Provider",
					fmt.Sprintf("Failed to get providers for project %s: %v", projectID, err))
				return
			}
			providers = extractProvidersFromProject(project)
			index = r.findProviderIndex(providers, providerID)
			if index >= 0 {
				break
			}
			if attempt < helpers.ReadRetryMaxAttempts-1 {
				select {
				case <-ctx.Done():
					resp.Diagnostics.AddError(
						"Error Reading SAML Provider",
						fmt.Sprintf("read canceled while retrying project %s: %v", projectID, ctx.Err()),
					)
					return
				case <-time.After(time.Duration(1<<attempt) * time.Second):
				}
			}
		}
	}

	if index < 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	provider := providers[index]

	if label, ok := provider["label"].(string); ok && label != "" {
		state.Label = types.StringValue(label)
	} else {
		state.Label = types.StringNull()
	}

	// The API transforms base64:// mapper URLs into hosted GCS URLs on read.
	// When the user supplied base64://..., preserve their original value to avoid
	// spurious drift — only refresh state when the user originally configured a URL.
	if !state.MapperURL.IsNull() && !state.MapperURL.IsUnknown() {
		userValue := state.MapperURL.ValueString()
		if !strings.HasPrefix(userValue, "base64://") {
			if mapper, ok := provider["mapper_url"].(string); ok && mapper != "" {
				state.MapperURL = types.StringValue(mapper)
			}
		}
	}

	// raw_idp_metadata_xml also gets transformed to a GCS URL on read.
	// When the user supplied base64://..., keep their original value to avoid
	// spurious diffs. When the state is null/unknown (e.g., during import) or
	// was originally a URL, refresh from the API so a required attribute is
	// always populated.
	userSuppliedBase64 := !state.RawIDPMetadataXML.IsNull() && !state.RawIDPMetadataXML.IsUnknown() &&
		strings.HasPrefix(state.RawIDPMetadataXML.ValueString(), "base64://")
	if !userSuppliedBase64 {
		if xml, ok := provider["raw_idp_metadata_xml"].(string); ok && xml != "" {
			state.RawIDPMetadataXML = types.StringValue(xml)
		}
	}

	if orgID, ok := provider["organization_id"].(string); ok && orgID != "" {
		state.OrganizationID = types.StringValue(orgID)
	} else {
		state.OrganizationID = types.StringNull()
	}

	if audience, ok := provider["audience_override_base_url"].(string); ok && audience != "" {
		state.AudienceOverrideBaseURL = types.StringValue(audience)
	} else {
		state.AudienceOverrideBaseURL = types.StringNull()
	}

	if override, ok := provider["proxy_saml_audience_override"].(string); ok && override != "" {
		state.ProxySAMLAudienceOverride = types.StringValue(override)
	} else {
		state.ProxySAMLAudienceOverride = types.StringNull()
	}

	state.ID = types.StringValue(providerID)
	state.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SAMLProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SAMLProviderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	providerConfig := r.buildProviderConfig(&plan)

	mu := projectMutex(projectID)
	mu.Lock()
	defer mu.Unlock()

	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting SAML Providers", err.Error())
		return
	}

	index := r.findProviderIndex(providers, plan.ProviderID.ValueString())
	if index < 0 {
		resp.Diagnostics.AddError("SAML Provider Not Found",
			fmt.Sprintf("Provider '%s' not found", plan.ProviderID.ValueString()))
		return
	}

	patches := []ory.JsonPatch{{
		Op:    "replace",
		Path:  fmt.Sprintf("/services/identity/config/selfservice/methods/saml/config/providers/%d", index),
		Value: providerConfig,
	}}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating SAML Provider", apiErrorDetail(err))
		return
	}

	plan.ID = plan.ProviderID
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *SAMLProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SAMLProviderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	if projectID == "" {
		projectID = r.client.ProjectID()
	}

	mu := projectMutex(projectID)
	mu.Lock()
	defer mu.Unlock()

	providers, err := r.getProviders(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error Getting SAML Providers", err.Error())
		return
	}

	index := r.findProviderIndex(providers, state.ProviderID.ValueString())
	if index < 0 {
		return
	}

	var patches []ory.JsonPatch

	// When removing the last provider, reset the SAML method config so we
	// don't leave an `enabled: true` method with an empty providers array,
	// which Ory treats as an invalid configuration.
	if len(providers) == 1 {
		patches = append(patches, ory.JsonPatch{
			Op:   "replace",
			Path: "/services/identity/config/selfservice/methods/saml",
			Value: map[string]interface{}{
				"enabled": false,
				"config": map[string]interface{}{
					"providers": []interface{}{},
				},
			},
		})
	} else {
		patches = append(patches, ory.JsonPatch{
			Op:   "remove",
			Path: fmt.Sprintf("/services/identity/config/selfservice/methods/saml/config/providers/%d", index),
		})
	}

	_, err = r.client.PatchProject(ctx, projectID, patches)
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting SAML Provider", apiErrorDetail(err))
		return
	}
}

func (r *SAMLProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("provider_id"), req.ID)...)
}
