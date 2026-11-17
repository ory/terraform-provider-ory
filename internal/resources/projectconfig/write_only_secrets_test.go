package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectConfigConfig returns a configuration with every attribute null except
// the given ones.
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
		wo, ok := schemaResp.Schema.Attributes[name+"_wo"]
		require.True(t, ok, name+"_wo")
		assert.True(t, wo.IsWriteOnly(), name+"_wo")
		assert.True(t, wo.IsSensitive(), name+"_wo")
		assert.True(t, wo.IsOptional(), name+"_wo")

		version, ok := schemaResp.Schema.Attributes[name+"_wo_version"]
		require.True(t, ok, name+"_wo_version")
		assert.False(t, version.IsWriteOnly(), name+"_wo_version")
		assert.False(t, version.IsSensitive(), name+"_wo_version")
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
	assert.Nil(t, findPatch(patches, "/services/identity/config/secrets/default"))
}

func TestAppendWriteOnlyIdentitySecretsPatches_NoneConfigured(t *testing.T) {
	ctx := context.Background()
	config := projectConfigConfig(t, nil)

	var diags diag.Diagnostics
	patches := (&ProjectConfigResource{}).appendWriteOnlyIdentitySecretsPatches(ctx, config, nil, &diags)
	require.False(t, diags.HasError(), diags)
	assert.Empty(t, patches)
}

// The stored and write-only forms of a secret list must not both be set, and a
// version trigger needs its write-only value.
func TestWriteOnlyIdentitySecretsValidators(t *testing.T) {
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	(&ProjectConfigResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	secrets, d := types.ListValueFrom(ctx, types.StringType, []string{"a-secret-value-of-sufficient-len"})
	require.False(t, d.HasError())
	empty, d := types.ListValueFrom(ctx, types.StringType, []string{})
	require.False(t, d.HasError())

	cases := map[string]struct {
		values  map[string]any
		attr    string
		wantErr bool
	}{
		"wo alone": {
			values: map[string]any{"identity_secrets_cookie_wo": secrets},
			attr:   "identity_secrets_cookie_wo",
		},
		"wo conflicts with stored": {
			values:  map[string]any{"identity_secrets_cookie_wo": secrets, "identity_secrets_cookie": secrets},
			attr:    "identity_secrets_cookie_wo",
			wantErr: true,
		},
		"wo must not be empty": {
			values:  map[string]any{"identity_secrets_cookie_wo": empty},
			attr:    "identity_secrets_cookie_wo",
			wantErr: true,
		},
		"version requires wo": {
			values:  map[string]any{"identity_secrets_cipher_wo_version": types.StringValue("1")},
			attr:    "identity_secrets_cipher_wo_version",
			wantErr: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			config := projectConfigConfig(t, tc.values)
			diags := validateAttribute(ctx, t, schemaResp.Schema.Attributes[tc.attr], config, tc.attr)
			assert.Equal(t, tc.wantErr, diags.HasError(), diags)
		})
	}
}

// validateAttribute runs the validators of one list or string attribute.
func validateAttribute(ctx context.Context, t *testing.T, a schema.Attribute, config tfsdk.Config, name string) diag.Diagnostics {
	t.Helper()
	p := path.Root(name)
	var diags diag.Diagnostics
	switch a := a.(type) {
	case schema.ListAttribute:
		var v types.List
		require.False(t, config.GetAttribute(ctx, p, &v).HasError())
		for _, val := range a.Validators {
			resp := &validator.ListResponse{}
			val.ValidateList(ctx, validator.ListRequest{Path: p, PathExpression: p.Expression(), Config: config, ConfigValue: v}, resp)
			diags.Append(resp.Diagnostics...)
		}
	case schema.StringAttribute:
		var v types.String
		require.False(t, config.GetAttribute(ctx, p, &v).HasError())
		for _, val := range a.Validators {
			resp := &validator.StringResponse{}
			val.ValidateString(ctx, validator.StringRequest{Path: p, PathExpression: p.Expression(), Config: config, ConfigValue: v}, resp)
			diags.Append(resp.Diagnostics...)
		}
	default:
		t.Fatalf("unsupported attribute type %T", a)
	}
	return diags
}
