package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findPatch(patches []ory.JsonPatch, path string) *ory.JsonPatch {
	for i := range patches {
		if patches[i].Path == path {
			return &patches[i]
		}
	}
	return nil
}

func TestBuildPatches_EmptyDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringValue(""),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	require.NotNil(t, p, "expected a patch for default_browser_return_url")
	assert.Equal(t, "remove", p.Op, "expected op 'remove' for empty default_return_url")
}

func TestBuildPatches_NonEmptyDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringValue("https://app.example.com"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	require.NotNil(t, p, "expected a patch for default_browser_return_url")
	assert.Equal(t, "replace", p.Op, "expected op 'replace' for non-empty default_return_url")
}

func TestBuildPatches_NullDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	assert.Nil(t, p, "expected no patch for null default_return_url")
}

func TestBuildPatches_EmptyAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	emptyList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{})
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: emptyList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	require.NotNil(t, p, "expected a patch for allowed_return_urls")
	assert.Equal(t, "remove", p.Op, "expected op 'remove' for empty allowed_return_urls")
}

func TestBuildPatches_NonEmptyAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	urlList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"https://app.example.com"})
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: urlList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	require.NotNil(t, p, "expected a patch for allowed_return_urls")
	assert.Equal(t, "replace", p.Op, "expected op 'replace' for non-empty allowed_return_urls")
}

func TestBuildPatches_NullAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: types.ListNull(types.StringType),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	assert.Nil(t, p, "expected no patch for null allowed_return_urls")
}

func TestBuildPatches_OAuth2IssuerURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2IssuerURL: types.StringValue("https://auth.example.com"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/urls/self/issuer")
	require.NotNil(t, p, "expected a patch for oauth2_issuer_url")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "https://auth.example.com", p.Value)
}

