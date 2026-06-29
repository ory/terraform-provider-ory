package socialprovider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
)

// buildTestConfig creates a ValidateConfigRequest from a SocialProviderResourceModel.
func buildTestConfig(t *testing.T, model SocialProviderResourceModel) resource.ValidateConfigRequest {
	t.Helper()

	r := &SocialProviderResource{}
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	vals := map[string]tftypes.Value{
		"id":                            tftypes.NewValue(tftypes.String, nil),
		"project_id":                    tftypes.NewValue(tftypes.String, nil),
		"provider_id":                   tftypes.NewValue(tftypes.String, model.ProviderID.ValueString()),
		"provider_type":                 tfStringValue(model.ProviderType),
		"client_id":                     tfStringValue(model.ClientID),
		"client_id_wo":                  tfStringValue(model.ClientIDWO),
		"client_id_wo_version":          tfStringValue(model.ClientIDWOVersion),
		"client_secret":                 tfStringValue(model.ClientSecret),
		"client_secret_wo":              tfStringValue(model.ClientSecretWO),
		"client_secret_wo_version":      tfStringValue(model.ClientSecretWOVersion),
		"issuer_url":                    tftypes.NewValue(tftypes.String, nil),
		"scope":                         tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"mapper_url":                    tftypes.NewValue(tftypes.String, nil),
		"auth_url":                      tftypes.NewValue(tftypes.String, nil),
		"token_url":                     tftypes.NewValue(tftypes.String, nil),
		"tenant":                        tftypes.NewValue(tftypes.String, nil),
		"apple_team_id":                 tfStringValue(model.AppleTeamID),
		"apple_private_key_id":          tfStringValue(model.ApplePrivateKeyID),
		"apple_private_key":             tfStringValue(model.ApplePrivateKey),
		"apple_private_key_wo":          tfStringValue(model.ApplePrivateKeyWO),
		"apple_private_key_wo_version":  tfStringValue(model.ApplePrivateKeyWOVersion),
		"auto_link":                     tfBoolValue(model.AutoLink),
		"label":                         tfStringValue(model.Label),
		"account_linking_mode":          tfStringValue(model.AccountLinkingMode),
		"base_redirect_uri":             tfStringValue(model.BaseRedirectURI),
		"additional_id_token_audiences": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"aal2_acr_values":               tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"aal2_amr_values":               tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"pkce":                          tfStringValue(model.Pkce),
		"fedcm_config_url":              tfStringValue(model.FedcmConfigURL),
	}

	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":                            tftypes.String,
			"project_id":                    tftypes.String,
			"provider_id":                   tftypes.String,
			"provider_type":                 tftypes.String,
			"client_id":                     tftypes.String,
			"client_id_wo":                  tftypes.String,
			"client_id_wo_version":          tftypes.String,
			"client_secret":                 tftypes.String,
			"client_secret_wo":              tftypes.String,
			"client_secret_wo_version":      tftypes.String,
			"issuer_url":                    tftypes.String,
			"scope":                         tftypes.List{ElementType: tftypes.String},
			"mapper_url":                    tftypes.String,
			"auth_url":                      tftypes.String,
			"token_url":                     tftypes.String,
			"tenant":                        tftypes.String,
			"apple_team_id":                 tftypes.String,
			"apple_private_key_id":          tftypes.String,
			"apple_private_key":             tftypes.String,
			"apple_private_key_wo":          tftypes.String,
			"apple_private_key_wo_version":  tftypes.String,
			"auto_link":                     tftypes.Bool,
			"label":                         tftypes.String,
			"account_linking_mode":          tftypes.String,
			"base_redirect_uri":             tftypes.String,
			"additional_id_token_audiences": tftypes.List{ElementType: tftypes.String},
			"aal2_acr_values":               tftypes.List{ElementType: tftypes.String},
			"aal2_amr_values":               tftypes.List{ElementType: tftypes.String},
			"pkce":                          tftypes.String,
			"fedcm_config_url":              tftypes.String,
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

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown client_secret: %v", resp.Diagnostics.Errors())
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

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown apple fields: %v", resp.Diagnostics.Errors())
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

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for known client_secret: %v", resp.Diagnostics.Errors())
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

	assert.True(t, resp.Diagnostics.HasError(), "expected error for missing client_secret")
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

	assert.True(t, resp.Diagnostics.HasError(), "expected error for empty client_secret")
}

