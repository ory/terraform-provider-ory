package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectConfigConfig returns a configuration with only the given attributes set.
func projectConfigConfig(t *testing.T, values map[string]any) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	(&ProjectConfigResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := projectConfigState(t, schemaResp.Schema)
	for name, value := range values {
		require.False(t, state.SetAttribute(ctx, path.Root(name), value).HasError(), name)
	}
	return tfsdk.Config{Schema: schemaResp.Schema, Raw: state.Raw}
}

func TestWriteOnlyIdentitySecretsSchema(t *testing.T) {
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	(&ProjectConfigResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	for _, name := range []string{"identity_secrets_cipher", "identity_secrets_cookie", "identity_secrets_default"} {
		wo := schemaResp.Schema.Attributes[name+"_wo"]
		require.NotNil(t, wo, name)
		assert.True(t, wo.IsWriteOnly() && wo.IsSensitive(), name)
		assert.Contains(t, schemaResp.Schema.Attributes, name+"_wo_version")
	}
}

func TestAppendWriteOnlyIdentitySecretsPatches(t *testing.T) {
	ctx := context.Background()
	cookie, d := types.ListValueFrom(ctx, types.StringType, []string{"new-cookie-secret-000000000000000", "old-cookie-secret-000000000000000"})
	require.False(t, d.HasError())
	cipher, d := types.ListValueFrom(ctx, types.StringType, []string{"cipher-secret-32-characters-0000"})
	require.False(t, d.HasError())

	config := projectConfigConfig(t, map[string]any{
		"identity_secrets_cookie_wo":         cookie,
		"identity_secrets_cookie_wo_version": types.StringValue("2"),
		"identity_secrets_cipher_wo":         cipher,
	})

	var diags diag.Diagnostics
	existing := []ory.JsonPatch{{Op: "replace", Path: "/services/identity/config/session/lifespan", Value: "24h"}}
	patches := (&ProjectConfigResource{}).appendWriteOnlyIdentitySecretsPatches(ctx, config, existing, &diags)
	require.False(t, diags.HasError(), diags)

	assert.Equal(t, []ory.JsonPatch{
		existing[0],
		{Op: "replace", Path: "/services/identity/config/secrets/cipher", Value: []string{"cipher-secret-32-characters-0000"}},
		{Op: "replace", Path: "/services/identity/config/secrets/cookie", Value: []string{"new-cookie-secret-000000000000000", "old-cookie-secret-000000000000000"}},
	}, patches)
}
