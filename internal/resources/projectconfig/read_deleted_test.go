package projectconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// projectConfigResourceForServer builds a ProjectConfigResource whose console
// client points at the given test server, so Read can be exercised end-to-end
// against a fake Ory Console.
func projectConfigResourceForServer(t *testing.T, srvURL string) *ProjectConfigResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: "ws-test-key",
		ConsoleAPIURL:   srvURL,
	})
	require.NoError(t, err)
	return &ProjectConfigResource{client: c}
}

// projectConfigState returns a state object holding only the two identifying
// attributes. The schema is generated and carries hundreds of attributes, so the
// object is built null and the two that Read needs are filled in.
func projectConfigState(t *testing.T, s schema.Schema) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	objType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	require.True(t, ok)

	values := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}

	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(objType, values)}
	require.False(t, state.SetAttribute(ctx, path.Root("id"), types.StringValue("proj-1")).HasError())
	require.False(t, state.SetAttribute(ctx, path.Root("project_id"), types.StringValue("proj-1")).HasError())
	return state
}

func readProjectConfig(t *testing.T, srv *httptest.Server) *resource.ReadResponse {
	t.Helper()
	ctx := context.Background()

	r := projectConfigResourceForServer(t, srv.URL)
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := projectConfigState(t, schemaResp.Schema)
	resp := &resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)
	return resp
}

// TestRead_ProjectConfigProjectPurged verifies that the config resource leaves
// state when the project it configures is gone. The config is not an object of
// its own: it lives inside the project, so a purged project takes it with it.
func TestRead_ProjectConfigProjectPurged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"project not found"}}`))
	}))
	defer srv.Close()

	resp := readProjectConfig(t, srv)

	assert.False(t, resp.Diagnostics.HasError(), "a purged project must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a purged project must take its config out of state")
}

// TestRead_ProjectConfigProjectAlive is the control: a live project keeps its
// config in state.
func TestRead_ProjectConfigProjectAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"running","services":{}}`))
	}))
	defer srv.Close()

	resp := readProjectConfig(t, srv)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live project must keep its config in state")
}

// TestRead_ProjectConfigServerErrorKeepsResource pins the direction of the gate:
// a 500 carrying "404" in its request ID must surface as an error, not a purge.
func TestRead_ProjectConfigServerErrorKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","request":"bfd5c404-7133-9506-8a5f-f7a738b14e04","message":"boom"}}`))
	}))
	defer srv.Close()

	resp := readProjectConfig(t, srv)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}

// TestRead_ProjectConfigProjectSoftDeleted covers what deleting a project in the
// console actually does. It is a soft delete: GetProject keeps answering 200 and
// only the state field changes, so the 404 path never fires and plan used to
// report no changes for the config of a project that is gone.
func TestRead_ProjectConfigProjectSoftDeleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r","organizations":[],"state":"deleted","services":{}}`))
	}))
	defer srv.Close()

	resp := readProjectConfig(t, srv)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted project must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted project must take its config out of state")
}
