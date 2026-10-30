package accountexperiencetranslation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// probeMessages and probeStoredURL are a write and the URL the staging API
// stored it at on 2026-10-29. The API names the file after the SHA-512 of the
// decoded document, so the pair pins encodeMessages to the bytes the read path
// compares against.
var probeMessages = map[string]string{
	"login.title":    "Sign in to Example",
	"recovery.title": "Forgot your password?",
}

const probeStoredURL = "https://storage.googleapis.com/bac-gcs-staging/" +
	"fcd380a9a0f4788e0e65d79127cc1b401639eee00e9b984fc277ac2674d958e8" +
	"2a4a84e4ca4c2a43e8e71d1b0015e5af03d5a22e2a8999a7e80aa5f1f7b47cb6.json"

func TestEncodeMessages_MatchesStoredFileName(t *testing.T) {
	assert.True(t, helpers.URLHashMatchesContent(probeStoredURL, encodeMessages(probeMessages)))
}

// TestEncodeMessages_KeepsHTMLCharacters pins the compact, unescaped form that
// the Console also writes. json.Marshal would store "&" as &.
func TestEncodeMessages_KeepsHTMLCharacters(t *testing.T) {
	got := encodeMessages(map[string]string{"footer.terms_of_service.label": "Terms & <b>Conditions</b>"})
	assert.Equal(t, `{"footer.terms_of_service.label":"Terms & <b>Conditions</b>"}`, string(got))
}

// stubFetch replaces the document download for one test and records the URLs
// it was asked for.
func stubFetch(t *testing.T, doc string, err error) *[]string {
	t.Helper()
	var fetched []string
	previous := fetchStoredDocument
	fetchStoredDocument = func(_ context.Context, rawURL string) ([]byte, error) {
		fetched = append(fetched, rawURL)
		return []byte(doc), err
	}
	t.Cleanup(func() { fetchStoredDocument = previous })
	return &fetched
}

func TestResolveMessages(t *testing.T) {
	ctx := context.Background()

	t.Run("matching hash keeps state without a download", func(t *testing.T) {
		fetched := stubFetch(t, "", errors.New("must not be called"))
		got, err := resolveMessages(ctx, probeStoredURL, probeMessages, storedDocument{})
		require.NoError(t, err)
		assert.Equal(t, probeMessages, got)
		assert.Empty(t, *fetched)
	})

	t.Run("changed document is downloaded", func(t *testing.T) {
		fetched := stubFetch(t, `{"login.title":"Changed in the Console"}`, nil)
		got, err := resolveMessages(ctx, probeStoredURL, map[string]string{"login.title": "Sign in"}, storedDocument{})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"login.title": "Changed in the Console"}, got)
		assert.Equal(t, []string{probeStoredURL}, *fetched)
	})

	t.Run("recorded URL keeps state without a download", func(t *testing.T) {
		// A document the Console wrote: its file name is not the hash of the
		// provider's encoding, but an earlier Read matched it with state.
		current := map[string]string{"login.title": "Sign in"}
		fetched := stubFetch(t, "", errors.New("must not be called"))
		got, err := resolveMessages(ctx, probeStoredURL, current, newStoredDocument(probeStoredURL, current))
		require.NoError(t, err)
		assert.Equal(t, current, got)
		assert.Empty(t, *fetched)
	})

	t.Run("record for other messages still downloads", func(t *testing.T) {
		// State changed since the record, for example through an update, so
		// the record proves nothing about the current messages.
		fetched := stubFetch(t, `{"login.title":"Old"}`, nil)
		recorded := newStoredDocument(probeStoredURL, map[string]string{"login.title": "Old"})
		got, err := resolveMessages(ctx, probeStoredURL, map[string]string{"login.title": "New"}, recorded)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"login.title": "Old"}, got)
		assert.Len(t, *fetched, 1)
	})

	t.Run("import downloads the document", func(t *testing.T) {
		stubFetch(t, `{"recovery.title":"Forgot your password?","login.title":"Sign in to Example"}`, nil)
		got, err := resolveMessages(ctx, probeStoredURL, nil, storedDocument{})
		require.NoError(t, err)
		assert.Equal(t, probeMessages, got)
	})

	t.Run("download failure is an error", func(t *testing.T) {
		stubFetch(t, "", errors.New("HTTP 503"))
		_, err := resolveMessages(ctx, probeStoredURL, nil, storedDocument{})
		assert.ErrorContains(t, err, "HTTP 503")
	})

	t.Run("nested document is an error", func(t *testing.T) {
		stubFetch(t, `{"login":{"title":"x"}}`, nil)
		_, err := resolveMessages(ctx, probeStoredURL, nil, storedDocument{})
		assert.ErrorContains(t, err, "parsing the translations document")
	})
}

