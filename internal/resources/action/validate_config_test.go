package action

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// buildActionTestConfig creates a ValidateConfigRequest from an ActionResourceModel.
func buildActionTestConfig(t *testing.T, model ActionResourceModel) resource.ValidateConfigRequest {
	t.Helper()

	r := &ActionResource{}
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	vals := map[string]tftypes.Value{
		"id":                               tftypes.NewValue(tftypes.String, nil),
		"project_id":                       tftypes.NewValue(tftypes.String, nil),
		"flow":                             tfActionStringValue(model.Flow),
		"timing":                           tfActionStringValue(model.Timing),
		"auth_method":                      tfActionStringValue(model.AuthMethod),
		"url":                              tfActionStringValue(model.URL),
		"method":                           tfActionStringValue(model.HTTPMethod),
		"body":                             tftypes.NewValue(tftypes.String, nil),
		"response_ignore":                  tftypes.NewValue(tftypes.Bool, nil),
		"response_parse":                   tftypes.NewValue(tftypes.Bool, nil),
		"can_interrupt":                    tftypes.NewValue(tftypes.Bool, nil),
		"webhook_auth_type":                tfActionStringValue(model.WebhookAuthType),
		"webhook_auth_basic_auth_user":     tfActionStringValue(model.WebhookAuthBasicAuthUser),
		"webhook_auth_basic_auth_password": tfActionStringValue(model.WebhookAuthBasicAuthPassword),
		"webhook_auth_api_key_name":        tfActionStringValue(model.WebhookAuthAPIKeyName),
		"webhook_auth_api_key_value":       tfActionStringValue(model.WebhookAuthAPIKeyValue),
		"webhook_auth_api_key_in":          tfActionStringValue(model.WebhookAuthAPIKeyIn),
	}

	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":                               tftypes.String,
			"project_id":                       tftypes.String,
			"flow":                             tftypes.String,
			"timing":                           tftypes.String,
			"auth_method":                      tftypes.String,
			"url":                              tftypes.String,
			"method":                           tftypes.String,
			"body":                             tftypes.String,
			"response_ignore":                  tftypes.Bool,
			"response_parse":                   tftypes.Bool,
			"can_interrupt":                    tftypes.Bool,
			"webhook_auth_type":                tftypes.String,
			"webhook_auth_basic_auth_user":     tftypes.String,
			"webhook_auth_basic_auth_password": tftypes.String,
			"webhook_auth_api_key_name":        tftypes.String,
			"webhook_auth_api_key_value":       tftypes.String,
			"webhook_auth_api_key_in":          tftypes.String,
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors when webhook auth is not configured, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for valid api_key config, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for missing webhook_auth_api_key_value, got none")
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown webhook_auth_api_key_value, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown webhook_auth_api_key_name, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown webhook_auth_api_key_in, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for valid basic_auth config, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for missing webhook_auth_basic_auth_password, got none")
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown webhook_auth_basic_auth_password, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
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

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown webhook_auth_basic_auth_user, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
}
