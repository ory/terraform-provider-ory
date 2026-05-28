package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGovernsToPatchPath(t *testing.T) {
	tests := []struct {
		property string
		desc     string
		wantPath string
		wantOK   bool
	}{
		{"kratos_session_lifespan", `This governs the "session.lifespan" setting.`, "/services/identity/config/session/lifespan", true},
		{"hydra_ttl_access_token", `This governs the "ttl.access_token" setting.`, "/services/oauth2/config/ttl/access_token", true},
		{"keto_namespace_configuration", `This governs the "namespaces.location" setting.`, "/services/permission/config/namespaces/location", true},
		{"account_experience_favicon_dark", `This governs the "favicon_dark" setting.`, "/services/account_experience/config/favicon_dark", true},
		{"disable_account_experience_welcome_screen", `This governs the "disable_welcome_screen" setting.`, "/services/account_experience/config/disable_welcome_screen", true},
		{"enable_ax_v2", `This governs the "enable_ax_v2" setting.`, "/services/account_experience/config/enable_ax_v2", true},
		// Underscore within key preserved
		{"kratos_selfservice_default_browser_return_url", `This governs the "selfservice.default_browser_return_url" setting.`, "/services/identity/config/selfservice/default_browser_return_url", true},
		// Nested dots
		{"hydra_oauth2_grant_jwt_max_ttl", `This governs the "oauth2.grant.jwt.max_ttl" setting.`, "/services/oauth2/config/oauth2/grant/jwt/max_ttl", true},
		// No governs
		{"kratos_session_lifespan", "Session lifespan setting.", "", false},
		// Unknown prefix
		{"unknown_field", `This governs the "foo.bar" setting.`, "", false},
	}

	for _, tt := range tests {
		path, ok := governsToPatchPath(tt.property, tt.desc)
		if !assert.Equal(t, tt.wantOK, ok, "governsToPatchPath(%q): ok", tt.property) {
			continue
		}
		assert.Equal(t, tt.wantPath, path, "governsToPatchPath(%q): path", tt.property)
	}
}

func TestCleanDescription(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`Configures the session lifespan. This governs the "session.lifespan" setting.`, "Session lifespan."},
		{`Configures whether PKCE is enforced. This governs the "oauth2.pkce.enforced" setting.`, "PKCE is enforced."},
		{"Ory Kratos Session Lifespan", "Session Lifespan"},
		{"Ory Hydra Token TTL", "Token TTL"},
		{"Mid-sentence Ory Kratos reference stays", "Mid-sentence Ory Kratos reference stays"},
		{"A very long description that exceeds two hundred characters and should be truncated at the first sentence boundary. This is the second sentence that should not appear in the output because it is too long.", "A very long description that exceeds two hundred characters and should be truncated at the first sentence boundary"},
		{`This governs the "session.lifespan" setting.`, "Configuration setting."},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, cleanDescription(tt.input), "cleanDescription(%q)", tt.input)
	}
}

func TestDeriveTerraformName(t *testing.T) {
	tests := []struct {
		openapi string
		want    string
	}{
		{"kratos_session_lifespan", "session_lifespan"},
		{"hydra_ttl_access_token", "oauth2_ttl_access_token"},
		{"hydra_oauth2_pkce_enforced", "oauth2_pkce_enforced"},
		{"hydra_oidc_dynamic_client_registration_enabled", "oidc_dynamic_client_registration_enabled"},
		{"keto_namespace_configuration", "keto_namespace_configuration"},
		{"account_experience_favicon_dark", "account_experience_favicon_dark"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, deriveTerraformName(tt.openapi), "deriveTerraformName(%q)", tt.openapi)
	}
}

func TestAppendToMappingsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mappings.yaml")
	original := "attributes:\n  - name: foo\n    go_field: Foo\n    type: bool\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600), "seed")
	require.NoError(t, appendToMappingsFile(path, "\n  - name: bar\n    go_field: Bar\n"), "append")
	got, err := os.ReadFile(path) //nolint:gosec // test file
	require.NoError(t, err, "read")
	want := "attributes:\n  - name: foo\n    go_field: Foo\n    type: bool\n\n  - name: bar\n    go_field: Bar\n"
	assert.Equal(t, want, string(got))
}

func TestAppendToMappingsFileHandlesMissingTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mappings.yaml")
	require.NoError(t, os.WriteFile(path, []byte("attributes:\n  - name: foo"), 0o600), "seed")
	require.NoError(t, appendToMappingsFile(path, "\n  - name: bar\n"), "append")
	got, err := os.ReadFile(path) //nolint:gosec // test file
	require.NoError(t, err, "read")
	assert.True(t, strings.HasPrefix(string(got), "attributes:\n  - name: foo\n\n"),
		"expected single-newline separator, got %q", got)
}

func TestInsertStructFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource.go")
	src := `package foo

type ProjectConfigResourceModel struct {
	ID types.String ` + "`tfsdk:\"id\"`" + `
}

func other() {}
`
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600), "seed")
	fields := []string{
		"\tNewField1 types.Bool `tfsdk:\"new_field_1\"`",
		"\tNewField2 types.String `tfsdk:\"new_field_2\"`",
	}
	require.NoError(t, insertStructFields(path, "ProjectConfigResourceModel", fields), "insert")
	got, err := os.ReadFile(path) //nolint:gosec // test file
	require.NoError(t, err, "read")
	content := string(got)
	assert.Contains(t, content, "\tID types.String `tfsdk:\"id\"`\n", "original field missing")
	assert.Contains(t, content, "NewField1 types.Bool", "NewField1 missing")
	assert.Contains(t, content, "NewField2 types.String", "NewField2 missing")
	assert.Contains(t, content, "\n}\n\nfunc other()", "closing brace or trailing content broken")
}

func TestInsertStructFieldsErrorsOnMissingStruct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resource.go")
	require.NoError(t, os.WriteFile(path, []byte("package foo\n"), 0o600), "seed")
	err := insertStructFields(path, "ProjectConfigResourceModel", []string{"\tX types.Bool"})
	require.Error(t, err, "expected error for missing struct")
}