func TestValidateConfig_EmptyFedcmConfigURL_Fails(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:     types.StringValue("google"),
		ProviderType:   types.StringValue("generic"),
		ClientID:       types.StringValue("my-client-id"),
		ClientSecret:   types.StringValue("my-secret"),
		FedcmConfigURL: types.StringValue(""), // empty string — should fail
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error for empty fedcm_config_url")
}

func TestValidateConfig_WriteOnlyClientSecret_Passes(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	// A non-Apple provider with only the write-only client_secret_wo set must
	// satisfy the "client_secret is required" rule.
	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:     types.StringValue("google"),
		ProviderType:   types.StringValue("generic"),
		ClientID:       types.StringValue("my-client-id"),
		ClientSecretWO: types.StringValue("my-wo-secret"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for write-only client_secret_wo: %v", resp.Diagnostics.Errors())
}

func TestValidateConfig_WriteOnlyClientID_Passes(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	// client_id supplied only via the write-only client_id_wo satisfies the
	// "client_id is required" rule.
	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringValue("generic"),
		ClientIDWO:   types.StringValue("my-wo-client-id"),
		ClientSecret: types.StringValue("my-secret"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for write-only client_id_wo: %v", resp.Diagnostics.Errors())
}

func TestValidateConfig_MissingClientID_Fails(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	// Neither client_id nor client_id_wo set — should fail.
	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:   types.StringValue("google"),
		ProviderType: types.StringValue("generic"),
		ClientSecret: types.StringValue("my-secret"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error when neither client_id nor client_id_wo is set")
}

func TestValidateConfig_EmptyWriteOnlyClientSecret_Fails(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:     types.StringValue("google"),
		ProviderType:   types.StringValue("generic"),
		ClientID:       types.StringValue("my-client-id"),
		ClientSecretWO: types.StringValue(""), // empty string — should fail
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.True(t, resp.Diagnostics.HasError(), "expected error for empty client_secret_wo")
}

func TestValidateConfig_UnknownWriteOnlyClientSecret_SkipsValidation(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	// An ephemeral resource may produce an unknown value at plan time.
	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:     types.StringValue("google"),
		ProviderType:   types.StringValue("generic"),
		ClientID:       types.StringValue("my-client-id"),
		ClientSecretWO: types.StringUnknown(),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown client_secret_wo: %v", resp.Diagnostics.Errors())
}

func TestValidateConfig_AppleWithWriteOnlyPrivateKey_Passes(t *testing.T) {
	r := &SocialProviderResource{}
	ctx := context.Background()

	// Apple provider using the write-only apple_private_key_wo plus the other two
	// Apple fields must satisfy the "all three Apple fields" rule.
	req := buildTestConfig(t, SocialProviderResourceModel{
		ProviderID:        types.StringValue("apple"),
		ProviderType:      types.StringValue("apple"),
		ClientID:          types.StringValue("com.example.app"),
		AppleTeamID:       types.StringValue("KP76DQS54M"),
		ApplePrivateKeyID: types.StringValue("UX56C66723"),
		ApplePrivateKeyWO: types.StringValue("-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----"),
	})
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, req, &resp)

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for Apple with apple_private_key_wo: %v", resp.Diagnostics.Errors())
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

	assert.False(t, resp.Diagnostics.HasError(), "expected no errors for unknown provider_type: %v", resp.Diagnostics.Errors())
}
