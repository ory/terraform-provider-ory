package action

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// actionTestProject wraps an identity config map in an *ory.Project so the hook
// read helpers can be exercised without hitting the API.
func actionTestProject(identityConfig map[string]interface{}) *ory.Project {
	return &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{
				Config: identityConfig,
			},
		},
	}
}

func actionTestWebhook(url string) map[string]interface{} {
	return map[string]interface{}{
		"hook": "web_hook",
		"config": map[string]interface{}{
			"url":    url,
			"method": "POST",
		},
	}
}

// afterConfig builds selfservice.flows.<flow>.after with the given after-block contents.
func afterConfig(flow string, after map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				flow: map[string]interface{}{
					"after": after,
				},
			},
		},
	}
}

// allAuthMethods is every authentication method the auth_method schema enum
// accepts. It is also the union of the per-flow sets returned by
// authMethodsForFlow.
var allAuthMethods = []string{
	"password", "oidc", "code", "profile", "webauthn", "passkey", "totp", "lookup_secret", "saml",
}

// authMethodValidate runs the auth_method schema validators against a value and
// returns the diagnostics they produce.
func authMethodValidate(t *testing.T, value string) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()
	r := &ActionResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["auth_method"].(schema.StringAttribute)
	require.True(t, ok, "auth_method must be a StringAttribute")
	validators := attr.StringValidators()
	require.NotEmpty(t, validators, "auth_method must have validators")

	var diags diag.Diagnostics
	for _, v := range validators {
		resp := &validator.StringResponse{}
		v.ValidateString(ctx, validator.StringRequest{
			Path:        path.Root("auth_method"),
			ConfigValue: types.StringValue(value),
		}, resp)
		diags.Append(resp.Diagnostics...)
	}
	return diags
}

// TestAuthMethodValidatorAcceptsDocumentedMethods is the schema-level regression
// for issues #305 ("saml") and #328 ("profile"): both are methods the Ory API
// stores hooks under, but the auth_method validator rejected them. Every
// documented method must pass validation, while an unknown method must still be
// rejected so we know the validator is active.
func TestAuthMethodValidatorAcceptsDocumentedMethods(t *testing.T) {
	for _, method := range allAuthMethods {
		t.Run(method, func(t *testing.T) {
			assert.Falsef(t, authMethodValidate(t, method).HasError(), "auth_method %q must be accepted", method)
		})
	}

	assert.True(t, authMethodValidate(t, "magic").HasError(), "an unknown auth_method must be rejected")
}

func TestFlowSupportsAuthMethod(t *testing.T) {
	cases := map[string]bool{
		"login":        true,
		"registration": true,
		"settings":     true,
		"recovery":     false,
		"verification": false,
		"unknown":      false,
	}
	for flow, want := range cases {
		assert.Equalf(t, want, flowSupportsAuthMethod(flow), "flowSupportsAuthMethod(%q)", flow)
	}
}

// TestAuthMethodsForFlow pins the per-flow method sets to what the live Console
// API reports as the keys under selfservice.flows.<flow>.after. Writing a hook
// to a method the flow has no key for is accepted with HTTP 200 and discarded.
func TestAuthMethodsForFlow(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{"password", "oidc", "code", "webauthn", "passkey", "totp", "lookup_secret", "saml"},
		authMethodsForFlow("login"))
	assert.ElementsMatch(t,
		[]string{"password", "oidc", "code", "webauthn", "passkey", "saml"},
		authMethodsForFlow("registration"))
	assert.ElementsMatch(t,
		[]string{"password", "oidc", "profile", "webauthn", "passkey", "totp", "lookup_secret", "saml"},
		authMethodsForFlow("settings"))

	for _, flow := range []string{"recovery", "verification", "unknown"} {
		assert.Nilf(t, authMethodsForFlow(flow), "flow %q has no method-scoped after-hooks", flow)
	}
}

// TestAuthMethodsForFlowCoversSchemaEnum guards the two tables against drifting
// apart: every method the schema accepts must be supported by at least one flow,
// and no per-flow set may name a method the schema rejects. Without this, adding
// a method to the enum and forgetting authMethodsForFlow would make the provider
// warn about a method it advertises.
func TestAuthMethodsForFlowCoversSchemaEnum(t *testing.T) {
	union := map[string]bool{}
	for _, flow := range []string{"login", "registration", "settings"} {
		for _, method := range authMethodsForFlow(flow) {
			union[method] = true
			assert.Falsef(t, authMethodValidate(t, method).HasError(),
				"method %q is offered for flow %q but rejected by the schema enum", method, flow)
		}
	}
	for _, method := range allAuthMethods {
		assert.Truef(t, union[method], "method %q is in the schema enum but no flow supports it", method)
	}
}

