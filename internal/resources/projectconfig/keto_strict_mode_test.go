package projectconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKetoStrictModeLocked(t *testing.T) {
	tests := []struct {
		name       string
		revision   map[string]interface{}
		planned    bool
		wantStored bool
		wantLocked bool
	}{
		{
			name:     "no readonly flag is writable",
			revision: map[string]interface{}{ketoStrictModeKey: true},
			planned:  false,
		},
		{
			name:     "readonly false is writable",
			revision: map[string]interface{}{ketoStrictModeReadonlyKey: false, ketoStrictModeKey: true},
			planned:  false,
		},
		{
			name:       "locked and planned matches stored is a no-op write",
			revision:   map[string]interface{}{ketoStrictModeReadonlyKey: true, ketoStrictModeKey: true},
			planned:    true,
			wantStored: true,
		},
		{
			name:       "locked and planned differs from stored is blocked",
			revision:   map[string]interface{}{ketoStrictModeReadonlyKey: true, ketoStrictModeKey: true},
			planned:    false,
			wantStored: true,
			wantLocked: true,
		},
		{
			name:       "locked without a stored value keeps false",
			revision:   map[string]interface{}{ketoStrictModeReadonlyKey: true},
			planned:    true,
			wantLocked: true,
		},
		{
			name:     "empty revision is writable",
			revision: map[string]interface{}{},
			planned:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored, locked := ketoStrictModeLocked(tt.revision, tt.planned)
			assert.Equal(t, tt.wantStored, stored, "stored")
			assert.Equal(t, tt.wantLocked, locked, "locked")
		})
	}
}

// normalizedRevisionServer serves GET /normalized/projects/{id} with the given
// current revision and records how many requests it saw.
func normalizedRevisionServer(t *testing.T, currentRevision string) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/normalized/projects/proj-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"proj-1","current_revision":` + currentRevision + `}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCheckKetoStrictModeWritable_UnsetAttributeSkipsTheRead(t *testing.T) {
	srv, calls := normalizedRevisionServer(t, `{"keto_strict_mode_readonly":true,"keto_feature_flags_strict_mode":true}`)
	r := projectConfigResourceForServer(t, srv.URL)

	diags := r.checkKetoStrictModeWritable(context.Background(), "proj-1", &ProjectConfigResourceModel{})

	assert.False(t, diags.HasError())
	assert.Equal(t, 0, *calls, "an unset attribute must not cost a console request")
}

func TestCheckKetoStrictModeWritable_LockedMismatchFails(t *testing.T) {
	srv, calls := normalizedRevisionServer(t, `{"keto_strict_mode_readonly":true,"keto_feature_flags_strict_mode":true}`)
	r := projectConfigResourceForServer(t, srv.URL)
	plan := &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(false)}

	diags := r.checkKetoStrictModeWritable(context.Background(), "proj-1", plan)

	require.True(t, diags.HasError())
	assert.Equal(t, 1, *calls)
	assert.Equal(t, "Keto strict mode is locked on this project", diags.Errors()[0].Summary())
	assert.Contains(t, diags.Errors()[0].Detail(), "keeps the stored value true, so Terraform cannot set it to false")
}

func TestCheckKetoStrictModeWritable_LockedMatchPasses(t *testing.T) {
	srv, _ := normalizedRevisionServer(t, `{"keto_strict_mode_readonly":true,"keto_feature_flags_strict_mode":true}`)
	r := projectConfigResourceForServer(t, srv.URL)
	plan := &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(true)}

	diags := r.checkKetoStrictModeWritable(context.Background(), "proj-1", plan)

	assert.False(t, diags.HasError())
}

func TestCheckKetoStrictModeWritable_UnlockedPasses(t *testing.T) {
	srv, _ := normalizedRevisionServer(t, `{"keto_strict_mode_readonly":null,"keto_feature_flags_strict_mode":null}`)
	r := projectConfigResourceForServer(t, srv.URL)
	plan := &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(true)}

	diags := r.checkKetoStrictModeWritable(context.Background(), "proj-1", plan)

	assert.False(t, diags.HasError())
}

func TestCheckKetoStrictModeWritable_ReadFailureLetsTheWriteThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"message":"boom"}}`))
	}))
	t.Cleanup(srv.Close)
	r := projectConfigResourceForServer(t, srv.URL)
	plan := &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(false)}

	diags := r.checkKetoStrictModeWritable(context.Background(), "proj-1", plan)

	assert.False(t, diags.HasError(), "a failed lock read must not block the apply")
}
