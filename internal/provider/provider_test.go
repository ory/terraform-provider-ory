package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProvider(t *testing.T) {
	// Verify the provider can be instantiated
	p := New("test")()
	if p == nil {
		t.Fatal("provider should not be nil")
	}
}

func TestProviderMetadata(t *testing.T) {
	p := New("1.0.0")().(*OryProvider)

	req := provider.MetadataRequest{}
	resp := &provider.MetadataResponse{}

	p.Metadata(context.Background(), req, resp)

	if resp.TypeName != "ory" {
		t.Errorf("expected TypeName 'ory', got '%s'", resp.TypeName)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected Version '1.0.0', got '%s'", resp.Version)
	}
}

func TestResolveString(t *testing.T) {
	tests := []struct {
		name     string
		tfValue  types.String
		envVar   string
		envValue string
		expected string
	}{
		{
			name:     "returns terraform value when set",
			tfValue:  types.StringValue("tf-value"),
			envVar:   "TEST_VAR",
			envValue: "env-value",
			expected: "tf-value",
		},
		{
			name:     "returns env value when terraform value is null",
			tfValue:  types.StringNull(),
			envVar:   "TEST_VAR",
			envValue: "env-value",
			expected: "env-value",
		},
		{
			name:     "returns empty when both null and env not set",
			tfValue:  types.StringNull(),
			envVar:   "TEST_VAR_UNSET",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns env value when terraform value is unknown",
			tfValue:  types.StringUnknown(),
			envVar:   "TEST_VAR",
			envValue: "env-value",
			expected: "env-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			result := resolveString(tt.tfValue, tt.envVar)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestResolveStringDefault(t *testing.T) {
	tests := []struct {
		name         string
		tfValue      types.String
		envVar       string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "returns terraform value when set",
			tfValue:      types.StringValue("tf-value"),
			envVar:       "TEST_VAR",
			envValue:     "env-value",
			defaultValue: "default-value",
			expected:     "tf-value",
		},
		{
			name:         "returns env value when terraform value is null",
			tfValue:      types.StringNull(),
			envVar:       "TEST_VAR",
			envValue:     "env-value",
			defaultValue: "default-value",
			expected:     "env-value",
		},
		{
			name:         "returns default when both null and env not set",
			tfValue:      types.StringNull(),
			envVar:       "TEST_VAR_UNSET",
			envValue:     "",
			defaultValue: "default-value",
			expected:     "default-value",
		},
		{
			name:         "returns env value when terraform value is unknown",
			tfValue:      types.StringUnknown(),
			envVar:       "TEST_VAR",
			envValue:     "env-value",
			defaultValue: "default-value",
			expected:     "env-value",
		},
		{
			name:         "returns default when terraform unknown and env not set",
			tfValue:      types.StringUnknown(),
			envVar:       "TEST_VAR_UNSET",
			envValue:     "",
			defaultValue: DefaultConsoleAPIURL,
			expected:     DefaultConsoleAPIURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			result := resolveStringDefault(tt.tfValue, tt.envVar, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestProviderModelAttributes(t *testing.T) {
	// Verify the OryProviderModel has all expected fields
	// Using example URLs to demonstrate custom URL configuration
	model := OryProviderModel{
		WorkspaceAPIKey: types.StringValue("ory_wak_test"),
		ProjectAPIKey:   types.StringValue("ory_pat_test"),
		ProjectID:       types.StringValue("project-id"),
		ProjectSlug:     types.StringValue("project-slug"),
		WorkspaceID:     types.StringValue("workspace-id"),
		ConsoleAPIURL:   types.StringValue("https://api.console.example.com"),
		ProjectAPIURL:   types.StringValue("https://%s.projects.example.com"),
	}

	// Verify values can be retrieved
	if model.ConsoleAPIURL.ValueString() != "https://api.console.example.com" {
		t.Error("ConsoleAPIURL not set correctly")
	}
	if model.ProjectAPIURL.ValueString() != "https://%s.projects.example.com" {
		t.Error("ProjectAPIURL not set correctly")
	}
}
