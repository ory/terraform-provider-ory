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

// Each OneOf validator must match the enum the API enforces. The API rejects
// other values with HTTP 400, and accepts "" with HTTP 200 but drops the key.
func TestSchema_EnumAttributesRejectInvalidValues(t *testing.T) {
	ctx := context.Background()
	var resp resource.SchemaResponse
	(&SocialProviderResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	tests := map[string]struct{ valid, invalid []string }{
		"subject_source": {valid: []string{"userinfo", "me", "oid"}, invalid: []string{"id_token", "ME", ""}},
		"claims_source":  {valid: []string{"id_token", "userinfo"}, invalid: []string{"me", "USERINFO", ""}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			attr, ok := resp.Schema.Attributes[name].(schema.StringAttribute)
			require.True(t, ok)
			assert.True(t, attr.Optional)
			assert.False(t, attr.Computed)
			rejects := func(value string) bool {
				var validation validator.StringResponse
				for _, v := range attr.Validators {
					v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue(value)}, &validation)
				}
				return validation.Diagnostics.HasError()
			}
			for _, value := range tt.valid {
				assert.False(t, rejects(value), "%q must be accepted", value)
			}
			for _, value := range tt.invalid {
				assert.True(t, rejects(value), "%q must be rejected", value)
			}
		})
	}
}
