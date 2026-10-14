package socialprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The whole point of copying the OIDC config is that callers work from cached
// project state: a shallow copy would let a caller mutate the cache and corrupt
// what the next operation in the same Terraform run reads back.
func TestCopyOIDCConfig_DeepCopiesNestedValues(t *testing.T) {
	original := map[string]interface{}{
		"base_redirect_uri": "https://iam.example.com",
		"providers": []interface{}{
			map[string]interface{}{"id": "google", "scope": []interface{}{"email"}},
		},
	}

	copied := copyOIDCConfig(original)
	require.NotNil(t, copied)
	assert.Equal(t, original, copied, "the copy must start out equal to the source")

	copied["base_redirect_uri"] = "https://mutated.example.com"
	copiedProviders, ok := copied["providers"].([]interface{})
	require.True(t, ok, "providers should survive the round-trip as []interface{}")
	firstProvider, ok := copiedProviders[0].(map[string]interface{})
	require.True(t, ok, "provider entries should survive as maps")
	firstProvider["id"] = "mutated"

	assert.Equal(t, "https://iam.example.com", original["base_redirect_uri"],
		"mutating the copy must not reach the source")
	originalProviders := original["providers"].([]interface{})
	assert.Equal(t, "google", originalProviders[0].(map[string]interface{})["id"],
		"mutating a nested provider in the copy must not reach the source")
}

func TestCopyOIDCConfig_NilStaysNil(t *testing.T) {
	assert.Nil(t, copyOIDCConfig(nil),
		"a project with no OIDC config must stay distinguishable from an empty one")
}

// providersFromOIDCConfig must always return a usable slice, because callers
// branch on len(providers) to decide between initializing the providers array
// and appending to it.
func TestProvidersFromOIDCConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantIDs []string
	}{
		{"nil config", nil, nil},
		{"no providers key", map[string]interface{}{"base_redirect_uri": "https://iam.example.com"}, nil},
		{"providers is not an array", map[string]interface{}{"providers": "google"}, nil},
		{"empty providers", map[string]interface{}{"providers": []interface{}{}}, nil},
		{
			"two providers",
			map[string]interface{}{"providers": []interface{}{
				map[string]interface{}{"id": "google"},
				map[string]interface{}{"id": "github"},
			}},
			[]string{"google", "github"},
		},
		{
			"non-object entries are skipped",
			map[string]interface{}{"providers": []interface{}{
				map[string]interface{}{"id": "google"},
				"github",
			}},
			[]string{"google"},
		},
	}

	for _, tt := range tests {
		got := providersFromOIDCConfig(tt.config)
		require.NotNil(t, got, "%s: must return an empty slice, never nil", tt.name)

		ids := make([]string, 0, len(got))
		for _, p := range got {
			id, _ := p["id"].(string)
			ids = append(ids, id)
		}
		if tt.wantIDs == nil {
			assert.Empty(t, ids, tt.name)
			continue
		}
		assert.Equal(t, tt.wantIDs, ids, tt.name)
	}
}
