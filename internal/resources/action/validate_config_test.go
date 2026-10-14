package action

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildActionTestConfig creates a ValidateConfigRequest from an ActionResourceModel.
func buildActionTestConfig(t *testing.T, model ActionResourceModel) resource.ValidateConfigRequest {
	t.Helper()

	r := &ActionResource{}
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	vals := map[string]tftypes.Value{
		"id":                                  tftypes.NewValue(tftypes.String, nil),
		"project_id":                          tftypes.NewValue(tftypes.String, nil),
		"flow":                                tfActionStringValue(model.Flow),
		"timing":                              tfActionStringValue(model.Timing),
		"auth_method":                         tfActionStringValue(model.AuthMethod),
		"url":                                 tfActionStringValue(model.URL),
		"method":                              tfActionStringValue(model.HTTPMethod),
		"body":                                tftypes.NewValue(tftypes.String, nil),
		"response_ignore":                     tftypes.NewValue(tftypes.Bool, nil),
		"response_parse":                      tftypes.NewValue(tftypes.Bool, nil),
		"can_interrupt":                       tftypes.NewValue(tftypes.Bool, nil),
		"webhook_auth_type":                   tfActionStringValue(model.WebhookAuthType),
		"webhook_auth_basic_auth_user":        tfActionStringValue(model.WebhookAuthBasicAuthUser),
		"webhook_auth_basic_auth_password":    tfActionStringValue(model.WebhookAuthBasicAuthPassword),
		"webhook_auth_basic_auth_password_wo": tfActionStringValue(model.WebhookAuthBasicAuthPasswordWO),
		"webhook_auth_basic_auth_password_wo_version": tfActionStringValue(model.WebhookAuthBasicAuthPasswordWOVersion),
		"webhook_auth_api_key_name":                   tfActionStringValue(model.WebhookAuthAPIKeyName),
		"webhook_auth_api_key_value":                  tfActionStringValue(model.WebhookAuthAPIKeyValue),
		"webhook_auth_api_key_value_wo":               tfActionStringValue(model.WebhookAuthAPIKeyValueWO),
		"webhook_auth_api_key_value_wo_version":       tfActionStringValue(model.WebhookAuthAPIKeyValueWOVersion),
		"webhook_auth_api_key_in":                     tfActionStringValue(model.WebhookAuthAPIKeyIn),
	}

	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":                                  tftypes.String,
			"project_id":                          tftypes.String,
			"flow":                                tftypes.String,
			"timing":                              tftypes.String,
			"auth_method":                         tftypes.String,
			"url":                                 tftypes.String,
			"method":                              tftypes.String,
			"body":                                tftypes.String,
			"response_ignore":                     tftypes.Bool,
			"response_parse":                      tftypes.Bool,
			"can_interrupt":                       tftypes.Bool,
			"webhook_auth_type":                   tftypes.String,
			"webhook_auth_basic_auth_user":        tftypes.String,
			"webhook_auth_basic_auth_password":    tftypes.String,
			"webhook_auth_basic_auth_password_wo": tftypes.String,
			"webhook_auth_basic_auth_password_wo_version": tftypes.String,
			"webhook_auth_api_key_name":                   tftypes.String,
			"webhook_auth_api_key_value":                  tftypes.String,
			"webhook_auth_api_key_value_wo":               tftypes.String,
			"webhook_auth_api_key_value_wo_version":       tftypes.String,
			"webhook_auth_api_key_in":                     tftypes.String,
		},
	}

	rawConfig := tftypes.NewValue(objType, vals)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    rawConfig,
	}

	return resource.ValidateConfigRequest{Config: config}
}

// tfActionStringValue converts a types.String to a tftypes.Value, preserving null/unknown state.
func tfActionStringValue(v types.String) tftypes.Value {
	if v.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}
	if v.IsUnknown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	return tftypes.NewValue(tftypes.String, v.ValueString())
}

// TestValidateConfig_AuthMethodOnUnsupportedFlow_Warns verifies that explicitly
// setting auth_method on the recovery/verification flows (which have no
// auth-method-scoped hooks) produces a warning. Regression for issue #241.
func TestValidateConfig_AuthMethodOnUnsupportedFlow_Warns(t *testing.T) {
	for _, flow := range []string{"verification", "recovery"} {
		t.Run(flow, func(t *testing.T) {
			r := &ActionResource{}
			ctx := context.Background()

			req := buildActionTestConfig(t, ActionResourceModel{
				Flow:       types.StringValue(flow),
				Timing:     types.StringValue("after"),
				AuthMethod: types.StringValue("code"),
				URL:        types.StringValue("https://example.com/webhook"),
			})
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, req, &resp)

			assert.False(t, resp.Diagnostics.HasError(), "auth_method on %s must not be an error: %v", flow, resp.Diagnostics.Errors())
			assert.NotEmpty(t, resp.Diagnostics.Warnings(), "expected a warning for auth_method on %s flow", flow)
		})
	}
}

