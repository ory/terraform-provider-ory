package trustedjwtissuer

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

// issuerResourceForServer builds a TrustedJwtIssuerResource whose project client
// points at the given test server, so Read can be exercised end-to-end against a
// fake Ory project API.
func issuerResourceForServer(t *testing.T, srvURL string) *TrustedJwtIssuerResource {
	t.Helper()
	c, err := client.NewOryClient(client.OryClientConfig{
		ProjectAPIKey: "test-key",
		ProjectSlug:   "test-slug",
		// The URL template takes the slug; the test server ignores the path.
		ProjectAPIURL: srvURL + "/%s",
	})
	require.NoError(t, err)
	return &TrustedJwtIssuerResource{client: c}
}

// issuerState returns a state object tracking a single trusted issuer, as it
// would look after a successful create.
func issuerState(t *testing.T, r *TrustedJwtIssuerResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(ctx, TrustedJwtIssuerResourceModel{
		ID:     types.StringValue("issuer-id"),
		Issuer: types.StringValue("https://example.com"),
		Scope:  types.ListNull(types.StringType),
	})
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

// TestRead_IssuerDeletedOutsideTerraform verifies that an issuer deleted through
// the Ory Console is dropped from state, so the next plan recreates it instead of
// failing with a read error that only `terraform state rm` can clear.
func TestRead_IssuerDeletedOutsideTerraform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"status":"Not Found","request":"req-1","message":"Unable to locate the resource"}}`))
	}))
	defer srv.Close()

	r := issuerResourceForServer(t, srv.URL)
	state := issuerState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.False(t, resp.Diagnostics.HasError(), "a deleted issuer must not surface as an error: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a deleted issuer must be removed from state")
}

// TestRead_IssuerStillExists is the control: a live issuer must stay in state,
// proving the removal path does not fire while the issuer is alive.
func TestRead_IssuerStillExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"issuer-id","issuer":"https://example.com","subject":"sub","scope":["openid"]}`))
	}))
	defer srv.Close()

	r := issuerResourceForServer(t, srv.URL)
	state := issuerState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull(), "a live issuer must stay in state")
}

// TestRead_ServerErrorKeepsResource verifies a non-404 failure still surfaces as
// an error and leaves state untouched: a transient outage must not be mistaken
// for a deletion and silently drop the resource.
func TestRead_ServerErrorKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","message":"boom"}}`))
	}))
	defer srv.Close()

	r := issuerResourceForServer(t, srv.URL)
	state := issuerState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}

// Same guard as the OAuth2 client: a 500 whose body happens to contain "404"
// must not drop a live issuer from state.
func TestRead_ServerErrorWith404InRequestIDKeepsResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":500,"status":"Internal Server Error","request":"bfd5c404-7133-9506-8a5f-f7a738b14e04","message":"boom"}}`))
	}))
	defer srv.Close()

	r := issuerResourceForServer(t, srv.URL)
	state := issuerState(t, r)

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}
