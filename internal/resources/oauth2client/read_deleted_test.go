package oauth2client

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

// clientResourceForServer builds an OAuth2ClientResource whose project client
// points at the given test server, so Read can be exercised end-to-end against a
// fake Ory project API.
func clientResourceForServer(t *testing.T, srvURL string) *OAuth2ClientResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		ProjectAPIKey: "test-key",
		ProjectSlug:   "test-slug",
		// The URL template takes the slug; the test server ignores the path.
		ProjectAPIURL: srvURL + "/%s",
	})
	require.NoError(t, err)
	return &OAuth2ClientResource{client: c}
}

// oauth2ClientState returns a state object tracking a single OAuth2 client, as
// it would look after a successful create.
func oauth2ClientState(t *testing.T, r *OAuth2ClientResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	nullList := types.ListNull(types.StringType)
	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, OAuth2ClientResourceModel{
		ID:                     types.StringValue("client-id"),
		ClientID:               types.StringValue("client-id"),
		ClientName:             types.StringValue("test client"),
		GrantTypes:             nullList,
		ResponseTypes:          nullList,
		Audience:               nullList,
		RedirectURIs:           nullList,
		PostLogoutRedirectURIs: nullList,
		AllowedCorsOrigins:     nullList,
		Contacts:               nullList,
	})
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

// TestRead_ClientDeletedOutsideTerraform verifies that a client deleted through
// the Ory Console is dropped from state, so the next plan recreates it instead of
// failing with a read error that only `terraform state rm` can clear.
func TestRead_ClientDeletedOutsideTerraform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"Unable to locate the resource"}}`))
	}))
	defer srv.Close()

	r := clientResourceForServer(t, srv.URL)
	state := oauth2ClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted client must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted client must be removed from state")
}

// TestRead_ClientDeletedOAuth2ErrorBody covers the other error shape this
// endpoint can return: the SDK decodes it into its OAuth2 error model, so the
// error keeps its "404 Not Found" status text, where the Ory-shaped body fails
// to decode and is recognized by its "code":404 instead. Both shapes must lead
// to removal, whichever way the 404 is carried.
func TestRead_ClientDeletedOAuth2ErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found","error_description":"Unable to locate the resource"}`))
	}))
	defer srv.Close()

	r := clientResourceForServer(t, srv.URL)
	state := oauth2ClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted client must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted client must be removed from state")
}

// TestRead_ClientStillExists is the control: a live client must stay in state,
// proving the removal path does not fire while the client is alive.
func TestRead_ClientStillExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"client_id":"client-id","client_name":"test client","scope":"openid"}`))
	}))
	defer srv.Close()

	r := clientResourceForServer(t, srv.URL)
	state := oauth2ClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live client must stay in state")
}

// TestRead_ServerErrorKeepsClient verifies a non-404 failure still surfaces as an
// error and leaves state untouched: a transient outage must not be mistaken for a
// deletion and silently drop the resource.
func TestRead_ServerErrorKeepsClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","message":"boom"}}`))
	}))
	defer srv.Close()

	r := clientResourceForServer(t, srv.URL)
	state := oauth2ClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}

// A 500 whose body merely contains the digits "404" must not be mistaken for a
// deletion. Request IDs are hex strings, so they carry those digits about once
// every 150 errors, and IsNotFound's substring fallback matches on them. Dropping
// a live client from state over a transient outage is worse than the read error
// this change removes, so the removal path checks the HTTP status alone.
func TestRead_ServerErrorWith404InRequestIDKeepsClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","request":"bfd5c404-7133-9506-8a5f-f7a738b14e04","message":"boom"}}`))
	}))
	defer srv.Close()

	r := clientResourceForServer(t, srv.URL)
	state := oauth2ClientState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}