// TestValidateConfig_AuthMethodOnAuthScopedFlow_NoWarn verifies that setting
// auth_method on login/registration/settings does not warn.
func TestValidateConfig_AuthMethodOnAuthScopedFlow_NoWarn(t *testing.T) {
	for _, flow := range []string{"login", "registration", "settings"} {
		t.Run(flow, func(t *testing.T) {
			r := &ActionResource{}
			ctx := context.Background()

			req := buildActionTestConfig(t, ActionResourceModel{
				Flow:       types.StringValue(flow),
				Timing:     types.StringValue("after"),
				AuthMethod: types.StringValue("password"),
				URL:        types.StringValue("https://example.com/webhook"),
			})
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, req, &resp)

			assert.Empty(t, resp.Diagnostics.Warnings(), "did not expect a warning for auth_method on %s flow: %v", flow, resp.Diagnostics.Warnings())
		})
	}
}

// TestValidateConfig_ProfileOnSettings_NoWarn is the plan-time regression for
// issue #328: auth_method = "profile" is valid on the settings flow and must
// neither error nor warn.
func TestValidateConfig_ProfileOnSettings_NoWarn(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:       types.StringValue("settings"),
		Timing:     types.StringValue("after"),
		AuthMethod: types.StringValue("profile"),
		URL:        types.StringValue("https://example.com/webhook"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "auth_method \"profile\" on settings must not error: %v", resp.Diagnostics.Errors())
	assert.Empty(t, resp.Diagnostics.Warnings(), "auth_method \"profile\" on settings must not warn: %v", resp.Diagnostics.Warnings())
}

// TestValidateConfig_MethodNotOnFlow_Warns verifies that a method the schema
// accepts but the chosen flow has no key for warns at plan time. Ory accepts such
// a write with HTTP 200 and discards the hook, so without the warning the first
// signal is an opaque verification failure during apply.
func TestValidateConfig_MethodNotOnFlow_Warns(t *testing.T) {
	cases := []struct{ flow, method string }{
		{"login", "profile"},
		{"registration", "profile"},
		{"settings", "code"},
		{"registration", "totp"},
		{"registration", "lookup_secret"},
	}
	for _, tt := range cases {
		t.Run(tt.flow+"/"+tt.method, func(t *testing.T) {
			r := &ActionResource{}
			ctx := context.Background()

			req := buildActionTestConfig(t, ActionResourceModel{
				Flow:       types.StringValue(tt.flow),
				Timing:     types.StringValue("after"),
				AuthMethod: types.StringValue(tt.method),
				URL:        types.StringValue("https://example.com/webhook"),
			})
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, req, &resp)

			assert.False(t, resp.Diagnostics.HasError(),
				"an unsupported method must warn, not error: %v", resp.Diagnostics.Errors())
			require.NotEmpty(t, resp.Diagnostics.Warnings(),
				"expected a warning for auth_method %q on the %q flow", tt.method, tt.flow)
			assert.Contains(t, resp.Diagnostics.Warnings()[0].Detail(), "does not support auth_method")
		})
	}
}

