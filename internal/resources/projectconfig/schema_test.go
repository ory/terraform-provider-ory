package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The deprecated attribute aliases were removed in v26.4.0. Nothing in the
// schema may carry a deprecation message, and the old names must be gone.
func TestSchema_HasNoDeprecatedAttributes(t *testing.T) {
	var resp resource.SchemaResponse
	(&ProjectConfigResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	for name, attr := range resp.Schema.Attributes {
		assert.Empty(t, attr.GetDeprecationMessage(), name)
	}
	for _, removed := range []string{"enable_password", "login_ui_url", "password_min_length", "smtp_from_address", "oauth2_access_token_lifespan"} {
		assert.NotContains(t, resp.Schema.Attributes, removed)
	}
	assert.Contains(t, resp.Schema.Attributes, "selfservice_methods_password_config_min_password_length")
}
