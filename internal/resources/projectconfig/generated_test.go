package projectconfig

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestGeneratedPatchEntries_ValidAndUnique verifies that every generated patch
// entry has a non-empty path, non-nil field pointer, and that all paths are
// unique (no two entries write to the same config location).
func TestGeneratedPatchEntries_ValidAndUnique(t *testing.T) {
	plan := &ProjectConfigResourceModel{}
	seen := make(map[string]bool)

	checkEntry := func(typeName, path string, field interface{}) {
		t.Helper()
		if path == "" {
			t.Errorf("%s patch entry has empty path", typeName)
			return
		}
		if field == nil {
			t.Errorf("%s patch entry %q has nil field pointer", typeName, path)
		}
		if seen[path] {
			t.Errorf("%s patch entry %q: duplicate path (already seen)", typeName, path)
		}
		seen[path] = true
	}

	for _, e := range simpleStringPatchEntries(plan) {
		checkEntry("string", e.Path, e.Field)
	}
	for _, e := range simpleBoolPatchEntries(plan) {
		checkEntry("bool", e.Path, e.Field)
	}
	for _, e := range simpleInt64PatchEntries(plan) {
		checkEntry("int64", e.Path, e.Field)
	}
	for _, e := range simpleListStringPatchEntries(plan) {
		checkEntry("list_string", e.Path, e.Field)
	}
	for _, e := range simpleMapStringPatchEntries(plan) {
		checkEntry("map_string", e.Path, e.Field)
	}
}

// TestGeneratedPatchEntries_PathFormat verifies all patch paths have the expected format.
func TestGeneratedPatchEntries_PathFormat(t *testing.T) {
	plan := &ProjectConfigResourceModel{}

	validPrefixes := []string{
		"/services/identity/config/",
		"/services/oauth2/config/",
		"/services/permission/config/",
		"/services/account_experience/config/",
	}

	checkPath := func(path string) {
		t.Helper()
		if !strings.HasPrefix(path, "/") {
			t.Errorf("patch path %q does not start with /", path)
			return
		}
		valid := false
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(path, prefix) {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("patch path %q does not match any known service prefix", path)
		}
	}

	for _, e := range simpleStringPatchEntries(plan) {
		checkPath(e.Path)
	}
	for _, e := range simpleBoolPatchEntries(plan) {
		checkPath(e.Path)
	}
	for _, e := range simpleInt64PatchEntries(plan) {
		checkPath(e.Path)
	}
	for _, e := range simpleListStringPatchEntries(plan) {
		checkPath(e.Path)
	}
	for _, e := range simpleMapStringPatchEntries(plan) {
		checkPath(e.Path)
	}
}

// TestGeneratedStringPatch_NullSkipped verifies null generated fields produce no patch.
func TestGeneratedStringPatch_NullSkipped(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{} // all fields are zero-value (null)

	// Build set of all generated patch paths; none should appear in patches
	// when the plan is all-null.
	generatedPaths := make(map[string]bool)
	for _, e := range simpleStringPatchEntries(plan) {
		generatedPaths[e.Path] = true
	}
	for _, e := range simpleBoolPatchEntries(plan) {
		generatedPaths[e.Path] = true
	}
	for _, e := range simpleInt64PatchEntries(plan) {
		generatedPaths[e.Path] = true
	}
	for _, e := range simpleListStringPatchEntries(plan) {
		generatedPaths[e.Path] = true
	}
	for _, e := range simpleMapStringPatchEntries(plan) {
		generatedPaths[e.Path] = true
	}

	patches := r.buildPatches(context.Background(), plan)

	for _, p := range patches {
		if generatedPaths[p.Path] {
			t.Errorf("unexpected patch for null generated field: %s", p.Path)
		}
	}
}

// TestGeneratedStringPatch_SetValue verifies a set string field produces correct patch.
func TestGeneratedStringPatch_SetValue(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginLifespan: types.StringValue("30m0s"),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/selfservice/flows/login/lifespan")
	if p == nil {
		t.Fatal("expected patch for selfservice_flows_login_lifespan")
	}
	if p.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", p.Op)
	}
	if p.Value != "30m0s" {
		t.Errorf("expected value '30m0s', got %v", p.Value)
	}
}

// TestGeneratedBoolPatch_SetValue verifies a set bool field produces correct patch.
func TestGeneratedBoolPatch_SetValue(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		FeatureFlagsCacheableSessions: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/feature_flags/cacheable_sessions")
	if p == nil {
		t.Fatal("expected patch for feature_flags_cacheable_sessions")
	}
	if p.Value != true {
		t.Errorf("expected value true, got %v", p.Value)
	}
}