// newTestResource returns a resource whose client talks to a fake Console that
// answers every request with handler.
func newTestResource(t *testing.T, handler http.HandlerFunc) *AccountExperienceTranslationResource {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := client.NewOryClient(client.OryClientConfig{WorkspaceAPIKey: "ws-test-key", ConsoleAPIURL: srv.URL})
	require.NoError(t, err)
	return &AccountExperienceTranslationResource{client: c}
}

// testState returns the state of the en locale of project proj-1.
func testState(t *testing.T, r *AccountExperienceTranslationResource, messages map[string]string) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	m, diags := types.MapValueFrom(ctx, types.StringType, messages)
	require.False(t, diags.HasError(), "%v", diags)
	state := tfsdk.State{Schema: schemaResp.Schema}
	require.False(t, state.Set(ctx, AccountExperienceTranslationResourceModel{
		ID:        types.StringValue("en"),
		ProjectID: types.StringValue("proj-1"),
		Locale:    types.StringValue("en"),
		Messages:  m,
	}).HasError())
	return state
}

// projectJSON is a GET /projects/{id} body in the given lifecycle state.
func projectJSON(state, translations string) string {
	return `{"id":"proj-1","name":"n","slug":"s","environment":"prod","home_region":"eu-central","revision_id":"r",` +
		`"organizations":[],"state":"` + state + `","services":{"account_experience":{"config":{"translations":` + translations + `}}}}`
}

func TestRead(t *testing.T) {
	ctx := context.Background()
	stored := `{"en":"` + probeStoredURL + `"}`
	for _, tc := range []struct {
		name    string
		project string
		removed bool
	}{
		{name: "stored locale keeps state without a download", project: projectJSON("running", stored)},
		{name: "missing locale is removed", project: projectJSON("running", `{}`), removed: true},
		{name: "soft-deleted project is removed", project: projectJSON("deleted", stored), removed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubFetch(t, "", errors.New("must not be called"))
			r := newTestResource(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.project))
			})
			state := testState(t, r, probeMessages)
			resp := &resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			require.Equal(t, tc.removed, resp.State.Raw.IsNull())
			if tc.removed {
				return
			}
			var got AccountExperienceTranslationResourceModel
			require.False(t, resp.State.Get(ctx, &got).HasError())
			var messages map[string]string
			require.False(t, got.Messages.ElementsAs(ctx, &messages, false).HasError())
			assert.Equal(t, probeMessages, messages)
		})
	}
}

// TestDelete_AlreadyRemovedLocale sends the error bodies the staging API
// returned on 2026-10-29 through the real client, so the check also covers the
// error wrapping that puts the response body into the error.
func TestDelete_AlreadyRemovedLocale(t *testing.T) {
	ctx := context.Background()
	for body, wantErr := range map[string]bool{
		`{"error":{"code":400,"status":"Bad Request","reason":"Unable to apply JSON Patch: error in remove for path: '/services/account_experience/config/translations/en': unable to remove nonexistent key: en: missing value"}}`: false,
		`{"error":{"code":400,"status":"Bad Request","reason":"field \"account_experience_custom_translations\" contains an invalid locale \"en_US\""}}`:                                                                            true,
	} {
		r := newTestResource(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(body))
		})
		state := testState(t, r, probeMessages)
		resp := &resource.DeleteResponse{State: state}
		r.Delete(ctx, resource.DeleteRequest{State: state}, resp)
		assert.Equal(t, wantErr, resp.Diagnostics.HasError(), "%s: %v", body, resp.Diagnostics)
	}
}

func TestImportState(t *testing.T) {
	ctx := context.Background()
	r := &AccountExperienceTranslationResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	imports := func(id string) (AccountExperienceTranslationResourceModel, diag.Diagnostics) {
		resp := &resource.ImportStateResponse{State: tfsdk.State{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		}}
		r.ImportState(ctx, resource.ImportStateRequest{ID: id}, resp)
		var got AccountExperienceTranslationResourceModel
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.Append(resp.State.Get(ctx, &got)...)
		}
		return got, resp.Diagnostics
	}

	got, diags := imports("proj-1/en")
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, "proj-1", got.ProjectID.ValueString())
	assert.Equal(t, "en", got.ID.ValueString())
	assert.Equal(t, "en", got.Locale.ValueString())

	got, diags = imports("de")
	require.False(t, diags.HasError(), "%v", diags)
	assert.True(t, got.ProjectID.IsNull(), "a bare locale leaves project_id to the provider")

	for _, id := range []string{"", "proj-1/", "proj-1/en/x", "proj-1/EN_us"} {
		_, diags = imports(id)
		assert.True(t, diags.HasError(), "import ID %q must be rejected", id)
	}
}
