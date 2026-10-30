package projectconfig

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func readProjectConfigWithCORS(t *testing.T, body string) *resource.ReadResponse {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	r := projectConfigResourceForServer(t, srv.URL)
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := projectConfigState(t, schemaResp.Schema)
	require.False(t, state.SetAttribute(ctx, path.Root("cors_enabled"), types.BoolValue(true)).HasError())
	resp := &resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)
	return resp
}

const liveProjectNoCORS = `{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"running","services":{}}`
const liveProjectCORSDisabled = `{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"running","services":{},"cors_public":{"enabled":false,"origins":[]}}`

func TestRead_AbsentCORSBlockErrors(t *testing.T) {
	resp := readProjectConfigWithCORS(t, liveProjectNoCORS)

	require.True(t, resp.Diagnostics.HasError(), "an absent cors_public block must fail the read")
	assert.Equal(t, "Project Config Read Incomplete", resp.Diagnostics.Errors()[0].Summary())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "omits cors_public,")

	var held types.Bool
	require.False(t, resp.State.GetAttribute(context.Background(), path.Root("cors_enabled"), &held).HasError())
	assert.True(t, held.ValueBool(), "the prior state value must stay as it was; the error is the signal")
}

func TestRead_PresentCORSBlockRefreshes(t *testing.T) {
	resp := readProjectConfigWithCORS(t, liveProjectCORSDisabled)

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
				r := projectConfigResourceForServer(t, srv.URL)
				schemaResp := &resource.SchemaResponse{}
				r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
				require.False(t, schemaResp.Diagnostics.HasError())
				state := projectConfigState(t, schemaResp.Schema)
				if tc.tracked {
					require.False(t, state.SetAttribute(ctx, path.Root(service.attribute), service.value).HasError())
				}
				resp := &resource.ReadResponse{State: state}
				r.Read(ctx, resource.ReadRequest{State: state}, resp)
				require.Equal(t, tc.wantError, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
				if tc.wantError {
					require.Len(t, resp.Diagnostics.Errors(), 1)
					assert.Equal(t, "Project Config Read Incomplete", resp.Diagnostics.Errors()[0].Summary())
					assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "services."+service.name)
					assert.True(t, state.Raw.Equal(resp.State.Raw), "incomplete reads must preserve all prior state")
				} else {
					var values map[string]tftypes.Value
					require.NoError(t, resp.State.Raw.As(&values))
					assert.True(t, values[service.attribute].IsNull())
				}
			})
		}
	}
}