func TestBuildPatches_OAuth2CookiesSameSiteMode(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2CookiesSameSiteMode: types.StringValue("Strict"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/serve/cookies/same_site_mode")
	require.NotNil(t, p, "expected a patch for oauth2_cookies_same_site_mode")
	assert.Equal(t, "Strict", p.Value)
}

func TestBuildPatches_OAuth2CookiesSameSiteLegacyWorkaround(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2CookiesSameSiteLegacyWorkaround: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/serve/cookies/same_site_legacy_workaround")
	require.NotNil(t, p, "expected a patch for oauth2_cookies_same_site_legacy_workaround")
	assert.Equal(t, true, p.Value)
}

func TestBuildPatches_OAuth2TokenHookAuth_APIKeyHeader(t *testing.T) {
	r := &ProjectConfigResource{}
	auth, diags := types.ObjectValue(oauth2TokenHookAuthAttrTypes, map[string]attr.Value{
		"type":  types.StringValue("api_key"),
		"name":  types.StringValue("X-Api-Key"),
		"value": types.StringValue("secret-value"),
		"in":    types.StringValue("header"),
	})
	require.False(t, diags.HasError(), "failed to build auth object: %s", diags)
	plan := &ProjectConfigResourceModel{
		OAuth2TokenHookAuth: auth,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/oauth2/token_hook/auth")
	require.NotNil(t, p, "expected a patch for oauth2_token_hook_auth")
	assert.Equal(t, "replace", p.Op)

	got, ok := p.Value.(map[string]interface{})
	require.True(t, ok, "expected map value, got %T", p.Value)
	assert.Equal(t, "api_key", got["type"])

	cfg, ok := got["config"].(map[string]interface{})
	require.True(t, ok, "expected config map, got %T", got["config"])
	assert.Equal(t, "X-Api-Key", cfg["name"])
	assert.Equal(t, "secret-value", cfg["value"])
	assert.Equal(t, "header", cfg["in"])
}

func TestBuildPatches_OAuth2TokenHookAuth_APIKeyCookie(t *testing.T) {
	r := &ProjectConfigResource{}
	auth, diags := types.ObjectValue(oauth2TokenHookAuthAttrTypes, map[string]attr.Value{
		"type":  types.StringValue("api_key"),
		"name":  types.StringValue("session_cookie"),
		"value": types.StringValue("cookie-value"),
		"in":    types.StringValue("cookie"),
	})
	require.False(t, diags.HasError(), "failed to build auth object: %s", diags)
	plan := &ProjectConfigResourceModel{
		OAuth2TokenHookAuth: auth,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/oauth2/token_hook/auth")
	require.NotNil(t, p, "expected a patch for oauth2_token_hook_auth")

	value, ok := p.Value.(map[string]interface{})
	require.True(t, ok, "expected map value, got %T", p.Value)
	cfg, ok := value["config"].(map[string]interface{})
	require.True(t, ok, "expected config map, got %T", value["config"])
	assert.Equal(t, "cookie", cfg["in"])
}

func TestRemoveURLOnlyTokenHookPatch(t *testing.T) {
	input := []ory.JsonPatch{
		{Op: "replace", Path: "/services/oauth2/config/oauth2/token_hook/url", Value: "https://example.com"},
		{Op: "replace", Path: "/services/oauth2/config/oauth2/token_hook/auth", Value: map[string]interface{}{}},
		{Op: "replace", Path: "/services/oauth2/config/urls/self/issuer", Value: "https://auth.example.com"},
	}
	got := removeURLOnlyTokenHookPatch(input)
	require.Len(t, got, 2, "expected 2 patches after filter")
	for _, p := range got {
		assert.NotEqual(t, "/services/oauth2/config/oauth2/token_hook/url", p.Path,
			"expected token_hook/url patch to be removed")
	}
}

func TestBuildPatches_NullOAuth2TokenHookAuth(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2TokenHookAuth: types.ObjectNull(oauth2TokenHookAuthAttrTypes),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/oauth2/token_hook/auth")
	assert.Nil(t, p, "expected no patch for null oauth2_token_hook_auth")
}

func TestBuildPatches_NullOAuth2IssuerURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2IssuerURL: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/urls/self/issuer")
	assert.Nil(t, p, "expected no patch for null oauth2_issuer_url")
}

func TestBuildPatches_LoginStyle(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		LoginStyle: types.StringValue("identifier_first"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/style")
	require.NotNil(t, p, "expected a patch for login_style")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "identifier_first", p.Value)
}

func TestBuildPatches_LoginStyleUnified(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		LoginStyle: types.StringValue("unified"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/style")
	require.NotNil(t, p, "expected a patch for login_style")
	assert.Equal(t, "unified", p.Value)
}

func TestBuildPatches_NullLoginStyle(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		LoginStyle: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/style")
	assert.Nil(t, p, "expected no patch for null login_style")
}

func TestBuildPatches_EnableProfile(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		EnableProfile: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/profile/enabled")
	require.NotNil(t, p, "expected a patch for enable_profile")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, true, p.Value)
}

func TestBuildPatches_NullEnableProfile(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		EnableProfile: types.BoolNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/profile/enabled")
	assert.Nil(t, p, "expected no patch for null enable_profile")
}

func TestBuildPatches_CodeLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeLifespan: types.StringValue("15m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/lifespan")
	require.NotNil(t, p, "expected a patch for code_lifespan")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "15m0s", p.Value)
}

func TestBuildPatches_NullCodeLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeLifespan: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/lifespan")
	assert.Nil(t, p, "expected no patch for null code_lifespan")
}

func TestBuildPatches_CodeMissingCredentialFallbackEnabled(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeMissingCredentialFallbackEnabled: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/missing_credential_fallback_enabled")
	require.NotNil(t, p, "expected a patch for code_missing_credential_fallback_enabled")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, true, p.Value)
}

func TestBuildPatches_NullCodeMissingCredentialFallbackEnabled(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeMissingCredentialFallbackEnabled: types.BoolNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/missing_credential_fallback_enabled")
	assert.Nil(t, p, "expected no patch for null code_missing_credential_fallback_enabled")
}

func TestBuildPatches_SessionEarliestPossibleExtend(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SessionEarliestPossibleExtend: types.StringValue("24h"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/session/earliest_possible_extend")
	require.NotNil(t, p, "expected a patch for session_earliest_possible_extend")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "24h", p.Value)
}

