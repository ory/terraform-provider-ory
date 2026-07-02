package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeStringList(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "all empty or whitespace", in: []string{"", "  ", "\t"}, want: nil},
		{name: "trims and drops empties", in: []string{" a ", "", "b"}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeStringList(tt.in))
		})
	}
}

func TestResolveStringList_FromConfig(t *testing.T) {
	var diags diag.Diagnostics
	list := types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue(" p1 "),
		types.StringValue("p2"),
		types.StringValue(""),
	})
	got := resolveAllowedProjectIDs(context.Background(), list, &diags)
	require.False(t, diags.HasError())
	assert.Equal(t, []string{"p1", "p2"}, got)
}

func TestResolveStringList_FromEnvFallback(t *testing.T) {
	var diags diag.Diagnostics
	t.Setenv("ORY_ALLOWED_PROJECT_IDS", " p1 , p2 ,,p3")
	got := resolveAllowedProjectIDs(context.Background(), types.ListNull(types.StringType), &diags)
	require.False(t, diags.HasError())
	assert.Equal(t, []string{"p1", "p2", "p3"}, got)
}

func TestResolveStringList_ConfigTakesPrecedenceOverEnv(t *testing.T) {
	var diags diag.Diagnostics
	t.Setenv("ORY_ALLOWED_PROJECT_IDS", "env-id")
	list := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("config-id")})
	got := resolveAllowedProjectIDs(context.Background(), list, &diags)
	require.False(t, diags.HasError())
	assert.Equal(t, []string{"config-id"}, got)
}

func TestResolveStringList_Unset(t *testing.T) {
	var diags diag.Diagnostics
	t.Setenv("ORY_ALLOWED_PROJECT_IDS", "")
	got := resolveAllowedProjectIDs(context.Background(), types.ListNull(types.StringType), &diags)
	require.False(t, diags.HasError())
	assert.Nil(t, got)
}
