package acctest

import (
	"errors"
	"testing"

	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verifyTestProject() *ory.Project {
	return &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{
				Config: map[string]interface{}{
					"courier": map[string]interface{}{
						"templates": map[string]interface{}{
							"recovery_code": map[string]interface{}{
								"valid": map[string]interface{}{
									"email": map[string]interface{}{"subject": "Recover"},
								},
							},
						},
					},
					"selfservice": map[string]interface{}{
						"methods": map[string]interface{}{
							"oidc": map[string]interface{}{
								"enabled": true,
								"config": map[string]interface{}{
									"providers": []interface{}{map[string]interface{}{"id": "google"}},
								},
							},
						},
					},
				},
			},
			Oauth2: &ory.ProjectServiceOAuth2{
				Config: map[string]interface{}{
					"ttl": map[string]interface{}{"access_token": "30m"},
				},
			},
		},
	}
}

func TestProjectConfigValue_ResolvesNestedPaths(t *testing.T) {
	project := verifyTestProject()

	value, ok := projectConfigValue(project,
		"/services/identity/config/courier/templates/recovery_code/valid/email/subject")
	require.True(t, ok)
	assert.Equal(t, "Recover", value)

	value, ok = projectConfigValue(project,
		"/services/identity/config/selfservice/methods/oidc/enabled")
	require.True(t, ok)
	assert.Equal(t, true, value)

	value, ok = projectConfigValue(project, "/services/oauth2/config/ttl/access_token")
	require.True(t, ok)
	assert.Equal(t, "30m", value)
}

// TestProjectConfigValue_MissingPathReportsFalse covers what a delete assertion
// depends on: an absent key must report false rather than fail, since the API
// prunes empty values and removes whole nodes.
func TestProjectConfigValue_MissingPathReportsFalse(t *testing.T) {
	project := verifyTestProject()

	for _, path := range []string{
		"/services/identity/config/courier/templates/recovery_code/valid/email/body",
		"/services/identity/config/courier/templates/verification_code",
		"/services/identity/config/selfservice/methods/saml",
		"/services/oauth2/config/ttl/refresh_token",
	} {
		t.Run(path, func(t *testing.T) {
			_, ok := projectConfigValue(project, path)
			assert.False(t, ok, "an absent path must report false")
		})
	}
}

// TestProjectConfigValue_RejectsNonServicePaths guards against a typo in a patch
// path silently reporting "absent" and making a delete assertion pass for the
// wrong reason.
func TestProjectConfigValue_RejectsNonServicePaths(t *testing.T) {
	project := verifyTestProject()

	for _, path := range []string{
		"",
		"/",
		"/services",
		"/services/identity",
		"/services/identity/selfservice",
		"/selfservice/methods/oidc",
		"/services/unknown/config/x",
	} {
		_, ok := projectConfigValue(project, path)
		assert.Falsef(t, ok, "path %q is not a service config path", path)
	}
}

func TestProjectConfigValue_AbsentServiceReportsFalse(t *testing.T) {
	project := &ory.Project{Services: ory.ProjectServices{}}

	_, ok := projectConfigValue(project, "/services/identity/config/selfservice")
	assert.False(t, ok, "a project with no identity service must report false")

	_, ok = projectConfigValue(project, "/services/oauth2/config/ttl")
	assert.False(t, ok, "a project with no oauth2 service must report false")
}

func TestEventually_RetriesUntilSuccess(t *testing.T) {
	calls := 0
	err := Eventually(func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestEventually_ReturnsLastErrorWhenBudgetRunsOut(t *testing.T) {
	if testing.Short() {
		t.Skip("the retry budget takes ~30s of wall clock")
	}
	want := errors.New("still wrong")
	err := Eventually(func() error { return want })
	assert.Equal(t, want, err)
}