func TestBuildPatches_NullSessionEarliestPossibleExtend(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SessionEarliestPossibleExtend: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/session/earliest_possible_extend")
	assert.Nil(t, p, "expected no patch for null session_earliest_possible_extend")
}

func TestBuildPatches_SettingsLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsLifespan: types.StringValue("30m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/lifespan")
	require.NotNil(t, p, "expected a patch for settings_lifespan")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "30m0s", p.Value)
}

func TestBuildPatches_NullSettingsLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsLifespan: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/lifespan")
	assert.Nil(t, p, "expected no patch for null settings_lifespan")
}

func TestBuildPatches_SettingsPrivilegedSessionMaxAge(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsPrivilegedSessionMaxAge: types.StringValue("15m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/privileged_session_max_age")
	require.NotNil(t, p, "expected a patch for settings_privileged_session_max_age")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "15m0s", p.Value)
}

func TestBuildPatches_NullSettingsPrivilegedSessionMaxAge(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsPrivilegedSessionMaxAge: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/privileged_session_max_age")
	assert.Nil(t, p, "expected no patch for null settings_privileged_session_max_age")
}

func TestBuildPatches_VerificationUse(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationUse: types.StringValue("code"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/use")
	require.NotNil(t, p, "expected a patch for verification_use")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "code", p.Value)
}

func TestBuildPatches_NullVerificationUse(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationUse: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/use")
	assert.Nil(t, p, "expected no patch for null verification_use")
}

func TestBuildPatches_VerificationLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationLifespan: types.StringValue("30m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/lifespan")
	require.NotNil(t, p, "expected a patch for verification_lifespan")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "30m0s", p.Value)
}

func TestBuildPatches_NullVerificationLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationLifespan: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/lifespan")
	assert.Nil(t, p, "expected no patch for null verification_lifespan")
}

func TestBuildPatches_VerificationNotifyUnknownRecipients(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationNotifyUnknownRecipients: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/notify_unknown_recipients")
	require.NotNil(t, p, "expected a patch for verification_notify_unknown_recipients")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, true, p.Value)
}

func TestBuildPatches_NullVerificationNotifyUnknownRecipients(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationNotifyUnknownRecipients: types.BoolNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/notify_unknown_recipients")
	assert.Nil(t, p, "expected no patch for null verification_notify_unknown_recipients")
}

// projectWithIdentityConfig wraps an identity config map in the *ory.Project
// shape that buildHookPatches expects.
func projectWithIdentityConfig(identityConfig map[string]interface{}) *ory.Project {
	return &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{
				Config: identityConfig,
			},
		},
	}
}

func TestBuildShowVerificationUIHookPatches_AddPreservesOtherHooks(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"registration": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	p := patches[0]
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "/services/identity/config/selfservice/flows/registration/after/password/hooks", p.Path)
	hooks, ok := p.Value.([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} value, got %T", p.Value)
	require.Len(t, hooks, 2, "expected 2 hooks (show_verification_ui + organization)")
	assert.Equal(t, "show_verification_ui", hooks[0]["hook"], "expected show_verification_ui first")
	assert.Equal(t, "organization", hooks[1]["hook"], "expected organization preserved")
}

