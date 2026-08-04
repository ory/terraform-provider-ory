package projectconfig

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratedPatchEntries_ValidAndUnique verifies that every generated patch
// entry has a non-empty path, non-nil field pointer, and that all paths are
// unique (no two entries write to the same config location).
func TestGeneratedPatchEntries_ValidAndUnique(t *testing.T) {
	plan := &ProjectConfigResourceModel{}
	seen := make(map[string]bool)

	checkEntry := func(typeName, path string, field interface{}) {
		t.Helper()
		assert.NotEmpty(t, path, "%s patch entry has empty path", typeName)
		if path == "" {
			return
		}
		assert.NotNil(t, field, "%s patch entry %q has nil field pointer", typeName, path)
		assert.False(t, seen[path], "%s patch entry %q: duplicate path (already seen)", typeName, path)
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
		require.True(t, strings.HasPrefix(path, "/"), "patch path %q does not start with /", path)
		valid := false
		for _, prefix := range validPrefixes {
			if strings.HasPrefix(path, prefix) {
				valid = true
				break
			}
		}
		assert.True(t, valid, "patch path %q does not match any known service prefix", path)
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
		assert.False(t, generatedPaths[p.Path], "unexpected patch for null generated field: %s", p.Path)
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
	require.NotNil(t, p, "expected patch for selfservice_flows_login_lifespan")
	assert.Equal(t, "replace", p.Op)
	assert.Equal(t, "30m0s", p.Value)
}

// TestGeneratedBoolPatch_SetValue verifies a set bool field produces correct patch.
func TestGeneratedBoolPatch_SetValue(t *testing.T) {
	r := &ProjectConfigResource{}
	plan := &ProjectConfigResourceModel{
		FeatureFlagsCacheableSessions: types.BoolValue(true),
	}

	patches := r.buildPatches(context.Background(), plan)
	p := findPatch(patches, "/services/identity/config/feature_flags/cacheable_sessions")
	require.NotNil(t, p, "expected patch for feature_flags_cacheable_sessions")
	assert.Equal(t, true, p.Value)
}

// TestGeneratedReadEntries_AllFieldsExist verifies read entries reference real struct fields.
func TestGeneratedReadEntries_AllFieldsExist(t *testing.T) {
	state := &ProjectConfigResourceModel{}

	checkStringEntries := func(name string, entries []StringReadEntry) {
		for _, e := range entries {
			assert.NotNil(t, e.Field, "%s: string read entry with keys %v has nil field", name, e.Keys)
			assert.NotEmpty(t, e.Keys, "%s: string read entry has empty keys", name)
		}
	}
	checkBoolEntries := func(name string, entries []BoolReadEntry) {
		for _, e := range entries {
			assert.NotNil(t, e.Field, "%s: bool read entry with keys %v has nil field", name, e.Keys)
		}
	}
	checkInt64Entries := func(name string, entries []Int64ReadEntry) {
		for _, e := range entries {
			assert.NotNil(t, e.Field, "%s: int64 read entry with keys %v has nil field", name, e.Keys)
		}
	}
	checkListStringEntries := func(name string, entries []ListStringReadEntry) {
		for _, e := range entries {
			assert.NotNil(t, e.Field, "%s: list_string read entry with keys %v has nil field", name, e.Keys)
			assert.NotEmpty(t, e.Keys, "%s: list_string read entry has empty keys", name)
		}
	}
	checkMapStringEntries := func(name string, entries []MapStringReadEntry) {
		for _, e := range entries {
			assert.NotNil(t, e.Field, "%s: map_string read entry with keys %v has nil field", name, e.Keys)
			assert.NotEmpty(t, e.Keys, "%s: map_string read entry has empty keys", name)
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
	assert.Equal(t, "30m0s", state.SelfserviceFlowsLoginLifespan.ValueString(), "login lifespan")
	assert.Equal(t, "code", state.SelfserviceFlowsRecoveryUse.ValueString(), "recovery use")
	assert.True(t, state.SelfserviceFlowsRecoveryNotifyUnknownRecipients.ValueBool(), "recovery notify_unknown_recipients")
	assert.True(t, state.FeatureFlagsCacheableSessions.ValueBool(), "cacheable_sessions")
}

// TestSMTPConnectionURI_WriteOnly verifies the SMTP connection URI is treated as
// write-only: it is sent on create/update (it remains in the schema and patch
// tables) but is never read back from the API. A value the API returns — empty,
// a masked sentinel such as "****", or a partially-masked URI — must never
// overwrite the configured value, otherwise every plan would show a perpetual
// diff between the real URI in HCL and the masked value in state.
//
// This is the provider-side guard for the Ory API stripping SMTP credentials from
// project-config responses.
func TestSMTPConnectionURI_WriteOnly(t *testing.T) {
	// A realistic SMTP URI whose credentials must never be clobbered by the API.
	const configured = "smtps://user:secret@smtp.example.com:465" //nolint:gosec // G101 false positive: test fixture, not a real credential

	// 1. It must be excluded from the generated read entries entirely.
	state := &ProjectConfigResourceModel{
		SMTPConnectionURI: types.StringValue(configured),
	}
	for _, e := range identityStringReadEntries(state) {
		assert.NotSame(t, &state.SMTPConnectionURI, e.Field,
			"smtp_connection_uri must not appear in read entries — it is write-only")
	}

	// 2. readSimpleFields must preserve the configured value regardless of what the
	//    API returns for courier.smtp.connection_uri (absent, empty, or masked).
	for _, apiValue := range []string{"", "****", "smtps://user:masked@smtp.example.com:465"} {
		st := &ProjectConfigResourceModel{
			SMTPConnectionURI: types.StringValue(configured),
		}
		project := projectWithIdentityConfig(map[string]interface{}{
			"courier": map[string]interface{}{
				"smtp": map[string]interface{}{
					"connection_uri": apiValue,
				},
			},
		})
		readSimpleFields(context.Background(), project, st)
		assert.Equal(t, configured, st.SMTPConnectionURI.ValueString(),
			"configured smtp_connection_uri must be preserved when the API returns %q", apiValue)
	}
}

// TestCourierHTTPRequestConfigAuth_WriteOnly verifies the flat HTTP courier auth
// secrets (basic-auth password and API-key value) are treated as write-only: they
// are sent on create/update (they remain in the schema and patch tables) but are
// never read back from the API. A value the API returns — empty or a masked
// sentinel such as "****" — must never overwrite the configured value, otherwise
// every plan would show a perpetual diff between the real secret in HCL and the
// masked value in state.
//
// This is the provider-side guard for the Ory API stripping HTTP courier
// credentials from project-config responses.
func TestCourierHTTPRequestConfigAuth_WriteOnly(t *testing.T) {
	const (
		configuredPassword = "basic-auth-secret" //nolint:gosec // G101 false positive: test fixture, not a real credential
		configuredAPIKey   = "api-key-secret"    //nolint:gosec // G101 false positive: test fixture, not a real credential
	)

	// 1. Both must be excluded from the generated read entries entirely.
	state := &ProjectConfigResourceModel{
		CourierHTTPRequestConfigAuthBasicAuthPassword: types.StringValue(configuredPassword),
		CourierHTTPRequestConfigAuthAPIKeyValue:       types.StringValue(configuredAPIKey),
	}
	for _, e := range identityStringReadEntries(state) {
		assert.NotSame(t, &state.CourierHTTPRequestConfigAuthBasicAuthPassword, e.Field,
			"courier_http_request_config_auth_basic_auth_password must not appear in read entries — it is write-only")
		assert.NotSame(t, &state.CourierHTTPRequestConfigAuthAPIKeyValue, e.Field,
			"courier_http_request_config_auth_api_key_value must not appear in read entries — it is write-only")
	}

	// 2. readSimpleFields must preserve the configured values regardless of what
	//    the API returns for the auth config secrets (absent, empty, or masked).
	for _, apiValue := range []string{"", "****"} {
		st := &ProjectConfigResourceModel{
			CourierHTTPRequestConfigAuthBasicAuthPassword: types.StringValue(configuredPassword),
			CourierHTTPRequestConfigAuthAPIKeyValue:       types.StringValue(configuredAPIKey),
		}
		project := projectWithIdentityConfig(map[string]interface{}{
			"courier": map[string]interface{}{
				"http": map[string]interface{}{
					"request_config": map[string]interface{}{
						"auth": map[string]interface{}{
							"config": map[string]interface{}{
								"password": apiValue,
								"value":    apiValue,
							},
						},
					},
				},
			},
		})
		readSimpleFields(context.Background(), project, st)
		assert.Equal(t, configuredPassword, st.CourierHTTPRequestConfigAuthBasicAuthPassword.ValueString(),
			"configured courier basic-auth password must be preserved when the API returns %q", apiValue)
		assert.Equal(t, configuredAPIKey, st.CourierHTTPRequestConfigAuthAPIKeyValue.ValueString(),
			"configured courier api-key value must be preserved when the API returns %q", apiValue)
	}
}

// TestGeneratedSchemaAttributes_Count verifies generation produced a non-trivial
// number of attributes. The AllOptional and ValidAndUnique tests verify correctness;
// this test catches catastrophic generation failures (e.g., empty output).
func TestGeneratedSchemaAttributes_Count(t *testing.T) {
	attrs := simpleSchemaAttributes()
	require.NotEmpty(t, attrs, "simpleSchemaAttributes() returned 0 attributes — generation likely failed")
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
			assert.Failf(t, "unexpected attribute type", "attribute %q has unexpected type %T", name, attr)
			continue
		}
		assert.True(t, optional, "attribute %q is not optional", name)
	}
}

// TestGeneratedReadEntries_MissingKeySemantics mechanically pins the
// missing-key behavior for EVERY generated read entry, so any codegen change
// and any future attribute inherits the issue #321 semantics automatically:
//
//   - The API prunes empty values (empty string/list/map, integer zero) from
//     stored configs, so a missing key matches an empty state value and the
//     value is kept — otherwise the attribute diffs on every plan and no
//     apply can settle it.
//   - A non-empty state value still nulls on a missing key (unless the entry
//     preserves on missing), so genuine out-of-band removals surface as drift.
func TestGeneratedReadEntries_MissingKeySemantics(t *testing.T) {
	emptyServicesProject := func() *ory.Project {
		return &ory.Project{Services: ory.ProjectServices{
			Identity:          &ory.ProjectServiceIdentity{Config: map[string]interface{}{}},
			Oauth2:            &ory.ProjectServiceOAuth2{Config: map[string]interface{}{}},
			Permission:        &ory.ProjectServicePermission{Config: map[string]interface{}{}},
			AccountExperience: &ory.ProjectServiceAccountExperience{Config: map[string]interface{}{}},
		}}
	}

	stringEntries := func(state *ProjectConfigResourceModel) []StringReadEntry {
		all := identityStringReadEntries(state)
		all = append(all, oauth2StringReadEntries(state)...)
		all = append(all, permissionStringReadEntries(state)...)
		all = append(all, account_experienceStringReadEntries(state)...)
		return all
	}
	boolEntries := func(state *ProjectConfigResourceModel) []BoolReadEntry {
		all := identityBoolReadEntries(state)
		all = append(all, oauth2BoolReadEntries(state)...)
		all = append(all, account_experienceBoolReadEntries(state)...)
		return all
	}
	int64Entries := func(state *ProjectConfigResourceModel) []Int64ReadEntry {
		all := identityInt64ReadEntries(state)
		all = append(all, oauth2Int64ReadEntries(state)...)
		return all
	}
	listEntries := func(state *ProjectConfigResourceModel) []ListStringReadEntry {
		all := identityListStringReadEntries(state)
		all = append(all, oauth2ListStringReadEntries(state)...)
		all = append(all, permissionListStringReadEntries(state)...)
		all = append(all, account_experienceListStringReadEntries(state)...)
		return all
	}
	mapEntries := func(state *ProjectConfigResourceModel) []MapStringReadEntry {
		all := identityMapStringReadEntries(state)
		all = append(all, account_experienceMapStringReadEntries(state)...)
		return all
	}

	t.Run("empty state values survive a missing key", func(t *testing.T) {
		state := &ProjectConfigResourceModel{}
		emptyList, diags := types.ListValueFrom(context.Background(), types.StringType, []string{})
		require.False(t, diags.HasError())
		emptyMap, diags := types.MapValue(types.StringType, map[string]attr.Value{})
		require.False(t, diags.HasError())

		for _, e := range stringEntries(state) {
			*e.Field = types.StringValue("")
		}
		for _, e := range int64Entries(state) {
			*e.Field = types.Int64Value(0)
		}
		for _, e := range listEntries(state) {
			*e.Field = emptyList
		}
		for _, e := range mapEntries(state) {
			*e.Field = emptyMap
		}
		// Bools have no empty value; the drift branch below covers them.

		readSimpleFields(context.Background(), emptyServicesProject(), state)

		for _, e := range stringEntries(state) {
			assert.False(t, e.Field.IsNull(), "string entry %v: empty value must survive a missing key", e.Keys)
		}
		for _, e := range int64Entries(state) {
			assert.False(t, e.Field.IsNull(), "int64 entry %v: zero must survive a missing key", e.Keys)
		}
		for _, e := range listEntries(state) {
			assert.True(t, e.Field.Equal(emptyList), "list entry %v: empty list must survive a missing key", e.Keys)
		}
		for _, e := range mapEntries(state) {
			assert.True(t, e.Field.Equal(emptyMap), "map entry %v: empty map must survive a missing key", e.Keys)
		}
	})

	t.Run("non-empty state values null on a missing key", func(t *testing.T) {
		state := &ProjectConfigResourceModel{}
		probeList, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"probe"})
		require.False(t, diags.HasError())
		probeMap, diags := types.MapValue(types.StringType, map[string]attr.Value{"probe": types.StringValue("v")})
		require.False(t, diags.HasError())

		for _, e := range stringEntries(state) {
			*e.Field = types.StringValue("probe")
		}
		for _, e := range boolEntries(state) {
			*e.Field = types.BoolValue(true)
		}
		for _, e := range int64Entries(state) {
			*e.Field = types.Int64Value(7)
		}
		for _, e := range listEntries(state) {
			*e.Field = probeList
		}
		for _, e := range mapEntries(state) {
			*e.Field = probeMap
		}

		readSimpleFields(context.Background(), emptyServicesProject(), state)

		for _, e := range stringEntries(state) {
			if e.PreserveOnMissing {
				assert.Equal(t, "probe", e.Field.ValueString(), "string entry %v: preserve-on-missing must keep state", e.Keys)
			} else {
				assert.True(t, e.Field.IsNull(), "string entry %v: non-empty value must null so removals surface as drift", e.Keys)
			}
		}
		for _, e := range boolEntries(state) {
			if e.PreserveOnMissing {
				assert.True(t, e.Field.ValueBool(), "bool entry %v: preserve-on-missing must keep state", e.Keys)
			} else {
				assert.True(t, e.Field.IsNull(), "bool entry %v: value must null so removals surface as drift", e.Keys)
			}
		}
		for _, e := range int64Entries(state) {
			if e.PreserveOnMissing {
				assert.EqualValues(t, 7, e.Field.ValueInt64(), "int64 entry %v: preserve-on-missing must keep state", e.Keys)
			} else {
				assert.True(t, e.Field.IsNull(), "int64 entry %v: non-zero value must null so removals surface as drift", e.Keys)
			}
		}
		for _, e := range listEntries(state) {
			if e.PreserveOnMissing {
				assert.True(t, e.Field.Equal(probeList), "list entry %v: preserve-on-missing must keep state", e.Keys)
			} else {
				assert.True(t, e.Field.IsNull(), "list entry %v: non-empty list must null so removals surface as drift", e.Keys)
			}
		}
		for _, e := range mapEntries(state) {
			if e.PreserveOnMissing {
				assert.True(t, e.Field.Equal(probeMap), "map entry %v: preserve-on-missing must keep state", e.Keys)
			} else {
				assert.True(t, e.Field.IsNull(), "map entry %v: non-empty map must null so removals surface as drift", e.Keys)
			}
		}
	})
}
