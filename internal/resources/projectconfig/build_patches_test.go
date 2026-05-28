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
	if p == nil {
		t.Fatal("expected a patch for default_browser_return_url, got none")
	}
	if p.Op != "remove" {
		t.Errorf("expected op 'remove' for empty default_return_url, got %q", p.Op)
	}
}

func TestBuildPatches_NonEmptyDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringValue("https://app.example.com"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	if p == nil {
		t.Fatal("expected a patch for default_browser_return_url, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace' for non-empty default_return_url, got %q", p.Op)
	}
}

func TestBuildPatches_NullDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	if p != nil {
		t.Error("expected no patch for null default_return_url")
	}
}

func TestBuildPatches_EmptyAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	emptyList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{})
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: emptyList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	if p == nil {
		t.Fatal("expected a patch for allowed_return_urls, got none")
	}
	if p.Op != "remove" {
		t.Errorf("expected op 'remove' for empty allowed_return_urls, got %q", p.Op)
	}
}

func TestBuildPatches_NonEmptyAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	urlList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"https://app.example.com"})
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: urlList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	if p == nil {
		t.Fatal("expected a patch for allowed_return_urls, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace' for non-empty allowed_return_urls, got %q", p.Op)
	}
}

func TestBuildPatches_NullAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: types.ListNull(types.StringType),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	if p != nil {
		t.Error("expected no patch for null allowed_return_urls")
	}
}