func TestBuildShowVerificationUIHookPatches_RemoveKeepsOthers(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookShowVerificationUI: types.BoolValue(false),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"settings": map[string]interface{}{
					"after": map[string]interface{}{
						"profile": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "show_verification_ui"},
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, ok := patches[0].Value.([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} value, got %T", patches[0].Value)
	require.Len(t, hooks, 1, "expected 1 remaining hook")
	assert.Equal(t, "organization", hooks[0]["hook"], "expected organization to remain")
}

func TestBuildShowVerificationUIHookPatches_AddIdempotent(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterOIDCHookShowVerificationUI: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"registration": map[string]interface{}{
					"after": map[string]interface{}{
						"oidc": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "show_verification_ui"},
								map[string]interface{}{"hook": "session"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 2, "expected 2 hooks (no duplicate): %v", hooks)
	seen := map[string]int{}
	for _, h := range hooks {
		if name, ok := h["hook"].(string); ok {
			seen[name]++
		}
	}
	assert.Equal(t, 1, seen["show_verification_ui"], "expected exactly one show_verification_ui")
	assert.Equal(t, 1, seen["session"], "expected session preserved")
}

func TestBuildShowVerificationUIHookPatches_NullSkipped(t *testing.T) {
	plan := &ProjectConfigResourceModel{}
	current := projectWithIdentityConfig(map[string]interface{}{})

	patches := buildHookPatches(plan, current)
	assert.Empty(t, patches, "expected no patches when all hook attrs are null")
}

func TestBuildShowVerificationUIHookPatches_NilCurrentProject(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
	}
	patches := buildHookPatches(plan, nil)
	assert.Empty(t, patches, "expected no patches when currentProject is nil")
}

func TestBuildShowVerificationUIHookPatches_MissingHooksPathIsEmptyArray(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1, "expected 1 patch even with empty config")
	hooks, ok := patches[0].Value.([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} value, got %T", patches[0].Value)
	require.Len(t, hooks, 1, "expected single show_verification_ui hook")
	assert.Equal(t, "show_verification_ui", hooks[0]["hook"])
}

func TestNeedsHookPrefetch(t *testing.T) {
	cases := []struct {
		name string
		plan ProjectConfigResourceModel
		want bool
	}{
		{
			name: "all null",
			plan: ProjectConfigResourceModel{},
			want: false,
		},
		{
			name: "password set",
			plan: ProjectConfigResourceModel{
				SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
			},
			want: true,
		},
		{
			name: "oidc set false",
			plan: ProjectConfigResourceModel{
				SelfserviceFlowsRegistrationAfterOIDCHookShowVerificationUI: types.BoolValue(false),
			},
			want: true,
		},
		{
			name: "profile set",
			plan: ProjectConfigResourceModel{
				SelfserviceFlowsSettingsAfterProfileHookShowVerificationUI: types.BoolValue(true),
			},
			want: true,
		},
		{
			name: "password session hook set",
			plan: ProjectConfigResourceModel{
				SelfserviceFlowsRegistrationAfterPasswordHookSession: types.BoolValue(true),
			},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, needsHookPrefetch(&tc.plan))
		})
	}
}

func TestBuildHookPatches_SessionAddPreservesOtherHooks(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookSession: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"registration": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	p := patches[0]
	assert.Equal(t, "/services/identity/config/selfservice/flows/registration/after/password/hooks", p.Path)
	hooks, ok := p.Value.([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} value, got %T", p.Value)
	require.Len(t, hooks, 2, "expected 2 hooks (session + organization)")
	assert.Equal(t, "session", hooks[0]["hook"], "expected session first")
	assert.Equal(t, "organization", hooks[1]["hook"], "expected organization preserved")
}

func TestBuildHookPatches_SessionRemoveKeepsOthers(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookSession: types.BoolValue(false),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"registration": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "session"},
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1, "expected 1 remaining hook")
	assert.Equal(t, "organization", hooks[0]["hook"], "expected organization to remain")
}

// When two hook attributes (show_verification_ui and session) target the same
// hooks array, buildHookPatches emits a single patch that reflects both
// toggles instead of clobbering one with the other.
func TestBuildHookPatches_MultipleHooksAtSamePathMerged(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
		SelfserviceFlowsRegistrationAfterPasswordHookSession:            types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"registration": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1, "expected exactly 1 merged patch for the password hooks path")
	hooks, _ := patches[0].Value.([]map[string]interface{})
	seen := map[string]int{}
	for _, h := range hooks {
		if name, ok := h["hook"].(string); ok {
			seen[name]++
		}
	}
	assert.Equal(t, 1, seen["show_verification_ui"], "expected show_verification_ui to be present once")
	assert.Equal(t, 1, seen["session"], "expected session to be present once")
	assert.Equal(t, 1, seen["organization"], "expected organization to be preserved")
}

