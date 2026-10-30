package projectconfig

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
)

func TestServiceStateFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state *ProjectConfigResourceModel
		want  map[string]bool
	}{
		{"empty", &ProjectConfigResourceModel{}, map[string]bool{"account_experience": false, "identity": false, "oauth2": false, "permission": false}},
		{"identity string", &ProjectConfigResourceModel{SessionLifespan: types.StringValue("20m0s")}, map[string]bool{"account_experience": false, "identity": true, "oauth2": false, "permission": false}},
		{"identity unknown counts", &ProjectConfigResourceModel{SessionLifespan: types.StringUnknown()}, map[string]bool{"account_experience": false, "identity": true, "oauth2": false, "permission": false}},
		{"identity int64", &ProjectConfigResourceModel{SelfserviceMethodsPasswordConfigMinPasswordLength: types.Int64Value(8)}, map[string]bool{"account_experience": false, "identity": true, "oauth2": false, "permission": false}},
		{"oauth2", &ProjectConfigResourceModel{OAuth2TTLAccessToken: types.StringValue("1h")}, map[string]bool{"account_experience": false, "identity": false, "oauth2": true, "permission": false}},
		{"account experience", &ProjectConfigResourceModel{AccountExperienceLocale: types.StringValue("en")}, map[string]bool{"account_experience": true, "identity": false, "oauth2": false, "permission": false}},
		{"permission bool", &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(true)}, map[string]bool{"account_experience": false, "identity": false, "oauth2": false, "permission": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, serviceStateFields(tc.state))
		})
	}
}

func identityOnlyProject() *ory.Project {
	return &ory.Project{Services: ory.ProjectServices{Identity: &ory.ProjectServiceIdentity{Config: map[string]interface{}{}}}}
}

func TestMissingResponseBlocks(t *testing.T) {
	enabled := false
	for _, tc := range []struct {
		name    string
		project *ory.Project
		state   *ProjectConfigResourceModel
		want    []string
	}{
		{"nothing in state, nothing missing", &ory.Project{}, &ProjectConfigResourceModel{}, []string{}},
		{"cors_public absent", &ory.Project{}, &ProjectConfigResourceModel{CorsEnabled: types.BoolValue(false)}, []string{"cors_public"}},
		{"cors_public absent, origins held", &ory.Project{}, &ProjectConfigResourceModel{CorsOrigins: types.ListValueMust(types.StringType, nil)}, []string{"cors_public"}},
		{"cors_public.enabled absent", &ory.Project{CorsPublic: &ory.ProjectCors{}}, &ProjectConfigResourceModel{CorsEnabled: types.BoolValue(false)}, []string{"cors_public.enabled"}},
		{"cors_public complete", &ory.Project{CorsPublic: &ory.ProjectCors{Enabled: &enabled}}, &ProjectConfigResourceModel{CorsEnabled: types.BoolValue(false)}, []string{}},
		{"cors_admin absent", &ory.Project{}, &ProjectConfigResourceModel{CorsAdminEnabled: types.BoolValue(true)}, []string{"cors_admin"}},
		{"cors_admin.enabled absent", &ory.Project{CorsAdmin: &ory.ProjectCors{}}, &ProjectConfigResourceModel{CorsAdminEnabled: types.BoolValue(true)}, []string{"cors_admin.enabled"}},
		{"account experience absent, generated field", &ory.Project{}, &ProjectConfigResourceModel{AccountExperienceLocale: types.StringValue("en")}, []string{"services.account_experience"}},
		{"account experience absent, dark favicon only", &ory.Project{}, &ProjectConfigResourceModel{AccountExperienceFaviconDark: types.StringValue("data:image/png;base64,YQ==")}, []string{"services.account_experience"}},
		{"account experience absent, light favicon only", &ory.Project{}, &ProjectConfigResourceModel{AccountExperienceFaviconLight: types.StringValue("data:image/png;base64,YQ==")}, []string{"services.account_experience"}},
		{"account experience absent, dark logo only", &ory.Project{}, &ProjectConfigResourceModel{AccountExperienceLogoDark: types.StringValue("data:image/png;base64,YQ==")}, []string{"services.account_experience"}},
		{"account experience absent, light logo only", &ory.Project{}, &ProjectConfigResourceModel{AccountExperienceLogoLight: types.StringValue("data:image/png;base64,YQ==")}, []string{"services.account_experience"}},
		{"account experience absent, unknown image", &ory.Project{}, &ProjectConfigResourceModel{AccountExperienceLogoLight: types.StringUnknown()}, []string{"services.account_experience"}},
		{"account experience present, image held", &ory.Project{Services: ory.ProjectServices{AccountExperience: &ory.ProjectServiceAccountExperience{Config: map[string]interface{}{}}}}, &ProjectConfigResourceModel{AccountExperienceLogoLight: types.StringValue("data:image/png;base64,YQ==")}, []string{}},
		{"identity absent, generated field", &ory.Project{}, &ProjectConfigResourceModel{SessionLifespan: types.StringValue("20m0s")}, []string{"services.identity"}},
		{"identity absent, unknown field", &ory.Project{}, &ProjectConfigResourceModel{SessionLifespan: types.StringUnknown()}, []string{"services.identity"}},
		{"identity absent, hook toggle", &ory.Project{}, &ProjectConfigResourceModel{SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true)}, []string{"services.identity"}},
		{"identity absent, custom field", &ory.Project{}, &ProjectConfigResourceModel{DefaultReturnURL: types.StringValue("https://x")}, []string{"services.identity"}},
		{"identity present", identityOnlyProject(), &ProjectConfigResourceModel{SessionLifespan: types.StringValue("20m0s")}, []string{}},
		{"oauth2 absent, custom field", &ory.Project{}, &ProjectConfigResourceModel{OAuth2TokenHookAuth: types.ObjectUnknown(oauth2TokenHookAuthAttrTypes)}, []string{"services.oauth2"}},
		{"permission absent, keto namespaces", &ory.Project{}, &ProjectConfigResourceModel{KetoNamespaces: types.ListValueMust(types.StringType, nil)}, []string{"services.permission"}},
		{"several, fixed order", &ory.Project{}, &ProjectConfigResourceModel{KetoNamespaces: types.ListValueMust(types.StringType, nil), SessionLifespan: types.StringValue("20m0s"), CorsEnabled: types.BoolValue(false)}, []string{"cors_public", "services.identity", "services.permission"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, missingResponseBlocks(tc.project, tc.state))
		})
	}
}
