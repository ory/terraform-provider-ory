package jwk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// Public EC keys with placeholder coordinates. The provider never checks key
// material, and the fake server below echoes whatever it gets.
const (
	keyA = `{"kty":"EC","crv":"P-256","kid":"key-a","use":"sig","alg":"ES256","x":"xa","y":"ya"}`
	keyB = `{"kty":"EC","crv":"P-256","kid":"key-b","use":"sig","alg":"ES256","x":"xb","y":"yb"}`
	keyC = `{"kty":"EC","crv":"P-256","kid":"key-c","use":"sig","alg":"ES256","x":"xc","y":"yc"}`
)

func keySet(keys ...string) string {
	return `{"keys":[` + strings.Join(keys, ",") + `]}`
}

// encoded returns the keys attribute value for a set, as Create, Update, and
// Read store it.
func encoded(t *testing.T, raw string) string {
	t.Helper()
	var set ory.JsonWebKeySet
	require.NoError(t, json.Unmarshal([]byte(raw), &set))
	keys, err := encodeKeySet(&set)
	require.NoError(t, err)
	return keys
}

func TestParseJWKS(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "one public key", raw: keySet(keyA)},
		{name: "keys for rotation", raw: keySet(keyA, keyB)},
		{name: "not JSON", raw: "not json", wantErr: "not a valid JSON Web Key Set"},
		{name: "array instead of a set", raw: "[" + keyA + "]", wantErr: "not a valid JSON Web Key Set"},
		{name: "single key instead of a set", raw: keyA, wantErr: "at least one key"},
		{name: "empty keys array", raw: `{"keys":[]}`, wantErr: "at least one key"},
		{name: "missing alg", raw: keySet(`{"kty":"EC","kid":"k","use":"sig"}`), wantErr: "required property alg"},
		{name: "empty use", raw: keySet(`{"kty":"EC","kid":"k","use":"","alg":"ES256"}`), wantErr: `key 0 in jwks has an empty "use" member`},
		{name: "empty kid", raw: keySet(keyA, `{"kty":"EC","kid":"","use":"sig","alg":"ES256"}`), wantErr: `key 1 in jwks has an empty "kid" member`},
		{name: "duplicate kid", raw: keySet(keyA, keyA), wantErr: `more than one key with kid "key-a"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseJWKS(tt.raw)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// Ory does not return the keys of a set in upload order. The keys attribute
// must not depend on either order.
func TestEncodeKeySet_SortsByKid(t *testing.T) {
	uploadOrder := encoded(t, keySet(keyB, keyA))
	readOrder := encoded(t, keySet(keyA, keyB))
	assert.Equal(t, readOrder, uploadOrder)
	assert.Less(t, strings.Index(uploadOrder, `"key-a"`), strings.Index(uploadOrder, `"key-b"`))
}

type recordedRequest struct {
	method, path, body string
}

// fakeKeysAPI serves GET and PUT on /admin/keys/{set}. GET returns getBody with
// getStatus, and PUT echoes the request body, as Ory does.
func fakeKeysAPI(t *testing.T, getStatus int, getBody string) (*JWKResource, *[]recordedRequest) {
	t.Helper()
	var requests []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, recordedRequest{method: r.Method, path: r.URL.Path, body: string(body)})
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(getStatus)
			_, _ = w.Write([]byte(getBody))
		case http.MethodPut:
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := client.NewOryClient(client.OryClientConfig{
		ProjectAPIKey: "test-key",
		ProjectSlug:   "test-slug",
		ProjectAPIURL: srv.URL + "/%s",
	})
	require.NoError(t, err)
	return &JWKResource{client: c}, &requests
}

func jwkState(t *testing.T, r *JWKResource, model JWKResourceModel) tfsdk.State {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := tfsdk.State{Schema: schemaResp.Schema}
	require.False(t, state.Set(context.Background(), model).HasError())
	return state
}

// suppliedModel is the state after Create uploaded jwks: the configured text is
// kept as is, and keys holds the stored set.
func suppliedModel(t *testing.T, jwks string) JWKResourceModel {
	return JWKResourceModel{
		ID:            types.StringValue("custom.session-token"),
		ProjectID:     types.StringNull(),
		ProjectSlug:   types.StringNull(),
		ProjectAPIKey: types.StringNull(),
		SetID:         types.StringValue("custom.session-token"),
		KeyID:         types.StringNull(),
		Algorithm:     types.StringNull(),
		Use:           types.StringNull(),
		Jwks:          types.StringValue(jwks),
		Keys:          types.StringValue(encoded(t, jwks)),
	}
}

func readModel(t *testing.T, r *JWKResource, state tfsdk.State) (JWKResourceModel, *resource.ReadResponse) {
	t.Helper()
	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	var got JWKResourceModel
	if !resp.State.Raw.IsNull() {
		require.False(t, resp.State.Get(context.Background(), &got).HasError())
	}
	return got, resp
}

// The configured text, pretty-printed as file() reads it, must survive a read
// that returns the same keys in another order, or every plan would show a diff.
func TestRead_SuppliedKeysKeepConfiguredText(t *testing.T) {
	configured := "{\n  \"keys\": [\n    " + keyA + ",\n    " + keyB + "\n  ]\n}\n"
	r, _ := fakeKeysAPI(t, http.StatusOK, keySet(keyB, keyA))
	model := suppliedModel(t, configured)

	got, _ := readModel(t, r, jwkState(t, r, model))

	assert.Equal(t, configured, got.Jwks.ValueString())
	assert.Equal(t, model.Keys.ValueString(), got.Keys.ValueString())
	assert.True(t, got.KeyID.IsNull(), "supplied keys must not set key_id")
	assert.True(t, got.Algorithm.IsNull(), "supplied keys must not set algorithm")
	assert.True(t, got.Use.IsNull(), "supplied keys must not set use")
}

// A key replaced outside Terraform must show up as a jwks change, so the next
// apply uploads the configured set again.
func TestRead_SuppliedKeysChangedOutsideTerraform(t *testing.T) {
	r, _ := fakeKeysAPI(t, http.StatusOK, keySet(keyC))
	model := suppliedModel(t, keySet(keyA))

	got, _ := readModel(t, r, jwkState(t, r, model))

	assert.Equal(t, encoded(t, keySet(keyC)), got.Jwks.ValueString())
	assert.Equal(t, encoded(t, keySet(keyC)), got.Keys.ValueString())
}

func TestRead_SetDeletedOutsideTerraform(t *testing.T) {
	r, _ := fakeKeysAPI(t, http.StatusNotFound, `{"error":"Not Found","error_description":"Unable to locate the requested resource"}`)

	_, resp := readModel(t, r, jwkState(t, r, suppliedModel(t, keySet(keyA))))

	assert.True(t, resp.State.Raw.IsNull(), "a deleted set must be removed from state")
}

func TestRead_ServerErrorKeepsResource(t *testing.T) {
	r, _ := fakeKeysAPI(t, http.StatusInternalServerError, `{"error":"boom, request 404"}`)
	state := jwkState(t, r, suppliedModel(t, keySet(keyA)))

	resp := &resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, resp)

	assert.True(t, resp.Diagnostics.HasError(), "a 500 must surface as an error")
	assert.False(t, resp.State.Raw.IsNull(), "a 500 must not remove the resource from state")
}

// Without jwks in state, Read keeps tracking the first key the way it always
// did. That covers a generated key and the first read after an import.
func TestRead_GeneratedKeyTracksFirstKey(t *testing.T) {
	r, _ := fakeKeysAPI(t, http.StatusOK, keySet(keyA))
	model := suppliedModel(t, keySet(keyA))
	model.Jwks = types.StringNull()

	got, _ := readModel(t, r, jwkState(t, r, model))

	assert.Equal(t, "key-a", got.KeyID.ValueString())
	assert.Equal(t, "ES256", got.Algorithm.ValueString())
	assert.Equal(t, "sig", got.Use.ValueString())
	assert.True(t, got.Jwks.IsNull())
}

func runUpdate(t *testing.T, r *JWKResource, prior, planned JWKResourceModel) JWKResourceModel {
	t.Helper()
	state := jwkState(t, r, prior)
	planState := jwkState(t, r, planned)
	plan := tfsdk.Plan(planState)

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Update(context.Background(), resource.UpdateRequest{Plan: plan, State: state}, resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got JWKResourceModel
	require.False(t, resp.State.Get(context.Background(), &got).HasError())
	return got
}

func TestUpdate_UploadsChangedJwks(t *testing.T) {
	r, requests := fakeKeysAPI(t, http.StatusOK, "")
	prior := suppliedModel(t, keySet(keyA))
	planned := prior
	planned.Jwks = types.StringValue(keySet(keyA, keyB))
	planned.Keys = types.StringUnknown()

	got := runUpdate(t, r, prior, planned)

	require.Len(t, *requests, 1)
	assert.Equal(t, http.MethodPut, (*requests)[0].method)
	assert.Equal(t, "/test-slug/admin/keys/custom.session-token", (*requests)[0].path)
	assert.Equal(t, encoded(t, keySet(keyA, keyB)), encoded(t, (*requests)[0].body))
	assert.Equal(t, keySet(keyA, keyB), got.Jwks.ValueString())
	assert.Equal(t, encoded(t, keySet(keyA, keyB)), got.Keys.ValueString())
}

// Moving resource-level credentials onto an existing set used to fail with
// "Update Not Supported". It must only update state.
func TestUpdate_CredentialsOnlyChangeSkipsUpload(t *testing.T) {
	r, requests := fakeKeysAPI(t, http.StatusOK, "")
	prior := suppliedModel(t, keySet(keyA))
	planned := prior
	planned.ProjectSlug = types.StringValue("test-slug")
	planned.ProjectAPIKey = types.StringValue("test-key")
	planned.Keys = types.StringUnknown()

	got := runUpdate(t, r, prior, planned)

	assert.Empty(t, *requests, "an unchanged jwks must not be uploaded again")
	assert.Equal(t, prior.Keys.ValueString(), got.Keys.ValueString())
	assert.Equal(t, "test-slug", got.ProjectSlug.ValueString())
}
