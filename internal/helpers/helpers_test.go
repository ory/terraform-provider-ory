package helpers

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveProjectID_PlanValue(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringValue("plan-id"), "client-id", &diags)
	require.False(t, diags.HasError(), "unexpected diagnostic errors: %v", diags.Errors())
	assert.Equal(t, "plan-id", result)
}

func TestResolveProjectID_FallbackToClient(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringValue(""), "client-id", &diags)
	require.False(t, diags.HasError(), "unexpected diagnostic errors: %v", diags.Errors())
	assert.Equal(t, "client-id", result)
}

func TestResolveProjectID_NullPlanFallbackToClient(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringNull(), "client-id", &diags)
	require.False(t, diags.HasError(), "unexpected diagnostic errors: %v", diags.Errors())
	assert.Equal(t, "client-id", result)
}

func TestResolveProjectID_BothEmpty(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringValue(""), "", &diags)
	require.True(t, diags.HasError(), "expected error when both plan and client ID are empty")
	assert.Empty(t, result)
	assert.Equal(t, "Missing Project ID", diags.Errors()[0].Summary())
}

func TestResolveProjectCreds_Valid(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("my-slug", "my-key", &diags)
	require.True(t, ok, "expected true for valid credentials")
	assert.False(t, diags.HasError(), "unexpected diagnostic errors: %v", diags.Errors())
}

func TestResolveProjectCreds_MissingSlug(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("", "my-key", &diags)
	require.False(t, ok, "expected false when slug is empty")
	require.True(t, diags.HasError(), "expected error when slug is empty")
	assert.Equal(t, "Missing Project Credentials", diags.Errors()[0].Summary())
}

func TestResolveProjectCreds_MissingAPIKey(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("my-slug", "", &diags)
	require.False(t, ok, "expected false when API key is empty")
	assert.True(t, diags.HasError(), "expected error when API key is empty")
}

func TestResolveProjectCreds_BothEmpty(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("", "", &diags)
	require.False(t, ok, "expected false when both are empty")
	assert.True(t, diags.HasError(), "expected error when both are empty")
}
