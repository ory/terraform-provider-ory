package scimclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

const (
	fakeProjectID = "proj-1"
	fakeStoredURL = "https://storage.googleapis.com/bucket/stored.jsonnet"
)

// fakeConsole imitates the two normalized-revision endpoints the resource
// uses. It applies the JSON Patch operations it receives to an in-memory
// scim_clients array the way the API does: each write produces a new
// revision id, assigns fresh row ids, redacts every secret to "", and
// rewrites mapper_url to an object-storage URL. Tests read the recorded
// operations to assert what the resource sent.
type fakeConsole struct {
	mu       sync.Mutex
	revision int
	rows     int
	clients  []map[string]interface{}
	patches  [][]map[string]interface{}
	secrets  []string // the secret each PATCH carried, "" when it carried none
	conflict func(f *fakeConsole) bool
	hasArray bool
}

func newFakeConsole(clients ...map[string]interface{}) *fakeConsole {
	f := &fakeConsole{revision: 1, hasArray: true}
	for _, c := range clients {
		f.clients = append(f.clients, f.store(c))
	}
	return f
}

func (f *fakeConsole) revisionID() string { return "rev-" + strconv.Itoa(f.revision) }

func (f *fakeConsole) store(c map[string]interface{}) map[string]interface{} {
	f.rows++
	stored := map[string]interface{}{}
	for k, v := range c {
		stored[k] = v
	}
	stored["id"] = "row-" + strconv.Itoa(f.rows)
	stored["authorization_header_secret"] = ""
	if mapper, _ := stored["mapper_url"].(string); strings.HasPrefix(mapper, "base64://") {
		stored["mapper_url"] = fakeStoredURL
	}
	return stored
}

func (f *fakeConsole) body() []byte {
	revision := map[string]interface{}{"id": f.revisionID()}
	if f.hasArray {
		arr := make([]interface{}, 0, len(f.clients))
		for _, c := range f.clients {
			arr = append(arr, c)
		}
		revision["scim_clients"] = arr
	}
	b, _ := json.Marshal(map[string]interface{}{"current_revision": revision})
	return b
}

func (f *fakeConsole) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/normalized/projects/"+fakeProjectID:
		_, _ = w.Write(f.body())
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/normalized/projects/"+fakeProjectID+"/revision/"):
		var ops []map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.patches = append(f.patches, ops)
		if f.conflict != nil && f.conflict(f) {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"code":409,"status":"Conflict"}}`))
			return
		}
		if path.Base(r.URL.Path) != f.revisionID() {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"code":409,"status":"Conflict","reason":"stale revision"}}`))
			return
		}
		if err := f.apply(ops); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":{"code":400,"status":"Bad Request","reason":%q}}`, err.Error())
			return
		}
		f.revision++
		_, _ = w.Write(f.body())
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeConsole) apply(ops []map[string]interface{}) error {
	for _, op := range ops {
		opPath, _ := op["path"].(string)
		f.secrets = append(f.secrets, secretOf(op["value"]))

		switch {
		case op["op"] == "add" && opPath == scimClientsPath:
			values, ok := op["value"].([]interface{})
			if !ok {
				return fmt.Errorf("add %s expects an array", opPath)
			}
			f.clients = nil
			for _, v := range values {
				f.clients = append(f.clients, f.store(v.(map[string]interface{})))
			}
			f.hasArray = true
		case op["op"] == "add" && opPath == scimClientsPath+"/-":
			if !f.hasArray {
				return fmt.Errorf("path %s not found", opPath)
			}
			f.clients = append(f.clients, f.store(op["value"].(map[string]interface{})))
		case op["op"] == "replace":
			index, err := f.index(opPath)
			if err != nil {
				return err
			}
			f.clients[index] = f.store(op["value"].(map[string]interface{}))
		case op["op"] == "remove":
			index, err := f.index(opPath)
			if err != nil {
				return err
			}
			f.clients = append(f.clients[:index], f.clients[index+1:]...)
		default:
			return fmt.Errorf("unsupported operation %v %s", op["op"], opPath)
		}
	}
	return nil
}

// secretOf returns the secret an operation value carries. The first client
// is written as a whole array, every later one as a single object.
func secretOf(value interface{}) string {
	switch v := value.(type) {
	case map[string]interface{}:
		secret, _ := v["authorization_header_secret"].(string)
		return secret
	case []interface{}:
		if len(v) == 1 {
			return secretOf(v[0])
		}
	}
	return ""
}