// TestGeneratedReadEntries_AllFieldsExist verifies read entries reference real struct fields.
func TestGeneratedReadEntries_AllFieldsExist(t *testing.T) {
	state := &ProjectConfigResourceModel{}

	checkStringEntries := func(name string, entries []StringReadEntry) {
		for _, e := range entries {
			if e.Field == nil {
				t.Errorf("%s: string read entry with keys %v has nil field", name, e.Keys)
			}
			if len(e.Keys) == 0 {
				t.Errorf("%s: string read entry has empty keys", name)
			}
		}
	}
	checkBoolEntries := func(name string, entries []BoolReadEntry) {
		for _, e := range entries {
			if e.Field == nil {
				t.Errorf("%s: bool read entry with keys %v has nil field", name, e.Keys)
			}
		}
	}
	checkInt64Entries := func(name string, entries []Int64ReadEntry) {
		for _, e := range entries {
			if e.Field == nil {
				t.Errorf("%s: int64 read entry with keys %v has nil field", name, e.Keys)
			}
		}
	}
	checkListStringEntries := func(name string, entries []ListStringReadEntry) {
		for _, e := range entries {
			if e.Field == nil {
				t.Errorf("%s: list_string read entry with keys %v has nil field", name, e.Keys)
			}
			if len(e.Keys) == 0 {
				t.Errorf("%s: list_string read entry has empty keys", name)
			}
		}
	}
	checkMapStringEntries := func(name string, entries []MapStringReadEntry) {
		for _, e := range entries {
			if e.Field == nil {
				t.Errorf("%s: map_string read entry with keys %v has nil field", name, e.Keys)
			}
			if len(e.Keys) == 0 {
				t.Errorf("%s: map_string read entry has empty keys", name)
			}
		}
	}

	checkStringEntries("identity", identityStringReadEntries(state))
	checkBoolEntries("identity", identityBoolReadEntries(state))
	checkInt64Entries("identity", identityInt64ReadEntries(state))
	checkListStringEntries("identity", identityListStringReadEntries(state))
	checkMapStringEntries("identity", identityMapStringReadEntries(state))
	checkStringEntries("oauth2", oauth2StringReadEntries(state))
	checkBoolEntries("oauth2", oauth2BoolReadEntries(state))
	checkListStringEntries("oauth2", oauth2ListStringReadEntries(state))
	checkStringEntries("permission", permissionStringReadEntries(state))
	checkListStringEntries("permission", permissionListStringReadEntries(state))
	checkStringEntries("account_experience", account_experienceStringReadEntries(state))
}

// TestGeneratedReadRoundTrip verifies that a value written via patch can be
// read back from the same config path structure.
func TestGeneratedReadRoundTrip(t *testing.T) {
	// Simulate an identity config with a value at a generated path
	identityConfig := map[string]interface{}{
		"selfservice": map[string]interface{}{
			"flows": map[string]interface{}{
				"login": map[string]interface{}{
					"lifespan": "30m0s",
				},
				"recovery": map[string]interface{}{
					"use":                       "code",
					"notify_unknown_recipients": true,
				},
			},
		},
		"feature_flags": map[string]interface{}{
			"cacheable_sessions": true,
		},
	}

	state := &ProjectConfigResourceModel{
		// Set non-null to enable read
		SelfserviceFlowsLoginLifespan:                   types.StringValue(""),
		SelfserviceFlowsRecoveryUse:                     types.StringValue(""),
		SelfserviceFlowsRecoveryNotifyUnknownRecipients: types.BoolValue(false),
		FeatureFlagsCacheableSessions:                   types.BoolValue(false),
	}

	// Read using generated entries
	for _, e := range identityStringReadEntries(state) {
		if !e.Field.IsNull() {
			if v, ok := getNestedString(identityConfig, e.Keys...); ok {
				if !e.SkipEmpty || v != "" {
					*e.Field = types.StringValue(v)
				}
			}
		}
	}
	for _, e := range identityBoolReadEntries(state) {
		if !e.Field.IsNull() {
			if v, ok := getNestedBool(identityConfig, e.Keys...); ok {
				*e.Field = types.BoolValue(v)
			}
		}
	}

	// Verify values were read correctly
	if state.SelfserviceFlowsLoginLifespan.ValueString() != "30m0s" {
		t.Errorf("expected login lifespan '30m0s', got %q", state.SelfserviceFlowsLoginLifespan.ValueString())
	}
	if state.SelfserviceFlowsRecoveryUse.ValueString() != "code" {
		t.Errorf("expected recovery use 'code', got %q", state.SelfserviceFlowsRecoveryUse.ValueString())
	}
	if state.SelfserviceFlowsRecoveryNotifyUnknownRecipients.ValueBool() != true {
		t.Error("expected recovery notify_unknown_recipients true")
	}
	if state.FeatureFlagsCacheableSessions.ValueBool() != true {
		t.Error("expected cacheable_sessions true")
	}
}

// TestGeneratedSchemaAttributes_Count verifies we have the expected number of generated attributes.
func TestGeneratedSchemaAttributes_Count(t *testing.T) {
	attrs := simpleSchemaAttributes()
	// 271 generated attributes: 218 primary + 53 deprecated aliases from mappings.yaml.
	// Use exact count to detect accidental additions/removals.
	const expected = 271
	if len(attrs) != expected {
		t.Errorf("expected %d generated schema attributes, got %d", expected, len(attrs))
	}
}

// TestGeneratedSchemaAttributes_AllOptional verifies all generated attributes are optional.
func TestGeneratedSchemaAttributes_AllOptional(t *testing.T) {
	attrs := simpleSchemaAttributes()
	for name, attr := range attrs {
		var optional bool
		switch a := attr.(type) {
		case schema.StringAttribute:
			optional = a.Optional
		case schema.BoolAttribute:
			optional = a.Optional
		case schema.Int64Attribute:
			optional = a.Optional
		case schema.ListAttribute:
			optional = a.Optional
		case schema.MapAttribute:
			optional = a.Optional
		default:
			t.Errorf("attribute %q has unexpected type %T", name, attr)
			continue
		}
		if !optional {
			t.Errorf("attribute %q is not optional", name)
		}
	}
}
