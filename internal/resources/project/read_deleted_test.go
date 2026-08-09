package project

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// projectResourceForServer builds a ProjectResource whose console client points
// at the given test server, so Read can be exercised end-to-end against a fake
// Ory Console.
func projectResourceForServer(t *testing.T, srvURL string) *ProjectResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: "ws-test-key",
		ConsoleAPIURL:   srvURL,
	})
	require.NoError(t, err)
	return &ProjectResource{client: c}
}

// projectState returns a state object tracking a single project, as it would
// look after a successful create.
func projectState(t *testing.T, r *ProjectResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, ProjectResourceModel{
		ID:   types.StringValue("proj-1"),
		Name: types.StringValue("test project"),
	})
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

// TestRead_ProjectPurgedOutsideTerraform verifies that a project purged through
// the Ory Console is dropped from state, so the next plan recreates it instead of
// failing every plan until the operator runs `terraform state rm`.
func TestRead_ProjectPurgedOutsideTerraform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"project not found"}}`))
	}))
	defer srv.Close()

	r := projectResourceForServer(t, srv.URL)
	state := projectState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.False(t, resp.Diagnostics.HasError(), "a purged project must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a purged project must be removed from state")
}

// TestRead_ProjectStillExists is the control: a live project must stay in state.
func TestRead_ProjectStillExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"proj-1","name":"test project","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"running","services":{}}`))
	}))
	defer srv.Close()

	r := projectResourceForServer(t, srv.URL)
	state := projectState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live project must stay in state")
}

// TestRead_ProjectServerErrorKeepsResource pins the direction of the removal
// gate: a 500 whose request ID happens to carry the digits "404" must surface as
// an error and leave state intact, never be mistaken for a purge.
func TestRead_ProjectServerErrorKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","request":"bfd5c404-7133-9506-8a5f-f7a738b14e04","message":"boom"}}`))
	}))
	defer srv.Close()

	r := projectResourceForServer(t, srv.URL)
	state := projectState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}