func (f *fakeConsole) index(opPath string) (int, error) {
	index, err := strconv.Atoi(strings.TrimPrefix(opPath, scimClientsPath+"/"))
	if err != nil || index < 0 || index >= len(f.clients) {
		return 0, fmt.Errorf("invalid index in %s", opPath)
	}
	return index, nil
}

func (f *fakeConsole) clientIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.clients))
	for _, c := range f.clients {
		ids = append(ids, c["client_id"].(string))
	}
	return ids
}

func (f *fakeConsole) lastOp(t *testing.T) map[string]interface{} {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.patches, "no PATCH was sent")
	last := f.patches[len(f.patches)-1]
	require.Len(t, last, 1, "the resource sends one operation per PATCH")
	return last[0]
}

func (f *fakeConsole) patchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.patches)
}

type harness struct {
	t       *testing.T
	console *fakeConsole
	r       *SCIMClientResource
	schema  schema.Schema
}

func newHarness(t *testing.T, console *fakeConsole) *harness {
	t.Helper()
	srv := httptest.NewServer(console)
	t.Cleanup(srv.Close)

	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   srv.URL,
		ProjectID:       fakeProjectID,
	})
	require.NoError(t, err)

	r := &SCIMClientResource{client: c}
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	previous := readRetryBaseDelay
	readRetryBaseDelay = time.Millisecond
	t.Cleanup(func() { readRetryBaseDelay = previous })

	return &harness{t: t, console: console, r: r, schema: schemaResp.Schema}
}

func (h *harness) plan(model SCIMClientResourceModel) tfsdk.Plan {
	p := tfsdk.Plan{Schema: h.schema}
	require.False(h.t, p.Set(context.Background(), model).HasError())
	return p
}

// config renders the model as a configuration. tfsdk.Config has no Set, so
// the value is encoded through a State with the same schema.
func (h *harness) config(model SCIMClientResourceModel) tfsdk.Config {
	return tfsdk.Config{Schema: h.schema, Raw: h.state(model).Raw}
}

func (h *harness) state(model SCIMClientResourceModel) tfsdk.State {
	s := tfsdk.State{Schema: h.schema}
	require.False(h.t, s.Set(context.Background(), model).HasError())
	return s
}

func (h *harness) create(plan, config SCIMClientResourceModel) (*resource.CreateResponse, SCIMClientResourceModel) {
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: h.schema}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(plan), Config: h.config(config)}, resp)
	var got SCIMClientResourceModel
	if !resp.Diagnostics.HasError() {
		require.False(h.t, resp.State.Get(context.Background(), &got).HasError())
	}
	return resp, got
}

func (h *harness) read(state SCIMClientResourceModel) (*resource.ReadResponse, SCIMClientResourceModel) {
	s := h.state(state)
	resp := &resource.ReadResponse{State: s}
	h.r.Read(context.Background(), resource.ReadRequest{State: s}, resp)
	var got SCIMClientResourceModel
	if !resp.Diagnostics.HasError() && !resp.State.Raw.IsNull() {
		require.False(h.t, resp.State.Get(context.Background(), &got).HasError())
	}
	return resp, got
}

func (h *harness) update(plan, config, state SCIMClientResourceModel) (*resource.UpdateResponse, SCIMClientResourceModel) {
	resp := &resource.UpdateResponse{State: h.state(state)}
	h.r.Update(context.Background(), resource.UpdateRequest{Plan: h.plan(plan), Config: h.config(config), State: h.state(state)}, resp)
	var got SCIMClientResourceModel
	if !resp.Diagnostics.HasError() {
		require.False(h.t, resp.State.Get(context.Background(), &got).HasError())
	}
	return resp, got
}

func (h *harness) delete(state SCIMClientResourceModel) *resource.DeleteResponse {
	resp := &resource.DeleteResponse{State: h.state(state)}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(state)}, resp)
	return resp
}

// planned is a plan for a stateful-secret client. State and project_id are
// unknown the way the framework leaves computed attributes before Create.
func planned(clientID string) SCIMClientResourceModel {
	return SCIMClientResourceModel{
		ID:                                 types.StringUnknown(),
		ProjectID:                          types.StringUnknown(),
		OrganizationID:                     types.StringValue("org-1"),
		ClientID:                           types.StringValue(clientID),
		Label:                              types.StringValue("Okta"),
		MapperURL:                          types.StringValue("base64://abc"),
		AuthorizationHeaderSecret:          types.StringValue("s3cret"),
		AuthorizationHeaderSecretWO:        types.StringNull(),
		AuthorizationHeaderSecretWOVersion: types.StringNull(),
		State:                              types.StringValue(stateEnabled),
	}
}

