package organization

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// organizationResourceForServer builds an OrganizationResource whose console
// client points at the given test server. The consistency backoff is shortened
// for the duration of the test: GetOrganization retries a 404 five times, which
// would otherwise make each case wait fifteen seconds.
func organizationResourceForServer(t *testing.T, srvURL string) *OrganizationResource {
	t.Helper()
	t.Cleanup(client.SetOrganizationRetryBaseDelay(time.Millisecond))
	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: "ws-test-key",
		ConsoleAPIURL:   srvURL,
		ProjectID:       "proj-1",
	})
	require.NoError(t, err)
	return &OrganizationResource{client: c}
}

// organizationState returns a state object tracking a single organization, as it
// would look after a successful create.
func organizationState(t *testing.T, r *OrganizationResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, OrganizationResourceModel{
		ID:        types.StringValue("org-1"),
		Label:     types.StringValue("test org"),
		ProjectID: types.StringValue("proj-1"),
		Domains:   types.ListNull(types.StringType),
	})
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

func readOrganization(t *testing.T, srv *httptest.Server) *resource.ReadResponse {
	t.Helper()
	r := organizationResourceForServer(t, srv.URL)
	state := organizationState(t, r)
	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	return resp
}

// TestRead_OrganizationDeletedOutsideTerraform verifies that an organization
// deleted through the Ory Console is dropped from state once the consistency
// retries are exhausted, rather than failing every plan.
func TestRead_OrganizationDeletedOutsideTerraform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"organization not found"}}`))
	}))
	defer srv.Close()

	resp := readOrganization(t, srv)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted organization must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted organization must be removed from state")
}

// TestRead_OrganizationStillExists is the control: a live organization stays in
// state.
func TestRead_OrganizationStillExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"organization":{"id":"org-1","label":"test org","domains":[],"project_id":"proj-1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()

	resp := readOrganization(t, srv)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live organization must stay in state")
}

// TestRead_OrganizationServerErrorKeepsResource pins the direction of the gate:
// a 500 carrying "404" in its request ID must surface as an error, not read as a
// deletion. It also proves the 500 is not retried as a consistency lag.
func TestRead_OrganizationServerErrorKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","request":"bfd5c404-7133-9506-8a5f-f7a738b14e04","message":"boom"}}`))
	}))
	defer srv.Close()

	resp := readOrganization(t, srv)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}