// settingsAfterProfileProject builds a project whose settings.after.profile
// hooks array holds the given entries.
func settingsAfterProfileProject(hooks ...interface{}) *ory.Project {
	return projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"settings": map[string]interface{}{
					"after": map[string]interface{}{
						"profile": map[string]interface{}{
							"hooks": hooks,
						},
					},
				},
			},
		},
	})
}

// hookByName returns the hooks-array entry with the given name.
func hookByName(t *testing.T, hooks []map[string]interface{}, name string) map[string]interface{} {
	t.Helper()
	for _, h := range hooks {
		if h["hook"] == name {
			return h
		}
	}
	t.Fatalf("hook %q not found in %v", name, hooks)
	return nil
}

// The Ory Console writes require_verified_address to the password login flow,
// and it must run before any other hook on that flow.
func TestBuildHookPatches_RequireVerifiedAddressAddedFirst(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"login": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	assert.Equal(t, "/services/identity/config/selfservice/flows/login/after/password/hooks", patches[0].Path)
	hooks, ok := patches[0].Value.([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} value, got %T", patches[0].Value)
	require.Len(t, hooks, 2)
	assert.Equal(t, "require_verified_address", hooks[0]["hook"], "expected require_verified_address first")
	assert.Equal(t, "organization", hooks[1]["hook"], "expected organization preserved")
	assert.NotContains(t, hooks[0], "config", "require_verified_address carries no config")
}

// The password and OIDC login flows have separate hooks arrays, so the two
// attributes must produce two independent patches.
func TestBuildHookPatches_RequireVerifiedAddressPasswordAndOIDCAreSeparate(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress: types.BoolValue(true),
		SelfserviceFlowsLoginAfterOIDCHookRequireVerifiedAddress:     types.BoolValue(false),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"login": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{"hooks": []interface{}{}},
						"oidc": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "require_verified_address"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 2, "expected one patch per login flow")

	password := findPatch(patches, "/services/identity/config/selfservice/flows/login/after/password/hooks")
	require.NotNil(t, password)
	passwordHooks, _ := password.Value.([]map[string]interface{})
	require.Len(t, passwordHooks, 1)
	assert.Equal(t, "require_verified_address", passwordHooks[0]["hook"])

	oidc := findPatch(patches, "/services/identity/config/selfservice/flows/login/after/oidc/hooks")
	require.NotNil(t, oidc)
	oidcHooks, _ := oidc.Value.([]map[string]interface{})
	assert.Empty(t, oidcHooks, "expected the OIDC hook to be removed")
}

func TestBuildHookPatches_RequireVerifiedAddressRemoveKeepsOthers(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress: types.BoolValue(false),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"login": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "require_verified_address"},
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1)
	assert.Equal(t, "organization", hooks[0]["hook"])
}

func TestBuildHookPatches_VerifyNewAddressAdded(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress: types.BoolValue(true),
	}
	current := settingsAfterProfileProject(map[string]interface{}{"hook": "organization"})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	assert.Equal(t, "/services/identity/config/selfservice/flows/settings/after/profile/hooks", patches[0].Path)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 2)
	assert.NotContains(t, hookByName(t, hooks, "verify_new_address"), "config", "verify_new_address carries no config")
}

// verify_new_address and show_verification_ui share a hooks array but are
// distinct toggles: enabling one must not imply the other.
func TestBuildHookPatches_VerifyNewAddressIndependentOfShowVerificationUI(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress:   types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookShowVerificationUI: types.BoolValue(false),
	}
	current := settingsAfterProfileProject(map[string]interface{}{"hook": "show_verification_ui"})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1, "expected one merged patch for the profile hooks path")
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1)
	assert.Equal(t, "verify_new_address", hooks[0]["hook"])
}

func TestBuildHookPatches_NotifyPreviousAddressesWithRecipients(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses:           types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients: types.StringValue("all_verified"),
	}
	current := settingsAfterProfileProject()

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1)
	assert.Equal(t, "notify_previous_addresses", hooks[0]["hook"])
	assert.Equal(t, map[string]interface{}{"recipients": "all_verified"}, hooks[0]["config"])
}

