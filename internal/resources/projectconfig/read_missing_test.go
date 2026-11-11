package projectconfig

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		{"identity absent, int64 field", &ory.Project{}, &ProjectConfigResourceModel{SelfserviceMethodsPasswordConfigMinPasswordLength: types.Int64Value(8)}, []string{"services.identity"}},
		{"oauth2 absent, string field", &ory.Project{}, &ProjectConfigResourceModel{OAuth2TTLAccessToken: types.StringValue("1h")}, []string{"services.oauth2"}},
		{"permission absent, bool field", &ory.Project{}, &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(true)}, []string{"services.permission"}},
		{"identity absent, list field", &ory.Project{}, &ProjectConfigResourceModel{WebAuthnRPOrigins: types.ListValueMust(types.StringType, nil)}, []string{"services.identity"}},
		{"identity absent, map field", &ory.Project{}, &ProjectConfigResourceModel{SMTPHeaders: types.MapValueMust(types.StringType, nil)}, []string{"services.identity"}},
		{"identity absent, allowed return URLs", &ory.Project{}, &ProjectConfigResourceModel{AllowedReturnURLs: types.ListValueMust(types.StringType, nil)}, []string{"services.identity"}},
		{"identity absent, tokenizer templates", &ory.Project{}, &ProjectConfigResourceModel{SessionTokenizerTemplates: types.MapUnknown(types.ObjectType{AttrTypes: tokenizerTemplateAttrTypes})}, []string{"services.identity"}},
		{"identity absent, session whoami rules", &ory.Project{}, &ProjectConfigResourceModel{SessionWhoamiRules: types.ListUnknown(types.ObjectType{AttrTypes: sessionWhoamiRuleAttrTypes})}, []string{"services.identity"}},
		{"identity absent, courier HTTP config", &ory.Project{}, &ProjectConfigResourceModel{CourierHTTPRequestConfig: types.ObjectUnknown(courierHTTPRequestConfigAttrTypes)}, []string{"services.identity"}},
		{"identity absent, courier channels", &ory.Project{}, &ProjectConfigResourceModel{CourierChannels: types.ListUnknown(types.ObjectType{AttrTypes: courierChannelAttrTypes})}, []string{"services.identity"}},
		{"cors_admin absent, origins held", &ory.Project{}, &ProjectConfigResourceModel{CorsAdminOrigins: types.ListValueMust(types.StringType, nil)}, []string{"cors_admin"}},
		{"cors_admin.enabled absent, nothing held", &ory.Project{CorsAdmin: &ory.ProjectCors{}}, &ProjectConfigResourceModel{}, []string{}},
		{"cors_public.enabled absent, nothing held", &ory.Project{CorsPublic: &ory.ProjectCors{}}, &ProjectConfigResourceModel{}, []string{}},
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

const liveProjectNoCORS = `{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"running","services":{}}`
const liveProjectCORSDisabled = `{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"running","services":{},"cors_public":{"enabled":false,"origins":[]}}`

func TestRead_AbsentCORSBlockErrors(t *testing.T) {
	resp := readProjectConfig(t, jsonServer(t, http.StatusOK, liveProjectNoCORS), map[string]attr.Value{"cors_enabled": types.BoolValue(true)})

	require.True(t, resp.Diagnostics.HasError(), "an absent cors_public block must fail the read")
	assert.Equal(t, "Project Config Read Incomplete", resp.Diagnostics.Errors()[0].Summary())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "omits cors_public,")

	var held types.Bool
	require.False(t, resp.State.GetAttribute(context.Background(), path.Root("cors_enabled"), &held).HasError())
	assert.True(t, held.ValueBool(), "the prior state value must stay as it was; the error is the signal")
}

func TestRead_PresentCORSBlockRefreshes(t *testing.T) {
	resp := readProjectConfig(t, jsonServer(t, http.StatusOK, liveProjectCORSDisabled), map[string]attr.Value{"cors_enabled": types.BoolValue(true)})

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	var got types.Bool
	require.False(t, resp.State.GetAttribute(context.Background(), path.Root("cors_enabled"), &got).HasError())
	assert.False(t, got.ValueBool(), "a present block must refresh state from the response")
}

