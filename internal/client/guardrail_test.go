package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildProjectSet(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string // expected keys (order-independent)
	}{
		{name: "nil input", in: nil, want: nil},
		{name: "empty slice", in: []string{}, want: nil},
		{name: "only empties and whitespace", in: []string{"", "  ", "\t"}, want: nil},
		{name: "trims whitespace", in: []string{" abc ", "def"}, want: []string{"abc", "def"}},
		{name: "dedupes", in: []string{"abc", "abc", "def"}, want: []string{"abc", "def"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProjectSet(tt.in)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Len(t, got, len(tt.want))
			for _, id := range tt.want {
				_, ok := got[id]
				assert.Truef(t, ok, "expected %q in set", id)
			}
		})
	}
}

func TestCheckProjectAllowed_NoAllowlist(t *testing.T) {
	// An empty allowlist is a no-op: any project ID is permitted.
	c := &OryClient{allowedProjects: buildProjectSet(nil)}
	assert.NoError(t, c.checkProjectAllowed("any-project-id"))
	assert.NoError(t, c.checkProjectAllowed(""))
}

func TestCheckProjectAllowed_Allowed(t *testing.T) {
	c := &OryClient{allowedProjects: buildProjectSet([]string{"proj-a", "proj-b"})}
	assert.NoError(t, c.checkProjectAllowed("proj-a"))
	assert.NoError(t, c.checkProjectAllowed("proj-b"))
}

func TestCheckProjectAllowed_Disallowed(t *testing.T) {
	c := &OryClient{allowedProjects: buildProjectSet([]string{"proj-b", "proj-a"})}
	err := c.checkProjectAllowed("proj-c")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrProjectNotAllowed)
	// The message names the offending project and lists the allowlist sorted.
	assert.Contains(t, err.Error(), "proj-c")
	idx := strings.Index(err.Error(), "proj-a")
	require.NotEqual(t, -1, idx)
	assert.Less(t, idx, strings.Index(err.Error(), "proj-b"), "allowlist should be sorted")
}

func TestNewOryClient_BuildsAllowedProjects(t *testing.T) {
	c, err := NewOryClient(OryClientConfig{AllowedProjectIDs: []string{"p1", " p2 "}})
	require.NoError(t, err)
	assert.NoError(t, c.checkProjectAllowed("p1"))
	assert.NoError(t, c.checkProjectAllowed("p2"))
	assert.ErrorIs(t, c.checkProjectAllowed("p3"), ErrProjectNotAllowed)
}

func TestWithProjectCredentials_CarriesAllowlist(t *testing.T) {
	parent, err := NewOryClient(OryClientConfig{AllowedProjectIDs: []string{"p1"}})
	require.NoError(t, err)
	child := parent.WithProjectCredentials("some-slug", "ory_pat_x")
	// The derived client must enforce the same allowlist.
	assert.NoError(t, child.checkProjectAllowed("p1"))
	assert.ErrorIs(t, child.checkProjectAllowed("p2"), ErrProjectNotAllowed)
}

func TestOryClientConfig_Equal(t *testing.T) {
	base := OryClientConfig{
		WorkspaceAPIKey:   "wak",
		ProjectAPIKey:     "pat",
		ProjectID:         "pid",
		ProjectSlug:       "slug",
		WorkspaceID:       "wid",
		ConsoleAPIURL:     "https://console",
		ProjectAPIURL:     "https://%s.projects",
		UserAgent:         "ua",
		AllowedProjectIDs: []string{"a", "b"},
	}

	same := base
	same.AllowedProjectIDs = []string{"a", "b"}
	assert.True(t, base.Equal(same))

	scalarDiff := base
	scalarDiff.ProjectID = "other"
	scalarDiff.AllowedProjectIDs = []string{"a", "b"}
	assert.False(t, base.Equal(scalarDiff))

	lenDiff := base
	lenDiff.AllowedProjectIDs = []string{"a"}
	assert.False(t, base.Equal(lenDiff))

	contentDiff := base
	contentDiff.AllowedProjectIDs = []string{"a", "c"}
	assert.False(t, base.Equal(contentDiff))

	// Order is significant: the provider normalizes ordering before comparison,
	// so a reordered slice is treated as a config change and rebuilds the client.
	orderDiff := base
	orderDiff.AllowedProjectIDs = []string{"b", "a"}
	assert.False(t, base.Equal(orderDiff))

	emptyBoth := OryClientConfig{}
	assert.True(t, emptyBoth.Equal(OryClientConfig{}))
}

// errorsIsSanity guards against accidental removal of the sentinel wrapping.
func TestErrProjectNotAllowed_Sentinel(t *testing.T) {
	c := &OryClient{allowedProjects: buildProjectSet([]string{"x"})}
	err := c.checkProjectAllowed("y")
	assert.True(t, errors.Is(err, ErrProjectNotAllowed))
}
