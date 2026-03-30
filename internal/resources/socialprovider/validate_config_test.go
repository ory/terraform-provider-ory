package socialprovider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// buildTestConfig creates a ValidateConfigRequest from a SocialProviderResourceModel.
func buildTestConfig(t *testing.T, model SocialProviderResourceModel) resource.ValidateConfigRequest {
	t.Helper()

	r := &SocialProviderResource{}
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	vals := map[string]tftypes.Value{
		"id":                   tftypes.NewValue(tftypes.String, nil),
		"project_id":           tftypes.NewValue(tftypes.String, nil),
		"provider_id":          tftypes.NewValue(tftypes.String, model.ProviderID.ValueString()),
		"provider_type":        tfStringValue(model.ProviderType),
		"client_id":            tfStringValue(model.ClientID),
		"client_secret":        tfStringValue(model.ClientSecret),
		"issuer_url":           tftypes.NewValue(tftypes.String, nil),
		"scope":                tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"mapper_url":           tftypes.NewValue(tftypes.String, nil),
		"auth_url":             tftypes.NewValue(tftypes.String, nil),
		"token_url":            tftypes.NewValue(tftypes.String, nil),
		"tenant":               tftypes.NewValue(tftypes.String, nil),
		"apple_team_id":        tfStringValue(model.AppleTeamID),
		"apple_private_key_id": tfStringValue(model.ApplePrivateKeyID),
		"apple_private_key":    tfStringValue(model.ApplePrivateKey),
		"auto_link":            tfBoolValue(model.AutoLink),
		"label":                tfStringValue(model.Label),
		"account_linking_mode": tfStringValue(model.AccountLinkingMode),
		"base_redirect_uri":    tfStringValue(model.BaseRedirectURI),
	}

	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":                   tftypes.String,
			"project_id":           tftypes.String,
			"provider_id":          tftypes.String,
			"provider_type":        tftypes.String,
			"client_id":            tftypes.String,
			"client_secret":        tftypes.String,
			"issuer_url":           tftypes.String,
			"scope":                tftypes.List{ElementType: tftypes.String},
			"mapper_url":           tftypes.String,
			"auth_url":             tftypes.String,
			"token_url":            tftypes.String,
			"tenant":               tftypes.String,
			"apple_team_id":        tftypes.String,
			"apple_private_key_id": tftypes.String,
			"apple_private_key":    tftypes.String,
			"auto_link":            tftypes.Bool,
			"label":                tftypes.String,
			"account_linking_mode": tftypes.String,
			"base_redirect_uri":    tftypes.String,
		},
	}

	rawConfig := tftypes.NewValue(objType, vals)
	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    rawConfig,
	}

	return resource.ValidateConfigRequest{Config: config}
}

// tfBoolValue converts a types.Bool to a tftypes.Value, preserving null/unknown state.
func tfBoolValue(v types.Bool) tftypes.Value {
	if v.IsNull() {
		return tftypes.NewValue(tftypes.Bool, nil)
	}
	if v.IsUnknown() {
		return tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
	}
	return tftypes.NewValue(tftypes.Bool, v.ValueBool())
}

// tfStringValue converts a types.String to a tftypes.Value, preserving null/unknown state.
func tfStringValue(v types.String) tftypes.Value {
	if v.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}
	if v.IsUnknown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}
	return tftypes.NewValue(tftypes.String, v.ValueString())
}

func TestValidateConfig_UnknownClientSecret_SkipsValidation(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringValue("generic"),
		ClientID:     types.StringValue("my-client-id"),
		ClientSecret: types.StringUnknown(), // data source value — unknown at plan time
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown client_secret, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestValidateConfig_UnknownAppleFields_SkipsValidation(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:        types.StringValue("apple"),
		ProviderType:      types.StringValue("apple"),
		ClientID:          types.StringValue("com.example.app"),
		AppleTeamID:       types.StringUnknown(), // unknown from data source
		ApplePrivateKeyID: types.StringUnknown(),
		ApplePrivateKey:   types.StringUnknown(),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown apple fields, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestValidateConfig_KnownClientSecret_Passes(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringValue("generic"),
		ClientID:     types.StringValue("my-client-id"),
		ClientSecret: types.StringValue("my-secret"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for known client_secret, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
}

func TestValidateConfig_MissingClientSecret_Fails(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringValue("generic"),
		ClientID:     types.StringValue("my-client-id"),
		// ClientSecret not set (null) — should fail
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for missing client_secret, got none")
	}
}

func TestValidateConfig_EmptyClientSecret_Fails(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringValue("generic"),
		ClientID:     types.StringValue("my-client-id"),
		ClientSecret: types.StringValue(""), // empty string — should fail
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected error for empty client_secret, got none")
	}
}

func TestValidateConfig_UnknownProviderType_SkipsValidation(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringUnknown(), // unknown provider_type
		ClientID:     types.StringValue("my-client-id"),
		ClientSecret: types.StringValue("my-secret"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors for unknown provider_type, got: %s", resp.Diagnostics.Errors()[0].Detail())
	}
}
