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
