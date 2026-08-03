package oidcdynamicclient

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

// dynamicClientResourceForServer builds an OIDCDynamicClientResource whose
// project client points at the given test server, so Read can be exercised
// end-to-end against a fake Ory project API.
func dynamicClientResourceForServer(t *testing.T, srvURL string) *OIDCDynamicClientResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		ProjectAPIKey: "test-key",
		ProjectSlug:   "test-slug",
		// The URL template takes the slug; the test server ignores the path.
		ProjectAPIURL: srvURL + "/%s",
	})
	require.NoError(t, err)
	return &OIDCDynamicClientResource{client: c}
}

// dynamicClientState returns a state object tracking a single dynamic client, as
// it would look after a successful create.
func dynamicClientState(t *testing.T, r *OIDCDynamicClientResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	nullList := types.ListNull(types.StringType)
	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, OIDCDynamicClientResourceModel{
		ID:            types.StringValue("client-id"),
		ClientID:      types.StringValue("client-id"),
		ClientName:    types.StringValue("test client"),
		GrantTypes:    nullList,
		ResponseTypes: nullList,
		RedirectURIs:  nullList,
	})
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

// TestRead_DynamicClientDeletedOutsideTerraform verifies that a client deleted
// through the Ory Console is dropped from state, so the next plan recreates it
// instead of failing with a read error that only `terraform state rm` can clear.
func TestRead_DynamicClientDeletedOutsideTerraform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"Unable to locate the resource"}}`))
	}))
	defer srv.Close()

	r := dynamicClientResourceForServer(t, srv.URL)
	state := dynamicClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted client must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted client must be removed from state")
}

// TestRead_DynamicClientStillExists is the control: a live client must stay in
// state, proving the removal path does not fire while the client is alive.
func TestRead_DynamicClientStillExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"client_id":"client-id","client_name":"test client","scope":"openid"}`))
	}))
	defer srv.Close()

	r := dynamicClientResourceForServer(t, srv.URL)
	state := dynamicClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live client must stay in state")
}

// TestRead_DynamicClientServerErrorKeepsResource verifies a non-404 failure still
// surfaces as an error and leaves state untouched: a transient outage must not be
// mistaken for a deletion and silently drop the resource.
func TestRead_DynamicClientServerErrorKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","message":"boom"}}`))
	}))
	defer srv.Close()

	r := dynamicClientResourceForServer(t, srv.URL)
	state := dynamicClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}