// stored is the state Create leaves behind for planned("okta").
func stored() SCIMClientResourceModel {
	m := planned("okta")
	m.ID = types.StringValue("okta")
	m.ProjectID = types.StringValue(fakeProjectID)
	return m
}

func storedClient(clientID string) map[string]interface{} {
	return map[string]interface{}{
		"client_id":                   clientID,
		"label":                       "Okta",
		"organization_id":             "org-1",
		"mapper_url":                  "base64://abc",
		"authorization_header_secret": "s3cret",
		"state":                       stateEnabled,
	}
}

func TestCreate_FirstClientWritesWholeArray(t *testing.T) {
	h := newHarness(t, newFakeConsole())

	resp, got := h.create(planned("okta"), planned("okta"))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	op := h.console.lastOp(t)
	assert.Equal(t, "add", op["op"])
	assert.Equal(t, scimClientsPath, op["path"], "the first client must create the array")
	values, ok := op["value"].([]interface{})
	require.True(t, ok, "the array is written as a whole")
	require.Len(t, values, 1)
	assert.Equal(t, "s3cret", values[0].(map[string]interface{})["authorization_header_secret"])
	assert.Equal(t, "enabled", values[0].(map[string]interface{})["state"], "state must always be sent")
	_, hasID := values[0].(map[string]interface{})["id"]
	assert.False(t, hasID, "a create must not invent a row id")

	assert.Equal(t, "okta", got.ID.ValueString(), "id is the client_id")
	assert.Equal(t, fakeProjectID, got.ProjectID.ValueString(), "project_id falls back to the provider's")
	assert.Equal(t, "s3cret", got.AuthorizationHeaderSecret.ValueString(), "the secret stays in state even though the API redacts it")
	assert.Equal(t, "base64://abc", got.MapperURL.ValueString(), "the configured mapper stays in state even though the API rewrites it")
}

func TestCreate_AppendsWhenClientsExist(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure")))

	resp, _ := h.create(planned("okta"), planned("okta"))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	op := h.console.lastOp(t)
	assert.Equal(t, "add", op["op"])
	assert.Equal(t, scimClientsPath+"/-", op["path"], "later clients append so siblings survive")
	assert.Equal(t, []string{"azure", "okta"}, h.console.clientIDs())
}

// A client_id that already exists is taken over instead of failing with the
// API's 409, matching ory_social_provider and ory_saml_provider.
func TestCreate_TakesOverExistingClientID(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure"), storedClient("okta")))

	plan := planned("okta")
	plan.Label = types.StringValue("Okta relabeled")
	resp, _ := h.create(plan, plan)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	op := h.console.lastOp(t)
	assert.Equal(t, "replace", op["op"])
	assert.Equal(t, scimClientsPath+"/1", op["path"])
	value := op["value"].(map[string]interface{})
	assert.Equal(t, "row-2", value["id"], "the stored row id is sent so the server matches strictly by id")
	assert.Equal(t, []string{"azure", "okta"}, h.console.clientIDs(), "no duplicate is created")
}

// The write-only secret lives only in the configuration. It must reach the
// API and must not reach state.
func TestCreate_WriteOnlySecretIsSentAndNotStored(t *testing.T) {
	h := newHarness(t, newFakeConsole())

	plan := planned("okta")
	plan.AuthorizationHeaderSecret = types.StringNull()
	plan.AuthorizationHeaderSecretWOVersion = types.StringValue("1")
	config := plan
	config.AuthorizationHeaderSecretWO = types.StringValue("wo-s3cret")

	resp, got := h.create(plan, config)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	assert.Equal(t, []string{"wo-s3cret"}, h.console.secrets, "the write-only value must be the secret sent")
	assert.True(t, got.AuthorizationHeaderSecret.IsNull())
	assert.True(t, got.AuthorizationHeaderSecretWO.IsNull())
	assert.Equal(t, "1", got.AuthorizationHeaderSecretWOVersion.ValueString())
}

