package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

func TestResolveMaxRetries(t *testing.T) {
	tests := []struct {
		name      string
		tfValue   types.Int64
		envValue  string
		want      *int
		wantError bool
	}{
		{
			name:    "unset leaves the client default in place",
			tfValue: types.Int64Null(),
			want:    nil,
		},
		{
			name:    "the configured value wins",
			tfValue: types.Int64Value(9),
			want:    intPtr(9),
		},
		{
			name:    "zero turns the retry off",
			tfValue: types.Int64Value(0),
			want:    intPtr(0),
		},
		{
			name:     "the configured value wins over the environment",
			tfValue:  types.Int64Value(3),
			envValue: "7",
			want:     intPtr(3),
		},
		{
			name:     "the environment applies when nothing is configured",
			tfValue:  types.Int64Null(),
			envValue: "7",
			want:     intPtr(7),
		},
		{
			name:     "surrounding whitespace is trimmed",
			tfValue:  types.Int64Null(),
			envValue: "  4 ",
			want:     intPtr(4),
		},
		{
			name:      "a value that is not a number is an error",
			tfValue:   types.Int64Null(),
			envValue:  "lots",
			wantError: true,
		},
		{
			name:      "a negative value is an error",
			tfValue:   types.Int64Null(),
			envValue:  "-1",
			wantError: true,
		},
		{
			name:      "a value above the maximum is an error",
			tfValue:   types.Int64Null(),
			envValue:  "9999",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ORY_MAX_RETRIES", tt.envValue)

			var diags diag.Diagnostics
			got := resolveMaxRetries(tt.tfValue, &diags)

			assert.Equal(t, tt.wantError, diags.HasError(), "diagnostics: %v", diags)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestProviderSchema_MaxRetries(t *testing.T) {
	p := New("test")()
	resp := &provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError(), "schema: %v", resp.Diagnostics)

	attr, ok := resp.Schema.Attributes["max_retries"]
	require.True(t, ok, "the provider must expose max_retries")

	intAttr, ok := attr.(schema.Int64Attribute)
	require.True(t, ok)
	assert.True(t, intAttr.Optional)
	assert.False(t, intAttr.Required)
	assert.Len(t, intAttr.Validators, 1, "the value must be bounded")
	assert.Contains(t, intAttr.MarkdownDescription, "429 Too Many Requests")
	assert.Contains(t, intAttr.MarkdownDescription, "ORY_MAX_RETRIES")
}

func TestProviderSchema_MaxRetriesRejectsOutOfRange(t *testing.T) {
	p := New("test")()
	resp := &provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError())

	intAttr := resp.Schema.Attributes["max_retries"].(schema.Int64Attribute)
	validate := intAttr.Validators[0]

	for _, tc := range []struct {
		value     int64
		wantError bool
	}{
		{value: 0},
		{value: client.DefaultMaxRetries},
		{value: client.MaxRetriesUpperBound},
		{value: -1, wantError: true},
		{value: client.MaxRetriesUpperBound + 1, wantError: true},
	} {
		req := validator.Int64Request{ConfigValue: types.Int64Value(tc.value)}
		res := &validator.Int64Response{}
		validate.ValidateInt64(context.Background(), req, res)
		assert.Equal(t, tc.wantError, res.Diagnostics.HasError(), "value %d", tc.value)
	}
}

func intPtr(v int) *int { return &v }