// Without an explicit recipient scope the hook is written bare, which lets Ory
// apply its own default instead of the provider inventing one.
func TestBuildHookPatches_NotifyPreviousAddressesWithoutRecipientsOmitsConfig(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses: types.BoolValue(true),
	}
	current := settingsAfterProfileProject()

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1)
	assert.NotContains(t, hooks[0], "config", "expected no config key when recipients is unset")
}

// Changing the recipient scope updates the existing entry in place rather than
// leaving the old scope behind.
func TestBuildHookPatches_NotifyPreviousAddressesRecipientsUpdatedInPlace(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses:           types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients: types.StringValue("all"),
	}
	current := settingsAfterProfileProject(
		map[string]interface{}{
			"hook":   "notify_previous_addresses",
			"config": map[string]interface{}{"recipients": "removed"},
		},
		map[string]interface{}{"hook": "organization"},
	)

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 2, "expected no duplicate notify_previous_addresses entry")
	notify := hookByName(t, hooks, "notify_previous_addresses")
	assert.Equal(t, map[string]interface{}{"recipients": "all"}, notify["config"])
	assert.Equal(t, "organization", hooks[1]["hook"], "expected organization preserved")
}

// An unset recipients attribute means "not managed here", so an existing scope
// on the project is left alone.
func TestBuildHookPatches_NotifyPreviousAddressesPreservesExistingConfigWhenUnset(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses: types.BoolValue(true),
	}
	current := settingsAfterProfileProject(map[string]interface{}{
		"hook":   "notify_previous_addresses",
		"config": map[string]interface{}{"recipients": "all_verified"},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1)
	assert.Equal(t, map[string]interface{}{"recipients": "all_verified"}, hooks[0]["config"])
}

func TestBuildHookPatches_NotifyPreviousAddressesRemoved(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses: types.BoolValue(false),
	}
	current := settingsAfterProfileProject(
		map[string]interface{}{
			"hook":   "notify_previous_addresses",
			"config": map[string]interface{}{"recipients": "all"},
		},
		map[string]interface{}{"hook": "verify_new_address"},
	)

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 1)
	assert.Equal(t, "verify_new_address", hooks[0]["hook"])
}

// All three email-verification toggles that share the profile hooks array must
// land in a single patch.
func TestBuildHookPatches_EmailVerificationHooksMergedAtProfilePath(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress:                  types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses:           types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients: types.StringValue("removed"),
		SelfserviceFlowsSettingsAfterProfileHookShowVerificationUI:                types.BoolValue(true),
	}
	current := settingsAfterProfileProject(map[string]interface{}{"hook": "organization"})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1, "expected exactly 1 merged patch for the profile hooks path")
	hooks, _ := patches[0].Value.([]map[string]interface{})
	seen := map[string]int{}
	for _, h := range hooks {
		if name, ok := h["hook"].(string); ok {
			seen[name]++
		}
	}
	assert.Equal(t, 1, seen["verify_new_address"])
	assert.Equal(t, 1, seen["notify_previous_addresses"])
	assert.Equal(t, 1, seen["show_verification_ui"])
	assert.Equal(t, 1, seen["organization"])
}

// Kratos runs a flow's hooks in list order, so the order the provider writes is
// part of its contract rather than an accident of the merge. Every hook is
// prepended as it is added, which is what the Ory Console does too, so the
// merged array comes out in reverse hookEntries order with pre-existing hooks
// last. Changing hookEntries order therefore changes execution order.
func TestBuildHookPatches_MergedProfileHookOrderIsStable(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookShowVerificationUI:      types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress:        types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses: types.BoolValue(true),
	}
	current := settingsAfterProfileProject(map[string]interface{}{"hook": "organization"})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})

	names := make([]string, 0, len(hooks))
	for _, h := range hooks {
		name, _ := h["hook"].(string)
		names = append(names, name)
	}
	assert.Equal(t, []string{
		"notify_previous_addresses",
		"verify_new_address",
		"show_verification_ui",
		"organization",
	}, names)
}

