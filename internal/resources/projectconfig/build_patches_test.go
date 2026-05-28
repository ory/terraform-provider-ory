package projectconfig

import (
	"context"
	"testing"

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