func TestBuildPatches_OAuth2IssuerURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2IssuerURL: types.StringValue("https://auth.example.com"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/urls/self/issuer")
	if p == nil {
		t.Fatal("expected a patch for oauth2_issuer_url, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "https://auth.example.com" {
		t.Errorf("expected value 'https://auth.example.com', got %v", p.Value)
	}
}

func TestBuildPatches_OAuth2CookiesSameSiteMode(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2CookiesSameSiteMode: types.StringValue("Strict"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/serve/cookies/same_site_mode")
	if p == nil {
		t.Fatal("expected a patch for oauth2_cookies_same_site_mode, got none")
	}
	if p.Value != "Strict" {
		t.Errorf("expected value 'Strict', got %v", p.Value)
	}
}

func TestBuildPatches_OAuth2CookiesSameSiteLegacyWorkaround(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2CookiesSameSiteLegacyWorkaround: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/serve/cookies/same_site_legacy_workaround")
	if p == nil {
		t.Fatal("expected a patch for oauth2_cookies_same_site_legacy_workaround, got none")
	}
	if p.Value != true {
		t.Errorf("expected value true, got %v", p.Value)
	}
}

func TestBuildPatches_NullOAuth2IssuerURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2IssuerURL: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/urls/self/issuer")
	if p != nil {
		t.Error("expected no patch for null oauth2_issuer_url")
	}
}

func TestBuildPatches_LoginStyle(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		LoginStyle: types.StringValue("identifier_first"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/style")
	if p == nil {
		t.Fatal("expected a patch for login_style, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "identifier_first" {
		t.Errorf("expected value 'identifier_first', got %v", p.Value)
	}
}

func TestBuildPatches_LoginStyleUnified(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		LoginStyle: types.StringValue("unified"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/style")
	if p == nil {
		t.Fatal("expected a patch for login_style, got none")
	}
	if p.Value != "unified" {
		t.Errorf("expected value 'unified', got %v", p.Value)
	}
}

func TestBuildPatches_NullLoginStyle(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		LoginStyle: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/style")
	if p != nil {
		t.Error("expected no patch for null login_style")
	}
}

func TestBuildPatches_EnableProfile(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		EnableProfile: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/profile/enabled")
	if p == nil {
		t.Fatal("expected a patch for enable_profile, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != true {
		t.Errorf("expected value true, got %v", p.Value)
	}
}

func TestBuildPatches_NullEnableProfile(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		EnableProfile: types.BoolNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/profile/enabled")
	if p != nil {
		t.Error("expected no patch for null enable_profile")
	}
}

func TestBuildPatches_CodeLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeLifespan: types.StringValue("15m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/lifespan")
	if p == nil {
		t.Fatal("expected a patch for code_lifespan, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "15m0s" {
		t.Errorf("expected value '15m0s', got %v", p.Value)
	}
}

func TestBuildPatches_NullCodeLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeLifespan: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/lifespan")
	if p != nil {
		t.Error("expected no patch for null code_lifespan")
	}
}

func TestBuildPatches_CodeMissingCredentialFallbackEnabled(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeMissingCredentialFallbackEnabled: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/missing_credential_fallback_enabled")
	if p == nil {
		t.Fatal("expected a patch for code_missing_credential_fallback_enabled, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != true {
		t.Errorf("expected value true, got %v", p.Value)
	}
}

func TestBuildPatches_NullCodeMissingCredentialFallbackEnabled(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		CodeMissingCredentialFallbackEnabled: types.BoolNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/methods/code/config/missing_credential_fallback_enabled")
	if p != nil {
		t.Error("expected no patch for null code_missing_credential_fallback_enabled")
	}
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
	if p == nil {
		t.Fatal("expected a patch for settings_lifespan, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "30m0s" {
		t.Errorf("expected value '30m0s', got %v", p.Value)
	}
}

func TestBuildPatches_NullSettingsLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsLifespan: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/lifespan")
	if p != nil {
		t.Error("expected no patch for null settings_lifespan")
	}
}

func TestBuildPatches_SettingsPrivilegedSessionMaxAge(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsPrivilegedSessionMaxAge: types.StringValue("15m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/privileged_session_max_age")
	if p == nil {
		t.Fatal("expected a patch for settings_privileged_session_max_age, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "15m0s" {
		t.Errorf("expected value '15m0s', got %v", p.Value)
	}
}

func TestBuildPatches_NullSettingsPrivilegedSessionMaxAge(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SettingsPrivilegedSessionMaxAge: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/settings/privileged_session_max_age")
	if p != nil {
		t.Error("expected no patch for null settings_privileged_session_max_age")
	}
}

func TestBuildPatches_VerificationUse(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationUse: types.StringValue("code"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/use")
	if p == nil {
		t.Fatal("expected a patch for verification_use, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "code" {
		t.Errorf("expected value 'code', got %v", p.Value)
	}
}

func TestBuildPatches_NullVerificationUse(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationUse: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/use")
	if p != nil {
		t.Error("expected no patch for null verification_use")
	}
}

func TestBuildPatches_VerificationLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationLifespan: types.StringValue("30m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/lifespan")
	if p == nil {
		t.Fatal("expected a patch for verification_lifespan, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "30m0s" {
		t.Errorf("expected value '30m0s', got %v", p.Value)
	}
}

func TestBuildPatches_NullVerificationLifespan(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationLifespan: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/lifespan")
	if p != nil {
		t.Error("expected no patch for null verification_lifespan")
	}
}

func TestBuildPatches_VerificationNotifyUnknownRecipients(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationNotifyUnknownRecipients: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/notify_unknown_recipients")
	if p == nil {
		t.Fatal("expected a patch for verification_notify_unknown_recipients, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != true {
		t.Errorf("expected value true, got %v", p.Value)
	}
}

func TestBuildPatches_NullVerificationNotifyUnknownRecipients(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		VerificationNotifyUnknownRecipients: types.BoolNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/verification/notify_unknown_recipients")
	if p != nil {
		t.Error("expected no patch for null verification_notify_unknown_recipients")
	}
}

// projectWithIdentityConfig wraps an identity config map in the *ory.Project
// shape that buildShowVerificationUIHookPatches expects.
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

	patches := buildShowVerificationUIHookPatches(plan, current)
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	p := patches[0]
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Path != "/services/identity/config/selfservice/flows/registration/after/password/hooks" {
		t.Errorf("unexpected patch path: %s", p.Path)
	}
	hooks, ok := p.Value.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{} value, got %T", p.Value)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks (show_verification_ui + organization), got %d", len(hooks))
	}
	if hooks[0]["hook"] != "show_verification_ui" {
		t.Errorf("expected show_verification_ui first, got %v", hooks[0]["hook"])
	}
	if hooks[1]["hook"] != "organization" {
		t.Errorf("expected organization preserved, got %v", hooks[1]["hook"])
	}
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

	patches := buildShowVerificationUIHookPatches(plan, current)
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	hooks, ok := patches[0].Value.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{} value, got %T", patches[0].Value)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 remaining hook, got %d", len(hooks))
	}
	if hooks[0]["hook"] != "organization" {
		t.Errorf("expected organization to remain, got %v", hooks[0]["hook"])
	}
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

	patches := buildShowVerificationUIHookPatches(plan, current)
	hooks, _ := patches[0].Value.([]map[string]interface{})
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks (no duplicate), got %d: %v", len(hooks), hooks)
	}
	seen := map[string]int{}
	for _, h := range hooks {
		if name, ok := h["hook"].(string); ok {
			seen[name]++
		}
	}
	if seen["show_verification_ui"] != 1 {
		t.Errorf("expected exactly one show_verification_ui, got %d", seen["show_verification_ui"])
	}
	if seen["session"] != 1 {
		t.Errorf("expected session preserved, got %d", seen["session"])
	}
}

func TestBuildShowVerificationUIHookPatches_NullSkipped(t *testing.T) {
	plan := &ProjectConfigResourceModel{}
	current := projectWithIdentityConfig(map[string]interface{}{})

	patches := buildShowVerificationUIHookPatches(plan, current)
	if len(patches) != 0 {
		t.Errorf("expected no patches when all hook attrs are null, got %d", len(patches))
	}
}

func TestBuildShowVerificationUIHookPatches_NilCurrentProject(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
	}
	patches := buildShowVerificationUIHookPatches(plan, nil)
	if len(patches) != 0 {
		t.Errorf("expected no patches when currentProject is nil, got %d", len(patches))
	}
}

func TestBuildShowVerificationUIHookPatches_MissingHooksPathIsEmptyArray(t *testing.T) {
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsRegistrationAfterPasswordHookShowVerificationUI: types.BoolValue(true),
	}
	current := projectWithIdentityConfig(map[string]interface{}{})

	patches := buildShowVerificationUIHookPatches(plan, current)
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch even with empty config, got %d", len(patches))
	}
	hooks, ok := patches[0].Value.([]map[string]interface{})
	if !ok || len(hooks) != 1 {
		t.Fatalf("expected single show_verification_ui hook, got %v", patches[0].Value)
	}
	if hooks[0]["hook"] != "show_verification_ui" {
		t.Errorf("expected show_verification_ui hook, got %v", hooks[0])
	}
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := needsHookPrefetch(&tc.plan); got != tc.want {
				t.Errorf("needsHookPrefetch() = %v, want %v", got, tc.want)
			}
		})
	}
}
