package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// workspaceResourceForServer builds a WorkspaceResource whose console client
// points at the given test server, so Read can be exercised end-to-end against a
// fake Ory Console.
func workspaceResourceForServer(t *testing.T, srvURL string) *WorkspaceResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: "ws-test-key",
		ConsoleAPIURL:   srvURL,
	})
	require.NoError(t, err)
	return &WorkspaceResource{client: c}
}

// workspaceState returns a state object tracking a single workspace, as it would
// look after a successful create.
func workspaceState(t *testing.T, r *WorkspaceResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, WorkspaceResourceModel{
		ID:   types.StringValue("ws-1"),
		Name: types.StringValue("test workspace"),
	})
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

func readWorkspace(t *testing.T, srv *httptest.Server) *resource.ReadResponse {
	t.Helper()
	r := workspaceResourceForServer(t, srv.URL)
	state := workspaceState(t, r)
	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	return resp
}

// TestRead_WorkspaceDeletedOutsideTerraform covers the direct path: GetWorkspace
// answers 404, so the workspace is gone and must leave state.
func TestRead_WorkspaceDeletedOutsideTerraform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"workspace not found"}}`))
	}))
	defer srv.Close()

	resp := readWorkspace(t, srv)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted workspace must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted workspace must be removed from state")
}

// TestRead_WorkspaceDeletedViaListFallback covers the other path into "gone".
// Some API keys may list workspaces but not GET one, so GetWorkspace falls back
// to ListWorkspaces on a 403. A workspace missing from that list is just as
// absent as a 404, and must leave state rather than fail every plan.
func TestRead_WorkspaceDeletedViaListFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.URL.Path, "/workspaces/") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":403,"status":"Forbidden","message":"no access"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"workspaces":[{"id":"other-ws","name":"other","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","subscription_id":null}],"has_next_page":false,"next_page_token":""}`))
	}))
	defer srv.Close()

	resp := readWorkspace(t, srv)

	assert.False(t, resp.Diagnostics.HasError(), "a workspace absent from the list must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a workspace absent from the list must be removed from state")
}

// TestRead_WorkspaceStillExists is the control: a live workspace stays in state.
func TestRead_WorkspaceStillExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ws-1","name":"test workspace","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z","subscription_id":null}`))
	}))
	defer srv.Close()

	resp := readWorkspace(t, srv)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live workspace must stay in state")
}

// TestRead_WorkspaceServerErrorKeepsResource pins the direction of the gate: a
// 500 carrying "404" in its request ID must surface as an error, not a deletion.
func TestRead_WorkspaceServerErrorKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","request":"bfd5c404-7133-9506-8a5f-f7a738b14e04","message":"boom"}}`))
	}))
	defer srv.Close()

	resp := readWorkspace(t, srv)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}