// require_verified_address must run before anything else on its login flow, so
// it stays at the head of the array even when other hooks are already there.
func TestBuildHookPatches_RequireVerifiedAddressStaysFirstAmongExistingHooks(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"login": map[string]interface{}{
					"after": map[string]interface{}{
						"password": map[string]interface{}{
							"hooks": []interface{}{
								map[string]interface{}{"hook": "revoke_active_sessions"},
								map[string]interface{}{"hook": "organization"},
							},
						},
					},
				},
			},
		},
	})

	patches := buildHookPatches(plan, current)
	require.Len(t, patches, 1)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	require.Len(t, hooks, 3)
	assert.Equal(t, "require_verified_address", hooks[0]["hook"])
	assert.Equal(t, "revoke_active_sessions", hooks[1]["hook"])
	assert.Equal(t, "organization", hooks[2]["hook"])
}

// feature_flags.password_profile_registration_node_group stores the string
// enum "password"/"default". The backend normalizes the key by comparing its
// string form with "password", so a raw JSON bool true stringifies to "true"
// and silently stores the false variant. The patch must send the enum.
func TestBuildPatches_PasswordProfileNodeGroupSendsEnumString(t *testing.T) {
	r := &ProjectConfigResource{}
	path := "/services/identity/config/feature_flags/password_profile_registration_node_group"

	for _, tc := range []struct {
		planned bool
		want    string
	}{
		{true, "password"},
		{false, "default"},
	} {
		plan := &ProjectConfigResourceModel{
			FeatureFlagsPasswordProfileRegistrationNodeGroup: types.BoolValue(tc.planned),
		}

		p := findPatch(r.buildPatches(context.Background(), plan), path)
		require.NotNil(t, p, "expected a patch for password_profile_registration_node_group")
		assert.Equal(t, "replace", p.Op)
		assert.Equal(t, tc.want, p.Value, "bool %v must patch the enum string %q", tc.planned, tc.want)
	}
}

// enable_ax_v2 has no config key of its own: the account experience config
// stores it as "enabled". Patching the spec-derived enable_ax_v2 key is
// accepted with HTTP 200 and silently discarded.
func TestBuildPatches_EnableAXV2PatchesEnabledKey(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		EnableAXV2: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/account_experience/config/enabled")
	require.NotNil(t, p, "expected a patch for the account experience 'enabled' key")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, true, p.Value)
	assert.Nil(t, findPatch(patches, "/services/account_experience/config/enable_ax_v2"),
		"the nonexistent enable_ax_v2 config key must not be patched")
}

// disable_account_experience_welcome_screen is a top-level revision column: a
// document patch to the spec-derived config path is accepted with HTTP 200 and
// silently discarded. It must be excluded from the document patches and routed
// through the revision patch builder instead.
func TestBuildPatches_WelcomeScreenFlagUsesRevisionEndpoint(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DisableAccountExperienceWelcomeScreen: types.BoolValue(true),
	}

	documentPatches := r.buildPatches(context.Background(), plan)
	assert.Nil(t, findPatch(documentPatches, "/services/account_experience/config/disable_welcome_screen"),
		"the nonexistent config document key must not be patched")
	assert.Nil(t, findPatch(documentPatches, "/disable_account_experience_welcome_screen"),
		"the revision column must not be sent to the document endpoint")

	revisionPatches := buildRevisionPatches(plan)
	p := findPatch(revisionPatches, "/disable_account_experience_welcome_screen")
	require.NotNil(t, p, "expected a revision patch for the welcome screen flag")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, true, p.Value)
}

// A null flag must produce no revision patches at all, so Create/Update skip
// the extra revision API call entirely.
func TestBuildRevisionPatches_NullFlagProducesNoPatches(t *testing.T) {
	assert.Empty(t, buildRevisionPatches(&ProjectConfigResourceModel{}))

	plan := &ProjectConfigResourceModel{
		DisableAccountExperienceWelcomeScreen: types.BoolValue(false),
	}
	patches := buildRevisionPatches(plan)
	require.Len(t, patches, 1, "an explicit false must still be sent")
	assert.Equal(t, false, patches[0].Value)
}