// TestFlowSupportsMethod covers the method and flow pairs that differ from the
// flat enum: profile is settings-only, code is not a settings method, and
// totp/lookup_secret are not registration methods.
func TestFlowSupportsMethod(t *testing.T) {
	cases := []struct {
		flow, method string
		want         bool
	}{
		{"settings", "profile", true},
		{"login", "profile", false},
		{"registration", "profile", false},
		{"login", "code", true},
		{"registration", "code", true},
		{"settings", "code", false},
		{"login", "totp", true},
		{"settings", "totp", true},
		{"registration", "totp", false},
		{"registration", "lookup_secret", false},
		{"login", "saml", true},
		{"recovery", "password", false},
		{"verification", "password", false},
	}
	for _, tt := range cases {
		t.Run(tt.flow+"/"+tt.method, func(t *testing.T) {
			assert.Equal(t, tt.want, flowSupportsMethod(tt.flow, tt.method))
		})
	}
}

// TestVerifyFailureDetail verifies the apply-time "hook not found" diagnostic
// names an unsupported flow and method pair as the likely cause, and stays terse
// for a supported pair (where the cause is something else).
func TestVerifyFailureDetail(t *testing.T) {
	unsupported := verifyFailureDetail("login", "after", "profile", "https://example.com/hook")
	assert.Contains(t, unsupported, "does not support auth_method \"profile\"")
	assert.Contains(t, unsupported, "password, oidc, code", "the detail must list the flow's supported methods")

	supported := verifyFailureDetail("settings", "after", "profile", "https://example.com/hook")
	assert.Contains(t, supported, "Hook not found in PatchProject response")
	assert.NotContains(t, supported, "does not support auth_method")

	// A flat-hook flow ignores auth_method, so never blame the method.
	flat := verifyFailureDetail("verification", "after", "password", "https://example.com/hook")
	assert.NotContains(t, flat, "does not support auth_method")
}

func TestHookPath(t *testing.T) {
	r := &ActionResource{}
	tests := []struct {
		name       string
		flow       string
		timing     string
		authMethod string
		want       string
	}{
		{
			name: "login after is auth-method scoped", flow: "login", timing: "after", authMethod: "password",
			want: "/services/identity/config/selfservice/flows/login/after/password/hooks",
		},
		{
			name: "registration after is auth-method scoped", flow: "registration", timing: "after", authMethod: "oidc",
			want: "/services/identity/config/selfservice/flows/registration/after/oidc/hooks",
		},
		{
			name: "settings after is auth-method scoped", flow: "settings", timing: "after", authMethod: "password",
			want: "/services/identity/config/selfservice/flows/settings/after/password/hooks",
		},
		{
			// Regression for issue #328: the settings flow stores profile-update
			// hooks under the "profile" method.
			name: "settings after profile", flow: "settings", timing: "after", authMethod: "profile",
			want: "/services/identity/config/selfservice/flows/settings/after/profile/hooks",
		},
		{
			// Regression for issue #241: verification has no auth-method level.
			name: "verification after is flat", flow: "verification", timing: "after", authMethod: "password",
			want: "/services/identity/config/selfservice/flows/verification/after/hooks",
		},
		{
			name: "recovery after is flat", flow: "recovery", timing: "after", authMethod: "password",
			want: "/services/identity/config/selfservice/flows/recovery/after/hooks",
		},
		{
			name: "before timing is always flat", flow: "login", timing: "before", authMethod: "password",
			want: "/services/identity/config/selfservice/flows/login/before/hooks",
		},
		{
			name: "verification before is flat", flow: "verification", timing: "before", authMethod: "password",
			want: "/services/identity/config/selfservice/flows/verification/before/hooks",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, r.hookPath(tt.flow, tt.timing, tt.authMethod))
		})
	}
}

