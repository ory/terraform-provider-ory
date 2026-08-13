package samlprovider

import (
	"testing"

	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// samlTestProject wraps a selfservice methods map in an *ory.Project.
func samlTestProject(methods map[string]interface{}) *ory.Project {
	return &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{
				Config: map[string]interface{}{
					"selfservice": map[string]interface{}{
						"methods": methods,
					},
				},
			},
		},
	}
}

// TestSAMLMethodPatches_ExistingNodeTargetsIndividualKeys is the regression for
// issue #332. When the SAML method node already exists, the patches must target
// its individual keys. A patch whose path is the method node itself replaces the
// whole object per RFC 6902, discarding every sibling key Ory keeps under it.
func TestSAMLMethodPatches_ExistingNodeTargetsIndividualKeys(t *testing.T) {
	patches := samlMethodPatches(true, false, []interface{}{})
	require.Len(t, patches, 2)

	for _, p := range patches {
		assert.NotEqualf(t, samlMethodPath, p.Path,
			"no patch may target the method node itself, it would discard sibling keys")
		assert.Equal(t, "add", p.Op, "an add on an existing member replaces just that member")
	}

	byPath := map[string]interface{}{}
	for _, p := range patches {
		byPath[p.Path] = p.Value
	}
	assert.Equal(t, false, byPath[samlMethodPath+"/enabled"])
	assert.Equal(t, []interface{}{}, byPath[samlMethodPath+"/config/providers"])
}

// TestSAMLMethodPatches_AbsentNodeIsWrittenWholesale verifies the one case where
// writing the whole node is correct: there is nothing under it to lose yet.
func TestSAMLMethodPatches_AbsentNodeIsWrittenWholesale(t *testing.T) {
	provider := map[string]interface{}{"id": "first"}
	patches := samlMethodPatches(false, true, []interface{}{provider})
	require.Len(t, patches, 1)

	assert.Equal(t, "add", patches[0].Op)
	assert.Equal(t, samlMethodPath, patches[0].Path)
	assert.Equal(t, map[string]interface{}{
		"enabled": true,
		"config": map[string]interface{}{
			"providers": []interface{}{provider},
		},
	}, patches[0].Value)
}

// TestSAMLMethodPatches_FirstProviderEnablesMethod covers the create side of the
// same defect: a project whose SAML method node exists with an empty providers
// array, which is the default state, must not have that node overwritten when the
// first provider is added.
func TestSAMLMethodPatches_FirstProviderEnablesMethod(t *testing.T) {
	provider := map[string]interface{}{"id": "first"}
	patches := samlMethodPatches(true, true, []interface{}{provider})
	require.Len(t, patches, 2)

	byPath := map[string]interface{}{}
	for _, p := range patches {
		require.NotEqual(t, samlMethodPath, p.Path)
		byPath[p.Path] = p.Value
	}
	assert.Equal(t, true, byPath[samlMethodPath+"/enabled"])
	assert.Equal(t, []interface{}{provider}, byPath[samlMethodPath+"/config/providers"])
}

// TestExtractSAMLConfigFromProject_DistinguishesAbsentFromEmpty verifies the
// signal Create branches on. An absent method node and a node holding an empty
// providers array both yield no providers, but only the first may be written
// wholesale, so the config map must be nil in one case and non-nil in the other.
func TestExtractSAMLConfigFromProject_DistinguishesAbsentFromEmpty(t *testing.T) {
	absent := samlTestProject(map[string]interface{}{})
	assert.Nil(t, extractSAMLConfigFromProject(absent), "an absent saml node must read as nil")
	assert.Empty(t, extractProvidersFromProject(absent))

	// The default shape on a real project: the node exists, providers is empty.
	empty := samlTestProject(map[string]interface{}{
		"saml": map[string]interface{}{
			"enabled": false,
			"config":  map[string]interface{}{"providers": []interface{}{}},
		},
	})
	assert.NotNil(t, extractSAMLConfigFromProject(empty), "an existing saml node must not read as nil")
	assert.Empty(t, extractProvidersFromProject(empty))
}

// TestProvidersFromSAMLConfig covers the shapes the API can return.
func TestProvidersFromSAMLConfig(t *testing.T) {
	assert.Empty(t, providersFromSAMLConfig(nil))
	assert.Empty(t, providersFromSAMLConfig(map[string]interface{}{}))

	providers := providersFromSAMLConfig(map[string]interface{}{
		"providers": []interface{}{
			map[string]interface{}{"id": "one"},
			"not-an-object",
			map[string]interface{}{"id": "two"},
		},
	})
	require.Len(t, providers, 2, "non-object entries are skipped")
	assert.Equal(t, "one", providers[0]["id"])
	assert.Equal(t, "two", providers[1]["id"])
}

// TestCopySAMLConfig verifies the cached config is not shared with callers.
func TestCopySAMLConfig(t *testing.T) {
	original := map[string]interface{}{
		"providers": []interface{}{map[string]interface{}{"id": "one"}},
	}
	cp := copySAMLConfig(original)
	require.NotNil(t, cp)

	providers, _ := cp["providers"].([]interface{})
	require.Len(t, providers, 1)
	entry, _ := providers[0].(map[string]interface{})
	entry["id"] = "mutated"

	assert.Equal(t, "one",
		original["providers"].([]interface{})[0].(map[string]interface{})["id"],
		"mutating the copy must not reach the original")
}
