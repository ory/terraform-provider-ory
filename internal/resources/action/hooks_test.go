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

// TestAuthMethodValidatorAcceptsSAML is the schema-level regression for issue
// #305: the Ory Console UI offers "saml" as an after-login authentication
// method, but the auth_method validator rejected it. Every documented method
// (including saml) must pass validation, while an unknown method must still be
// rejected so we know the validator is active.
func TestAuthMethodValidatorAcceptsSAML(t *testing.T) {
	ctx := context.Background()
	r := &ActionResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes["auth_method"].(schema.StringAttribute)
	require.True(t, ok, "auth_method must be a StringAttribute")
	validators := attr.StringValidators()
	require.NotEmpty(t, validators, "auth_method must have validators")

	validate := func(value string) diag.Diagnostics {
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

	for _, method := range []string{"password", "oidc", "code", "webauthn", "passkey", "totp", "lookup_secret", "saml"} {
		t.Run(method, func(t *testing.T) {
			assert.Falsef(t, validate(method).HasError(), "auth_method %q must be accepted", method)
		})
	}

	assert.True(t, validate("magic").HasError(), "an unknown auth_method must be rejected")
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

// TestHooksExcluding verifies Delete rebuilds the hooks array without the
// removed element, preserving order, for the `replace` patch it sends.
func TestHooksExcluding(t *testing.T) {
	a := actionTestWebhook("https://example.com/a")
	b := actionTestWebhook("https://example.com/b")
	c := actionTestWebhook("https://example.com/c")

	t.Run("removes middle element, preserves order", func(t *testing.T) {
		got := hooksExcluding([]map[string]interface{}{a, b, c}, 1)
		require.Len(t, got, 2)
		assert.Equal(t, "https://example.com/a", got[0].(map[string]interface{})["config"].(map[string]interface{})["url"])
		assert.Equal(t, "https://example.com/c", got[1].(map[string]interface{})["config"].(map[string]interface{})["url"])
	})

	t.Run("removing the only hook yields a non-nil empty slice", func(t *testing.T) {
		got := hooksExcluding([]map[string]interface{}{a}, 0)
		assert.NotNil(t, got, "must be non-nil so replace sends [] rather than null")
		assert.Empty(t, got)
	})

	t.Run("out-of-range index keeps all hooks", func(t *testing.T) {
		got := hooksExcluding([]map[string]interface{}{a, b}, -1)
		assert.Len(t, got, 2)
	})
}