// TestGetHooksFromProject_VerificationReadsFlatHooks is the read-side regression
// for issue #241: verification/recovery "after" hooks live in a flat array, so
// getHooksFromProject must read .../after/hooks and ignore auth_method.
func TestGetHooksFromProject_VerificationReadsFlatHooks(t *testing.T) {
	r := &ActionResource{}

	for _, flow := range []string{"verification", "recovery"} {
		t.Run(flow, func(t *testing.T) {
			cfg := afterConfig(flow, map[string]interface{}{
				"hooks": []interface{}{actionTestWebhook("https://example.com/" + flow)},
			})
			hooks := r.getHooksFromProject(actionTestProject(cfg), flow, "after", "password")
			require.Len(t, hooks, 1)
			config, _ := hooks[0]["config"].(map[string]interface{})
			assert.Equal(t, "https://example.com/"+flow, config["url"])
		})
	}
}

// TestGetHooksFromProject_AuthScopedFlows verifies that login/registration/settings
// "after" hooks are read from the auth-method-scoped path and do not fall back to
// the flat array (which would mix hooks across methods).
func TestGetHooksFromProject_AuthScopedFlows(t *testing.T) {
	r := &ActionResource{}

	// Hook nested under the password method is found for auth_method=password.
	scoped := afterConfig("login", map[string]interface{}{
		"password": map[string]interface{}{
			"hooks": []interface{}{actionTestWebhook("https://example.com/login-password")},
		},
	})
	hooks := r.getHooksFromProject(actionTestProject(scoped), "login", "after", "password")
	require.Len(t, hooks, 1)
	config, _ := hooks[0]["config"].(map[string]interface{})
	assert.Equal(t, "https://example.com/login-password", config["url"])

	// A flat hook is NOT returned when reading the password-scoped path.
	flatOnly := afterConfig("login", map[string]interface{}{
		"hooks": []interface{}{actionTestWebhook("https://example.com/login-global")},
	})
	hooks = r.getHooksFromProject(actionTestProject(flatOnly), "login", "after", "password")
	assert.Empty(t, hooks, "auth-scoped read must not pick up flat after-hooks")
}

// TestGetHooksFromProject_SettingsProfile is the read-side regression for issue
// #328. On a real project, settings.after.profile already carries the built-in
// verify_new_address and organization hooks, so the read must return the whole
// array and findHookIndex must pick out the web_hook without tripping over the
// entries that have no config block.
func TestGetHooksFromProject_SettingsProfile(t *testing.T) {
	r := &ActionResource{}
	cfg := afterConfig("settings", map[string]interface{}{
		"profile": map[string]interface{}{
			"hooks": []interface{}{
				map[string]interface{}{"hook": "verify_new_address"},
				actionTestWebhook("https://example.com/profile-updated"),
				map[string]interface{}{"hook": "organization"},
			},
		},
	})

	hooks := r.getHooksFromProject(actionTestProject(cfg), "settings", "after", "profile")
	require.Len(t, hooks, 3)

	index := r.findHookIndex(hooks, "https://example.com/profile-updated", "POST")
	require.Equal(t, 1, index)

	// The password method on the same flow must not see the profile hooks.
	assert.Empty(t, r.getHooksFromProject(actionTestProject(cfg), "settings", "after", "password"))
}

// TestGetHooksFromProject_BeforeTiming verifies before-hooks are read flat for all flows.
func TestGetHooksFromProject_BeforeTiming(t *testing.T) {
	r := &ActionResource{}
	cfg := map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"login": map[string]interface{}{
					"before": map[string]interface{}{
						"hooks": []interface{}{actionTestWebhook("https://example.com/pre-login")},
					},
				},
			},
		},
	}
	hooks := r.getHooksFromProject(actionTestProject(cfg), "login", "before", "password")
	require.Len(t, hooks, 1)
	config, _ := hooks[0]["config"].(map[string]interface{})
	assert.Equal(t, "https://example.com/pre-login", config["url"])
}

// TestGetHooksFromProject_DeletedProject verifies a soft-deleted project (which
// GetProject still returns with state "deleted") reports no hooks, so Delete and
// Read converge during teardown instead of trying to patch a gone project.
func TestGetHooksFromProject_DeletedProject(t *testing.T) {
	r := &ActionResource{}
	cfg := afterConfig("login", map[string]interface{}{
		"password": map[string]interface{}{
			"hooks": []interface{}{actionTestWebhook("https://example.com/x")},
		},
	})
	project := actionTestProject(cfg)
	project.State = projectStateDeleted
	assert.Empty(t, r.getHooksFromProject(project, "login", "after", "password"))
}
