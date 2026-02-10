package helpers

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveProjectID_PlanValue(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringValue("plan-id"), "client-id", &diags)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags.Errors())
	}
	if result != "plan-id" {
		t.Errorf("expected 'plan-id', got '%s'", result)
	}
}

func TestResolveProjectID_FallbackToClient(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringValue(""), "client-id", &diags)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags.Errors())
	}
	if result != "client-id" {
		t.Errorf("expected 'client-id', got '%s'", result)
	}
}

func TestResolveProjectID_NullPlanFallbackToClient(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringNull(), "client-id", &diags)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags.Errors())
	}
	if result != "client-id" {
		t.Errorf("expected 'client-id', got '%s'", result)
	}
}

func TestResolveProjectID_BothEmpty(t *testing.T) {
	var diags diag.Diagnostics
	result := ResolveProjectID(types.StringValue(""), "", &diags)
	if !diags.HasError() {
		t.Fatal("expected error when both plan and client ID are empty")
	}
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
	if diags.Errors()[0].Summary() != "Missing Project ID" {
		t.Errorf("expected 'Missing Project ID' error, got '%s'", diags.Errors()[0].Summary())
	}
}

func TestResolveProjectCreds_Valid(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("my-slug", "my-key", &diags)
	if !ok {
		t.Fatal("expected true for valid credentials")
	}
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags.Errors())
	}
}

func TestResolveProjectCreds_MissingSlug(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("", "my-key", &diags)
	if ok {
		t.Fatal("expected false when slug is empty")
	}
	if !diags.HasError() {
		t.Fatal("expected error when slug is empty")
	}
	if diags.Errors()[0].Summary() != "Missing Project Credentials" {
		t.Errorf("expected 'Missing Project Credentials' error, got '%s'", diags.Errors()[0].Summary())
	}
}

func TestResolveProjectCreds_MissingAPIKey(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("my-slug", "", &diags)
	if ok {
		t.Fatal("expected false when API key is empty")
	}
	if !diags.HasError() {
		t.Fatal("expected error when API key is empty")
	}
}

func TestResolveProjectCreds_BothEmpty(t *testing.T) {
	var diags diag.Diagnostics
	ok := ResolveProjectCreds("", "", &diags)
	if ok {
		t.Fatal("expected false when both are empty")
	}
	if !diags.HasError() {
		t.Fatal("expected error when both are empty")
	}
}