func TestRead_ServiceConfigNull(t *testing.T) {
	ctx := context.Background()
	for _, service := range []struct {
		name, attribute string
		value           attr.Value
	}{
		{"account_experience", "account_experience_contact_url", types.StringValue("https://example.com/contact")},
		{"identity", "session_lifespan", types.StringValue("20m0s")},
		{"oauth2", "oauth2_ttl_access_token", types.StringValue("1h")},
		{"permission", "keto_feature_flags_strict_mode", types.BoolValue(true)},
	} {
		for _, tc := range []struct {
			name, config string
			tracked      bool
			wantError    bool
		}{
			{"null tracked config", "null", true, true},
			{"null untracked config", "null", false, false},
			{"empty tracked config", "{}", true, false},
		} {
			t.Run(service.name+"/"+tc.name, func(t *testing.T) {
				body := strings.Replace(liveProjectNoCORS, `"services":{}`,
					fmt.Sprintf(`"services":{"%s":{"config":%s}}`, service.name, tc.config), 1)
				srv := jsonServer(t, http.StatusOK, body)
				held := map[string]attr.Value{}
				if tc.tracked {
					held[service.attribute] = service.value
				}
				resp := readProjectConfig(t, srv, held)
				require.Equal(t, tc.wantError, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
				if tc.wantError {
					require.Len(t, resp.Diagnostics.Errors(), 1)
					assert.Equal(t, "Project Config Read Incomplete", resp.Diagnostics.Errors()[0].Summary())
					assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "services."+service.name)
					prior := projectConfigState(t, resp.State.Schema.(schema.Schema))
					require.False(t, prior.SetAttribute(ctx, path.Root(service.attribute), service.value).HasError())
					assert.True(t, prior.Raw.Equal(resp.State.Raw), "incomplete reads must preserve all prior state")
				} else if tc.tracked {
					var values map[string]tftypes.Value
					require.NoError(t, resp.State.Raw.As(&values))
					assert.True(t, values[service.attribute].IsNull())
				}
			})
		}
	}
}

func TestRead_IncompleteResponseGuidance(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, cors string
		held       map[string]attr.Value
		blocks     []string
	}{
		{"public absent", "", map[string]attr.Value{"cors_enabled": types.BoolValue(true)}, []string{"cors_public"}},
		{"public enabled absent", `,"cors_public":{}`, map[string]attr.Value{"cors_enabled": types.BoolValue(true)}, []string{"cors_public.enabled"}},
		{"admin absent", "", map[string]attr.Value{"cors_admin_enabled": types.BoolValue(true)}, []string{"cors_admin"}},
		{"admin enabled absent", `,"cors_admin":{}`, map[string]attr.Value{"cors_admin_enabled": types.BoolValue(true)}, []string{"cors_admin.enabled"}},
		{"permission absent", "", map[string]attr.Value{"keto_feature_flags_strict_mode": types.BoolValue(true)}, []string{"services.permission"}},
		{"multiple blocks", "", map[string]attr.Value{"cors_enabled": types.BoolValue(true), "cors_admin_enabled": types.BoolValue(true), "keto_feature_flags_strict_mode": types.BoolValue(true)}, []string{"cors_public", "cors_admin", "services.permission"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.TrimSuffix(liveProjectNoCORS, "}") + tc.cors + "}"
			srv := jsonServer(t, http.StatusOK, body)
			resp := readProjectConfig(t, srv, tc.held)
			require.Len(t, resp.Diagnostics.Errors(), len(tc.blocks))
			prior := projectConfigState(t, resp.State.Schema.(schema.Schema))
			for name, value := range tc.held {
				require.False(t, prior.SetAttribute(ctx, path.Root(name), value).HasError())
			}
			assert.True(t, prior.Raw.Equal(resp.State.Raw))
			for i, block := range tc.blocks {
				diagnostic := resp.Diagnostics.Errors()[i]
				assert.Equal(t, "Project Config Read Incomplete", diagnostic.Summary())
				detail := diagnostic.Detail()
				assert.Contains(t, detail, "project proj-1 omits "+block+",")
				assert.Contains(t, detail, "Retry the command.")
				assert.Contains(t, detail, "report the project ID to Ory support")
				assert.Contains(t, detail, "terraform plan -refresh=false")
				assert.Contains(t, detail, "This skips checking remote changes.")
				assert.Equal(t, block == "cors_public" || block == "cors_public.enabled", strings.Contains(detail, "cors_enabled"))
			}
		})
	}
}
