package action

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// actionResourceWithConsole builds an ActionResource whose console client (used
// by GetProject) points at the given test server, so getHooks can be exercised
// end-to-end against a fake Ory Console.
func actionResourceWithConsole(t *testing.T, srvURL string) *ActionResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: "ws-test-key",
		ConsoleAPIURL:   srvURL,
	})
	require.NoError(t, err)
	return &ActionResource{client: c}
}

// liveProjectBody renders a 200 GetProject body for the given state that carries
// a single login/after/password web_hook, so tests can assert whether that hook
// is reported for a project in that state.
func liveProjectBody(id, state string) string {
	return `{
		"id":"` + id + `","name":"n","slug":"s","environment":"prod",
		"home_region":"eu-central","revision_id":"r","organizations":[],
		"state":"` + state + `",
		"services":{"identity":{"config":{"selfservice":{"flows":{"login":{"after":{"password":{"hooks":[
			{"hook":"web_hook","config":{"url":"https://example.com/x","method":"POST"}}
		]}}}}}}}}
	}`
}

// TestGetHooks_PurgedProjectReturnsEmpty verifies that when GetProject 404s
// (a fully purged project during teardown), getHooks reports no hooks instead of
// erroring, so Delete/Read can converge.
func TestGetHooks_PurgedProjectReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-404","message":"project not found"}}`))
	}))
	defer srv.Close()

	r := actionResourceWithConsole(t, srv.URL)
	hooks, err := r.getHooks(context.Background(), "purged-project", "login", "after", "password")
	require.NoError(t, err, "a 404 project must not surface as an error")
	assert.Empty(t, hooks, "a purged project must report no hooks")
}

// TestGetHooks_DeletedProjectReturnsEmpty verifies that a soft-deleted project
// (GetProject returns 200 with state "deleted") reports no hooks even when its
// config still carries one.
func TestGetHooks_DeletedProjectReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(liveProjectBody("soft-deleted", "deleted")))
	}))
	defer srv.Close()

	r := actionResourceWithConsole(t, srv.URL)
	hooks, err := r.getHooks(context.Background(), "soft-deleted", "login", "after", "password")
	require.NoError(t, err)
	assert.Empty(t, hooks, "a soft-deleted project must report no hooks even if config still holds one")
}

// TestGetHooks_LiveProjectReturnsHooks is the control: a live (running) project
// with a hook must still report it, proving the deleted-project handling does not
// change behavior while the project is alive.
func TestGetHooks_LiveProjectReturnsHooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(liveProjectBody("live", "running")))
	}))
	defer srv.Close()

	r := actionResourceWithConsole(t, srv.URL)
	hooks, err := r.getHooks(context.Background(), "live", "login", "after", "password")
	require.NoError(t, err)
	require.Len(t, hooks, 1, "a live project must still report its hook")
}
