package projectconfig

import ory "github.com/ory/client-go"

func missingResponseBlocks(project *ory.Project, state *ProjectConfigResourceModel) []string {
	blocks := []string{}
	if project.CorsPublic == nil {
		if !state.CorsEnabled.IsNull() || !state.CorsOrigins.IsNull() {
			blocks = append(blocks, "cors_public")
		}
	} else if project.CorsPublic.Enabled == nil && !state.CorsEnabled.IsNull() {
		blocks = append(blocks, "cors_public.enabled")
	}
	if project.CorsAdmin == nil {
		if !state.CorsAdminEnabled.IsNull() || !state.CorsAdminOrigins.IsNull() {
			blocks = append(blocks, "cors_admin")
		}
	} else if project.CorsAdmin.Enabled == nil && !state.CorsAdminEnabled.IsNull() {
		blocks = append(blocks, "cors_admin.enabled")
	}

	services := serviceStateFields(state)
	for _, e := range accountExperienceImageFields(state) {
		if !e.field.IsNull() {
			services["account_experience"] = true
			break
		}
	}
	if project.Services.AccountExperience == nil && services["account_experience"] {
		blocks = append(blocks, "services.account_experience")
	}
	if project.Services.Identity == nil && (services["identity"] || identityCustomStateFields(state)) {
		blocks = append(blocks, "services.identity")
	}
	if project.Services.Oauth2 == nil && (services["oauth2"] || !state.OAuth2TokenHookAuth.IsNull()) {
		blocks = append(blocks, "services.oauth2")
	}
	if project.Services.Permission == nil && (services["permission"] || !state.KetoNamespaces.IsNull()) {
		blocks = append(blocks, "services.permission")
	}
	return blocks
}

func identityCustomStateFields(state *ProjectConfigResourceModel) bool {
	if !state.DefaultReturnURL.IsNull() || !state.AllowedReturnURLs.IsNull() ||
		!state.SessionTokenizerTemplates.IsNull() || !state.CourierHTTPRequestConfig.IsNull() ||
		!state.CourierChannels.IsNull() {
		return true
	}
	for _, e := range hookEntries(state) {
		if !e.Field.IsNull() {
			return true
		}
	}
	return false
}
