package socialprovider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// base_redirect_uri was removed in v26.4.0; it is managed by ory_project_config.
func TestSchema_HasNoDeprecatedAttributes(t *testing.T) {
	var resp resource.SchemaResponse
	(&SocialProviderResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	for name, attr := range resp.Schema.Attributes {
		assert.Empty(t, attr.GetDeprecationMessage(), name)
	}
	assert.NotContains(t, resp.Schema.Attributes, "base_redirect_uri")
}

func TestSchema_SubjectSourceRejectsUnknownValue(t *testing.T) {
	ctx := context.Background()
	var resp resource.SchemaResponse
	(&SocialProviderResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	attr, ok := resp.Schema.Attributes["subject_source"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, attr.Optional)
	assert.False(t, attr.Computed)
	for _, value := range []string{"userinfo", "me", "oid"} {
		t.Run(value, func(t *testing.T) {
			var validation validator.StringResponse
			for _, v := range attr.Validators {
				v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(value)}, &validation)
			}
			assert.False(t, validation.Diagnostics.HasError(), "%v", validation.Diagnostics)
		})
	}
	for name, value := range map[string]string{"id_token": "id_token", "upper_case": "ME", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			var validation validator.StringResponse
			for _, v := range attr.Validators {
				v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(value)}, &validation)
			}
			require.True(t, validation.Diagnostics.HasError())
		})
	}
}

func TestSchema_ClaimsSourceRejectsUnknownValue(t *testing.T) {
	ctx := context.Background()
	var resp resource.SchemaResponse
	(&SocialProviderResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	attr, ok := resp.Schema.Attributes["claims_source"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, attr.Optional)
	assert.False(t, attr.Computed)
	for _, value := range []string{"userinfo", "id_token"} {
		t.Run(value, func(t *testing.T) {
			var validation validator.StringResponse
			for _, v := range attr.Validators {
				v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(value)}, &validation)
			}
			assert.False(t, validation.Diagnostics.HasError(), "%v", validation.Diagnostics)
		})
	}
	for name, value := range map[string]string{"ID_TOKEN": "ID_TOKEN", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			var validation validator.StringResponse
			for _, v := range attr.Validators {
				v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(value)}, &validation)
			}
			require.True(t, validation.Diagnostics.HasError())
		})
	}
}
