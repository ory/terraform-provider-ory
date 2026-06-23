package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// TestListWorkspaceIdentitySchemas covers the bootstrap strategy (issue #138):
// listing workspace-managed identity schemas with only a workspace API key.
func TestListWorkspaceIdentitySchemas(t *testing.T) {
	const wantHash = "d4581d88d1256a9526b342191fcb86976f1047a4c2cfb29cbef25bdde9af5ce386cc5feb291c406fd1a9e85e59b2bda722fd2917a5f588893b0ca1d6b0a79a4c"

	// Blob server (HTTPS) returns the actual schema body. fetchSchemaFromURL
	// requires https and rejects loopback, so we relax the host checker and use
	// the test server's TLS client for the duration of the test.
	blob := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"object","properties":{"email":{"type":"string"}}}`))
	}))
	defer blob.Close()

	origClient, origChecker := schemaFetchClient, hostChecker
	schemaFetchClient = blob.Client()
	hostChecker = func(context.Context, string) (bool, error) { return false, nil }
	defer func() { schemaFetchClient, hostChecker = origClient, origChecker }()

	t.Run("returns workspace schemas keyed by content hash", func(t *testing.T) {
		var gotAuth, gotPath string
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "11111111-1111-1111-1111-111111111111", "name": "customer", "content_hash": wantHash, "blob_url": blob.URL + "/schema"},
				{"id": "22222222-2222-2222-2222-222222222222", "name": "preset", "content_hash": "", "blob_url": ""},
			})
		}))
		defer api.Close()

		c, err := NewOryClient(OryClientConfig{
			WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
			ConsoleAPIURL:   api.URL,
		})
		if err != nil {
			t.Fatalf("NewOryClient: %v", err)
		}

		schemas, err := c.ListWorkspaceIdentitySchemas(context.Background())
		if err != nil {
			t.Fatalf("ListWorkspaceIdentitySchemas: %v", err)
		}

		if gotPath != "/identity-schemas" {
			t.Errorf("path = %q, want /identity-schemas", gotPath)
		}
		if gotAuth != "Bearer "+testutil.TestWorkspaceAPIKey {
			t.Errorf("auth header = %q, want bearer workspace key", gotAuth)
		}
		if len(schemas) != 2 {
			t.Fatalf("got %d schemas, want 2", len(schemas))
		}
		// The schema is keyed by its content hash (matches the issue #138 lookup).
		if schemas[0].GetId() != wantHash {
			t.Errorf("schema[0].id = %q, want content hash %q", schemas[0].GetId(), wantHash)
		}
		if props, _ := schemas[0].GetSchema()["properties"].(map[string]any); props["email"] == nil {
			t.Errorf("schema[0] body not fetched from blob_url: %v", schemas[0].GetSchema())
		}
		// content_hash empty -> falls back to the row ID.
		if schemas[1].GetId() != "22222222-2222-2222-2222-222222222222" {
			t.Errorf("schema[1].id = %q, want row id fallback", schemas[1].GetId())
		}
	})

	t.Run("surfaces API errors", func(t *testing.T) {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":401,"message":"Unauthorized"}}`))
		}))
		defer api.Close()

		c, err := NewOryClient(OryClientConfig{WorkspaceAPIKey: testutil.TestWorkspaceAPIKey, ConsoleAPIURL: api.URL})
		if err != nil {
			t.Fatalf("NewOryClient: %v", err)
		}
		if _, err := c.ListWorkspaceIdentitySchemas(context.Background()); err == nil {
			t.Fatal("expected error for 401 response")
		}
	})

	t.Run("requires a console client", func(t *testing.T) {
		c, err := NewOryClient(OryClientConfig{}) // no workspace API key
		if err != nil {
			t.Fatalf("NewOryClient: %v", err)
		}
		_, err = c.ListWorkspaceIdentitySchemas(context.Background())
		if err == nil {
			t.Fatal("expected error when console client is not configured")
		}
	})
}
