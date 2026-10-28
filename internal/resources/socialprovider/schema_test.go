package socialprovider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
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
