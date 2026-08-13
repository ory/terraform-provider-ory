package acctest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	ory "github.com/ory/client-go"
)

// Verification helpers for asserting what the Ory API actually stores, rather
// than what Terraform state records.
//
// Terraform state only holds what the provider believes it wrote. A Delete that
// silently no-ops, or that rewrites a shared config node and takes a sibling key
// with it, leaves state looking correct. Assertions that must catch that have to
// read the API back.

// eventualConsistency bounds how long a verification retries. GetProject is
// eventually consistent, so a single read straight after an apply or a destroy
// can still report the previous revision.
const (
	eventualConsistencyAttempts = 15
	eventualConsistencyInterval = 2 * time.Second
)

// Eventually retries check until it returns nil, and returns its last error if
// the budget runs out. Use it to wrap any assertion that reads project state
// back from the API right after a mutation.
func Eventually(check func() error) error {
	var err error
	for i := 0; i < eventualConsistencyAttempts; i++ {
		if i > 0 {
			time.Sleep(eventualConsistencyInterval)
		}
		if err = check(); err == nil {
			return nil
		}
	}
	return err
}

// ProjectConfigValue reads the value the project stores at a Console API JSON
// Patch path, for example
// "/services/identity/config/selfservice/methods/oidc/config/providers".
//
// The second return reports whether the path resolves. A missing key reports
// false rather than an error, because the API prunes empty values and removes
// whole nodes, which is exactly what a delete assertion needs to observe.
func ProjectConfigValue(t *testing.T, projectID, patchPath string) (interface{}, bool) {
	t.Helper()

	project, err := getProjectForVerify(t, projectID)
	if err != nil {
		t.Fatalf("could not read project %s: %v", projectID, err)
		return nil, false
	}
	return projectConfigValue(project, patchPath)
}

// ProjectState returns the lifecycle state the API reports for a project.
// Projects are soft deleted, so a deleted project still returns HTTP 200 with
// state "deleted" rather than a 404.
func ProjectState(t *testing.T, projectID string) (string, error) {
	t.Helper()

	project, err := getProjectForVerify(t, projectID)
	if err != nil {
		return "", err
	}
	return project.GetState(), nil
}

// WorkspaceExists reports whether the API still returns the workspace. Ory has no
// workspace delete endpoint, so ory_workspace has a no-op Delete and destroying it
// must leave the real workspace alone.
func WorkspaceExists(t *testing.T, workspaceID string) (bool, error) {
	t.Helper()

	c, err := GetOryClient()
	if err != nil {
		return false, fmt.Errorf("could not create client: %w", err)
	}
	workspace, err := c.GetWorkspace(context.Background(), workspaceID)
	if err != nil {
		return false, err
	}
	return workspace != nil && workspace.Id != "", nil
}

func getProjectForVerify(t *testing.T, projectID string) (*ory.Project, error) {
	t.Helper()

	c, err := GetOryClient()
	if err != nil {
		return nil, fmt.Errorf("could not create client: %w", err)
	}
	project, err := c.GetProject(context.Background(), projectID)
	if err != nil {
		return nil, fmt.Errorf("could not get project: %w", err)
	}
	return project, nil
}

// projectConfigValue walks a JSON Patch path into the right service config map.
// Split out from ProjectConfigValue so it can be unit tested without a client.
func projectConfigValue(project *ory.Project, patchPath string) (interface{}, bool) {
	segments := strings.Split(strings.TrimPrefix(patchPath, "/"), "/")
	// A service config path is /services/<service>/config/<key>...
	if len(segments) < 3 || segments[0] != "services" || segments[2] != "config" {
		return nil, false
	}

	// Kept as a concrete map rather than an interface so the nil check below is
	// real. Assigning a nil map to an interface yields a non-nil interface holding
	// a nil map, so an `interface{} == nil` check here would never fire.
	var config map[string]interface{}
	switch segments[1] {
	case "identity":
		if project.Services.Identity == nil {
			return nil, false
		}
		config = project.Services.Identity.Config
	case "oauth2":
		if project.Services.Oauth2 == nil {
			return nil, false
		}
		config = project.Services.Oauth2.Config
	case "permission":
		if project.Services.Permission == nil {
			return nil, false
		}
		config = project.Services.Permission.Config
	default:
		return nil, false
	}
	if config == nil {
		return nil, false
	}

	var current interface{} = config
	for _, segment := range segments[3:] {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