func TestCreate_ExplainsMissingOrganization(t *testing.T) {
	console := newFakeConsole()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":500,"message":"insert on table \"project_revision_scim_clients\" violates foreign key constraint \"project_revision_scim_clients_organization_fkey\""}}`))
			return
		}
		console.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c, err := client.NewOryClient(client.OryClientConfig{WorkspaceAPIKey: testutil.TestWorkspaceAPIKey, ConsoleAPIURL: srv.URL, ProjectID: fakeProjectID})
	require.NoError(t, err)
	h := newHarness(t, console)
	h.r.client = c

	resp, _ := h.create(planned("okta"), planned("okta"))
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "organization_id must be the ID of an organization in project proj-1")
}

// Three creates in one apply share one revision array. The client-level
// mutex serializes them, so none may overwrite another.
func TestCreate_ConcurrentCreatesAllPersist(t *testing.T) {
	h := newHarness(t, newFakeConsole())

	ids := []string{"one", "two", "three"}
	var wg sync.WaitGroup
	errs := make(chan string, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			resp := &resource.CreateResponse{State: tfsdk.State{Schema: h.schema}}
			h.r.Create(context.Background(), resource.CreateRequest{Plan: h.plan(planned(id)), Config: h.config(planned(id))}, resp)
			if resp.Diagnostics.HasError() {
				errs <- fmt.Sprintf("%v", resp.Diagnostics)
			}
		}(id)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	assert.ElementsMatch(t, ids, h.console.clientIDs())
}

func TestRead_MapsServerFieldsAndPreservesSecretAndMapper(t *testing.T) {
	c := storedClient("okta")
	c["label"] = "Renamed in the Console"
	c["organization_id"] = "org-2"
	c["state"] = stateDisabled
	h := newHarness(t, newFakeConsole(c))

	resp, got := h.read(stored())
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.False(t, resp.State.Raw.IsNull())

	assert.Equal(t, "Renamed in the Console", got.Label.ValueString(), "label drift is detected")
	assert.Equal(t, "org-2", got.OrganizationID.ValueString(), "organization drift is detected")
	assert.Equal(t, "disabled", got.State.ValueString(), "state drift is detected")
	assert.Equal(t, "s3cret", got.AuthorizationHeaderSecret.ValueString(), "the redacted secret must not replace the state value")
	assert.Equal(t, "base64://abc", got.MapperURL.ValueString(), "the rewritten mapper URL must not replace the configured value")
	assert.Equal(t, "okta", got.ID.ValueString())
	assert.Equal(t, fakeProjectID, got.ProjectID.ValueString())
}

// After an import state has no mapper_url, so the stored URL is the only
// value available. It is accepted by the API on a later write.
func TestRead_PopulatesMapperURLOnImport(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("okta")))

	imported := SCIMClientResourceModel{
		ID:                                 types.StringValue("okta"),
		ProjectID:                          types.StringNull(),
		OrganizationID:                     types.StringNull(),
		ClientID:                           types.StringValue("okta"),
		Label:                              types.StringNull(),
		MapperURL:                          types.StringNull(),
		AuthorizationHeaderSecret:          types.StringNull(),
		AuthorizationHeaderSecretWO:        types.StringNull(),
		AuthorizationHeaderSecretWOVersion: types.StringNull(),
		State:                              types.StringNull(),
	}
	resp, got := h.read(imported)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	assert.Equal(t, fakeStoredURL, got.MapperURL.ValueString())
	assert.Equal(t, "Okta", got.Label.ValueString())
	assert.Equal(t, "org-1", got.OrganizationID.ValueString())
	assert.Equal(t, "enabled", got.State.ValueString())
	assert.Equal(t, fakeProjectID, got.ProjectID.ValueString(), "project_id resolves from the provider when the import id had none")
	assert.True(t, got.AuthorizationHeaderSecret.IsNull(), "the secret cannot be recovered by an import")
}

// A client deleted in the Console, or removed with its organization, must
// leave state so the next plan recreates it instead of failing.
func TestRead_MissingClientIsRemovedFromState(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure")))

	resp, _ := h.read(stored())
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "a missing client must be removed from state")
}

func TestUpdate_ReplacesAtIndexWithRowID(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure"), storedClient("okta")))

	plan := stored()
	plan.Label = types.StringValue("Okta updated")
	plan.State = types.StringValue(stateDisabled)
	plan.AuthorizationHeaderSecret = types.StringValue("rotated")
	resp, got := h.update(plan, plan, stored())
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	op := h.console.lastOp(t)
	assert.Equal(t, "replace", op["op"])
	assert.Equal(t, scimClientsPath+"/1", op["path"])
	value := op["value"].(map[string]interface{})
	assert.Equal(t, "row-2", value["id"])
	assert.Equal(t, "Okta updated", value["label"])
	assert.Equal(t, "disabled", value["state"])
	assert.Equal(t, "rotated", value["authorization_header_secret"])
	assert.Equal(t, "rotated", got.AuthorizationHeaderSecret.ValueString())
	assert.Equal(t, []string{"azure", "okta"}, h.console.clientIDs())
}

func TestUpdate_MissingClientFails(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure")))

	resp, _ := h.update(stored(), stored(), stored())
	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "SCIM Client Not Found", resp.Diagnostics.Errors()[0].Summary())
	assert.Equal(t, 0, h.console.patchCount(), "nothing may be written for a client that is gone")
}

// The patch is rebuilt from the re-read revision after a conflict, so an
// array that shifted underneath the update still hits the right element.
func TestUpdate_RebuildsIndexAfterConflict(t *testing.T) {
	console := newFakeConsole(storedClient("okta"))
	console.conflict = func(f *fakeConsole) bool {
		if len(f.patches) > 1 {
			return false
		}
		// Someone else prepended a client between the read and the patch.
		f.clients = append([]map[string]interface{}{f.store(storedClient("azure"))}, f.clients...)
		f.revision++
		return true
	}
	h := newHarness(t, console)

	plan := stored()
	plan.Label = types.StringValue("Okta updated")
	resp, _ := h.update(plan, plan, stored())
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	require.Equal(t, 2, h.console.patchCount())
	assert.Equal(t, scimClientsPath+"/0", h.console.patches[0][0]["path"])
	assert.Equal(t, scimClientsPath+"/1", h.console.patches[1][0]["path"], "the retry must address the shifted index")
	assert.Equal(t, "Okta updated", console.clients[1]["label"])
	assert.Equal(t, "Okta", console.clients[0]["label"], "the out-of-band client must be untouched")
}

func TestDelete_RemovesAtIndex(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure"), storedClient("okta"), storedClient("google")))

	resp := h.delete(stored())
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	op := h.console.lastOp(t)
	assert.Equal(t, "remove", op["op"])
	assert.Equal(t, scimClientsPath+"/1", op["path"])
	assert.Equal(t, []string{"azure", "google"}, h.console.clientIDs(), "siblings survive")
}

// Deleting the organization deletes its clients server-side, so Delete must
// treat an already-missing client as done.
func TestDelete_MissingClientIsNoOp(t *testing.T) {
	h := newHarness(t, newFakeConsole(storedClient("azure")))

	resp := h.delete(stored())
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, 0, h.console.patchCount())
	assert.Equal(t, []string{"azure"}, h.console.clientIDs())
}

func TestImportState_CompositeAndBareIDs(t *testing.T) {
	h := newHarness(t, newFakeConsole())

	imports := func(id string) (*resource.ImportStateResponse, SCIMClientResourceModel) {
		resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: h.schema}}
		resp.State.Raw = h.state(SCIMClientResourceModel{
			ID: types.StringNull(), ProjectID: types.StringNull(), OrganizationID: types.StringNull(),
			ClientID: types.StringNull(), Label: types.StringNull(), MapperURL: types.StringNull(),
			AuthorizationHeaderSecret: types.StringNull(), AuthorizationHeaderSecretWO: types.StringNull(),
			AuthorizationHeaderSecretWOVersion: types.StringNull(), State: types.StringNull(),
		}).Raw
		h.r.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, resp)
		var got SCIMClientResourceModel
		if !resp.Diagnostics.HasError() {
			require.False(t, resp.State.Get(context.Background(), &got).HasError())
		}
		return resp, got
	}

	resp, got := imports("proj-9/okta")
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, "okta", got.ID.ValueString())
	assert.Equal(t, "okta", got.ClientID.ValueString())
	assert.Equal(t, "proj-9", got.ProjectID.ValueString())

	resp, got = imports("okta")
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, "okta", got.ClientID.ValueString())
	assert.True(t, got.ProjectID.IsNull(), "a bare client id leaves project_id to the provider")

	resp, _ = imports("proj-9/")
	assert.True(t, resp.Diagnostics.HasError(), "an id without a client id must be rejected")
}