// TestValidateConfig_MethodNotOnFlow_UnknownTiming_NoWarn verifies the
// unsupported-method warning is suppressed while timing is unknown, since
// auth_method is ignored outright for "before" hooks.
func TestValidateConfig_MethodNotOnFlow_UnknownTiming_NoWarn(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:       types.StringValue("login"),
		Timing:     types.StringUnknown(),
		AuthMethod: types.StringValue("profile"),
		URL:        types.StringValue("https://example.com/webhook"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.Empty(t, resp.Diagnostics.Warnings(), "did not expect a warning while timing is unknown: %v", resp.Diagnostics.Warnings())
}

// TestValidateConfig_AuthMethodOnBeforeTiming_Warns verifies that auth_method set
// on a "before" hook warns even for an auth-scoped flow, since auth_method only
// applies to "after" hooks. Covers the timing dimension of the warning.
func TestValidateConfig_AuthMethodOnBeforeTiming_Warns(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:       types.StringValue("login"),
		Timing:     types.StringValue("before"),
		AuthMethod: types.StringValue("password"),
		URL:        types.StringValue("https://example.com/webhook"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "auth_method on a before hook must not be an error: %v", resp.Diagnostics.Errors())
	assert.NotEmpty(t, resp.Diagnostics.Warnings(), "expected a warning for auth_method on a before hook")
}

// TestValidateConfig_NoAuthMethodOnVerification_NoWarn verifies that the common
// case (auth_method unset, defaulted by the schema) does not warn — only an
// explicit value does.
func TestValidateConfig_NoAuthMethodOnVerification_NoWarn(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:   types.StringValue("verification"),
		Timing: types.StringValue("after"),
		URL:    types.StringValue("https://example.com/webhook"),
		// AuthMethod left null — the schema default applies after validation.
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.Empty(t, resp.Diagnostics.Warnings(), "did not expect a warning when auth_method is unset: %v", resp.Diagnostics.Warnings())
}

// TestValidateConfig_NoWebhookAuth passes when no webhook auth is configured.
func TestValidateConfig_NoWebhookAuth(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:   types.StringValue("login"),
		Timing: types.StringValue("after"),
		URL:    types.StringValue("https://example.com/webhook"),
		// No webhook auth fields
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors when webhook auth is not configured: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_APIKey_AllKnown_Passes verifies valid api_key config passes.
func TestValidateConfig_APIKey_AllKnown_Passes(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                   types.StringValue("login"),
		Timing:                 types.StringValue("after"),
		URL:                    types.StringValue("https://example.com/webhook"),
		WebhookAuthType:        types.StringValue("api_key"),
		WebhookAuthAPIKeyName:  types.StringValue("X-API-Key"),
		WebhookAuthAPIKeyValue: types.StringValue("secret"),
		WebhookAuthAPIKeyIn:    types.StringValue("header"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for valid api_key config: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_APIKey_MissingValue_Fails verifies that null api_key_value fails.
func TestValidateConfig_APIKey_MissingValue_Fails(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                  types.StringValue("login"),
		Timing:                types.StringValue("after"),
		URL:                   types.StringValue("https://example.com/webhook"),
		WebhookAuthType:       types.StringValue("api_key"),
		WebhookAuthAPIKeyName: types.StringValue("X-API-Key"),
		// WebhookAuthAPIKeyValue is null — should fail
		WebhookAuthAPIKeyIn: types.StringValue("header"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error for missing webhook_auth_api_key_value")
}

// TestValidateConfig_APIKey_UnknownValue_SkipsValidation verifies that an unknown
// webhook_auth_api_key_value (e.g. from AWS Secrets Manager) defers validation.
func TestValidateConfig_APIKey_UnknownValue_SkipsValidation(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                   types.StringValue("login"),
		Timing:                 types.StringValue("after"),
		URL:                    types.StringValue("https://example.com/webhook"),
		WebhookAuthType:        types.StringValue("api_key"),
		WebhookAuthAPIKeyName:  types.StringValue("X-API-Key"),
		WebhookAuthAPIKeyValue: types.StringUnknown(), // sourced from data source
		WebhookAuthAPIKeyIn:    types.StringValue("header"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown webhook_auth_api_key_value: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_APIKey_UnknownName_SkipsValidation verifies that an unknown
// webhook_auth_api_key_name defers validation.
func TestValidateConfig_APIKey_UnknownName_SkipsValidation(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                   types.StringValue("login"),
		Timing:                 types.StringValue("after"),
		URL:                    types.StringValue("https://example.com/webhook"),
		WebhookAuthType:        types.StringValue("api_key"),
		WebhookAuthAPIKeyName:  types.StringUnknown(), // sourced from data source
		WebhookAuthAPIKeyValue: types.StringValue("secret"),
		WebhookAuthAPIKeyIn:    types.StringValue("header"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown webhook_auth_api_key_name: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_APIKey_UnknownIn_SkipsValidation verifies that an unknown
// webhook_auth_api_key_in defers validation.
func TestValidateConfig_APIKey_UnknownIn_SkipsValidation(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                   types.StringValue("login"),
		Timing:                 types.StringValue("after"),
		URL:                    types.StringValue("https://example.com/webhook"),
		WebhookAuthType:        types.StringValue("api_key"),
		WebhookAuthAPIKeyName:  types.StringValue("X-API-Key"),
		WebhookAuthAPIKeyValue: types.StringValue("secret"),
		WebhookAuthAPIKeyIn:    types.StringUnknown(), // sourced from data source
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown webhook_auth_api_key_in: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_BasicAuth_AllKnown_Passes verifies valid basic_auth config passes.
func TestValidateConfig_BasicAuth_AllKnown_Passes(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                         types.StringValue("registration"),
		Timing:                       types.StringValue("after"),
		URL:                          types.StringValue("https://example.com/webhook"),
		WebhookAuthType:              types.StringValue("basic_auth"),
		WebhookAuthBasicAuthUser:     types.StringValue("user"),
		WebhookAuthBasicAuthPassword: types.StringValue("pass"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for valid basic_auth config: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_BasicAuth_MissingPassword_Fails verifies that null password fails.
func TestValidateConfig_BasicAuth_MissingPassword_Fails(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                     types.StringValue("registration"),
		Timing:                   types.StringValue("after"),
		URL:                      types.StringValue("https://example.com/webhook"),
		WebhookAuthType:          types.StringValue("basic_auth"),
		WebhookAuthBasicAuthUser: types.StringValue("user"),
		// WebhookAuthBasicAuthPassword is null — should fail
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error for missing webhook_auth_basic_auth_password")
}

// TestValidateConfig_BasicAuth_UnknownPassword_SkipsValidation verifies that an unknown
// password (e.g. from AWS Secrets Manager) defers validation.
func TestValidateConfig_BasicAuth_UnknownPassword_SkipsValidation(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                         types.StringValue("registration"),
		Timing:                       types.StringValue("after"),
		URL:                          types.StringValue("https://example.com/webhook"),
		WebhookAuthType:              types.StringValue("basic_auth"),
		WebhookAuthBasicAuthUser:     types.StringValue("user"),
		WebhookAuthBasicAuthPassword: types.StringUnknown(), // sourced from data source
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown webhook_auth_basic_auth_password: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_BasicAuth_UnknownUser_SkipsValidation verifies that an unknown
// user (e.g. from AWS Secrets Manager) defers validation.
func TestValidateConfig_BasicAuth_UnknownUser_SkipsValidation(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                         types.StringValue("registration"),
		Timing:                       types.StringValue("after"),
		URL:                          types.StringValue("https://example.com/webhook"),
		WebhookAuthType:              types.StringValue("basic_auth"),
		WebhookAuthBasicAuthUser:     types.StringUnknown(), // sourced from data source
		WebhookAuthBasicAuthPassword: types.StringValue("pass"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown webhook_auth_basic_auth_user: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_BasicAuth_WriteOnlyPassword_Passes verifies that supplying
// only the write-only webhook_auth_basic_auth_password_wo satisfies the password
// requirement for basic_auth.
func TestValidateConfig_BasicAuth_WriteOnlyPassword_Passes(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                           types.StringValue("registration"),
		Timing:                         types.StringValue("after"),
		URL:                            types.StringValue("https://example.com/webhook"),
		WebhookAuthType:                types.StringValue("basic_auth"),
		WebhookAuthBasicAuthUser:       types.StringValue("user"),
		WebhookAuthBasicAuthPasswordWO: types.StringValue("wo-pass"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for write-only basic auth password: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_APIKey_WriteOnlyValue_Passes verifies that supplying only the
// write-only webhook_auth_api_key_value_wo satisfies the value requirement for
// api_key.
func TestValidateConfig_APIKey_WriteOnlyValue_Passes(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:                     types.StringValue("login"),
		Timing:                   types.StringValue("after"),
		URL:                      types.StringValue("https://example.com/webhook"),
		WebhookAuthType:          types.StringValue("api_key"),
		WebhookAuthAPIKeyName:    types.StringValue("X-API-Key"),
		WebhookAuthAPIKeyValueWO: types.StringValue("wo-secret"),
		WebhookAuthAPIKeyIn:      types.StringValue("header"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for write-only api key value: %v", resp.Diagnostics.Errors())
}

// TestValidateConfig_APIKey_UnknownValue_NullName_StillFails verifies that when
// webhook_auth_api_key_value is unknown but webhook_auth_api_key_name is null, the null
// name is still flagged. Per-field unknown checks allow validation of known fields.
func TestValidateConfig_APIKey_UnknownValue_NullName_StillFails(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:            types.StringValue("login"),
		Timing:          types.StringValue("after"),
		URL:             types.StringValue("https://example.com/webhook"),
		WebhookAuthType: types.StringValue("api_key"),
		// WebhookAuthAPIKeyName is null — should still fail even though value is unknown
		WebhookAuthAPIKeyValue: types.StringUnknown(), // sourced from data source
		WebhookAuthAPIKeyIn:    types.StringValue("header"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error for null webhook_auth_api_key_name even when value is unknown")
}

// TestValidateConfig_BasicAuth_UnknownPassword_NullUser_StillFails verifies that when
// webhook_auth_basic_auth_password is unknown but webhook_auth_basic_auth_user is null,
// the null user is still flagged.
func TestValidateConfig_BasicAuth_UnknownPassword_NullUser_StillFails(t *testing.T) {
	r := &ActionResource{}
	ctx := context.Background()

	req := buildActionTestConfig(t, ActionResourceModel{
		Flow:            types.StringValue("registration"),
		Timing:          types.StringValue("after"),
		URL:             types.StringValue("https://example.com/webhook"),
		WebhookAuthType: types.StringValue("basic_auth"),
		// WebhookAuthBasicAuthUser is null — should still fail even though password is unknown
		WebhookAuthBasicAuthPassword: types.StringUnknown(), // sourced from data source
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error for null webhook_auth_basic_auth_user even when password is unknown")
}
