package main

import "testing"

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
		if ok != tt.wantOK {
			t.Errorf("governsToPatchPath(%q): got ok=%v, want %v", tt.property, ok, tt.wantOK)
			continue
		}
		if path != tt.wantPath {
			t.Errorf("governsToPatchPath(%q): got %q, want %q", tt.property, path, tt.wantPath)
		}
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
		got := cleanDescription(tt.input)
		if got != tt.want {
			t.Errorf("cleanDescription(%q):\n  got  %q\n  want %q", tt.input, got, tt.want)
		}
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
		got := deriveTerraformName(tt.openapi)
		if got != tt.want {
			t.Errorf("deriveTerraformName(%q): got %q, want %q", tt.openapi, got, tt.want)
		}
	}
}
