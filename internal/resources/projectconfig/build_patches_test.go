package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
)

func findPatch(patches []ory.JsonPatch, path string) *ory.JsonPatch {
	for i := range patches {
		if patches[i].Path == path {
			return &patches[i]
		}
	}
	return nil
}

func TestBuildPatches_EmptyDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringValue(""),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	if p == nil {
		t.Fatal("expected a patch for default_browser_return_url, got none")
	}
	if p.Op != "remove" {
		t.Errorf("expected op 'remove' for empty default_return_url, got %q", p.Op)
	}
}

func TestBuildPatches_NonEmptyDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringValue("https://app.example.com"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	if p == nil {
		t.Fatal("expected a patch for default_browser_return_url, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace' for non-empty default_return_url, got %q", p.Op)
	}
}

func TestBuildPatches_NullDefaultReturnURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		DefaultReturnURL: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/default_browser_return_url")
	if p != nil {
		t.Error("expected no patch for null default_return_url")
	}
}

func TestBuildPatches_EmptyAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	emptyList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{})
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: emptyList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	if p == nil {
		t.Fatal("expected a patch for allowed_return_urls, got none")
	}
	if p.Op != "remove" {
		t.Errorf("expected op 'remove' for empty allowed_return_urls, got %q", p.Op)
	}
}

func TestBuildPatches_NonEmptyAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	urlList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"https://app.example.com"})
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: urlList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	if p == nil {
		t.Fatal("expected a patch for allowed_return_urls, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace' for non-empty allowed_return_urls, got %q", p.Op)
	}
}

func TestBuildPatches_NullAllowedReturnURLs(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		AllowedReturnURLs: types.ListNull(types.StringType),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/allowed_return_urls")
	if p != nil {
		t.Error("expected no patch for null allowed_return_urls")
	}
}

func TestBuildPatches_OAuth2IssuerURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2IssuerURL: types.StringValue("https://auth.example.com"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/urls/self/issuer")
	if p == nil {
		t.Fatal("expected a patch for oauth2_issuer_url, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "https://auth.example.com" {
		t.Errorf("expected value 'https://auth.example.com', got %v", p.Value)
	}
}

func TestBuildPatches_OAuth2CookiesSameSiteMode(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2CookiesSameSiteMode: types.StringValue("Strict"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/serve/cookies/same_site_mode")
	if p == nil {
		t.Fatal("expected a patch for oauth2_cookies_same_site_mode, got none")
	}
	if p.Value != "Strict" {
		t.Errorf("expected value 'Strict', got %v", p.Value)
	}
}

func TestBuildPatches_OAuth2CookiesSameSiteLegacyWorkaround(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2CookiesSameSiteLegacyWorkaround: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/serve/cookies/same_site_legacy_workaround")
	if p == nil {
		t.Fatal("expected a patch for oauth2_cookies_same_site_legacy_workaround, got none")
	}
	if p.Value != true {
		t.Errorf("expected value true, got %v", p.Value)
	}
}

func TestBuildPatches_NullOAuth2IssuerURL(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		OAuth2IssuerURL: types.StringNull(),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/oauth2/config/urls/self/issuer")
	if p != nil {
		t.Error("expected no patch for null oauth2_issuer_url")
	}
}

func TestBuildPatches_WebhookHeaderAllowlist(t *testing.T) {
	r := &ProjectConfigResource{}
	headerList, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"Accept", "X-Custom-Header"})
	plan := &ProjectConfigResourceModel{
		WebhookHeaderAllowlist: headerList,
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/clients/web_hook/header_allowlist")
	if p == nil {
		t.Fatal("expected a patch for webhook_header_allowlist, got none")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
}

func TestBuildPatches_NullWebhookHeaderAllowlist(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		WebhookHeaderAllowlist: types.ListNull(types.StringType),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/clients/web_hook/header_allowlist")
	if p != nil {
		t.Error("expected no patch for null webhook_header_allowlist")
	}
}
