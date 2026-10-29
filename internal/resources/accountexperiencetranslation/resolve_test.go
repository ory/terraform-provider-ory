package accountexperiencetranslation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		got, err := resolveMessages(ctx, probeStoredURL, probeMessages)
		require.NoError(t, err)
		assert.Equal(t, probeMessages, got)
		assert.Empty(t, *fetched)
	})

	t.Run("changed document is downloaded", func(t *testing.T) {
		fetched := stubFetch(t, `{"login.title":"Changed in the Console"}`, nil)
		got, err := resolveMessages(ctx, probeStoredURL, map[string]string{"login.title": "Sign in"})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"login.title": "Changed in the Console"}, got)
		assert.Equal(t, []string{probeStoredURL}, *fetched)
	})

	t.Run("import downloads the document", func(t *testing.T) {
		stubFetch(t, `{"recovery.title":"Forgot your password?","login.title":"Sign in to Example"}`, nil)
		got, err := resolveMessages(ctx, probeStoredURL, nil)
		require.NoError(t, err)
		assert.Equal(t, probeMessages, got)
	})

	t.Run("download failure is an error", func(t *testing.T) {
		stubFetch(t, "", errors.New("HTTP 503"))
		_, err := resolveMessages(ctx, probeStoredURL, nil)
		assert.ErrorContains(t, err, "HTTP 503")
	})

	t.Run("nested document is an error", func(t *testing.T) {
		stubFetch(t, `{"login":{"title":"x"}}`, nil)
		_, err := resolveMessages(ctx, probeStoredURL, nil)
		assert.ErrorContains(t, err, "parsing the translations document")
	})
}

func TestIsAlreadyRemoved(t *testing.T) {
	// The body the staging API returned on 2026-10-29 for a remove of a locale
	// that had no translations.
	gone := errors.New(`patching project: HTTP 400: 400 Bad Request
Response body: {"error":{"code":400,"status":"Bad Request","reason":"Unable to apply JSON Patch: error in remove for path: '/services/account_experience/config/translations/ja': unable to remove nonexistent key: ja: missing value"}}`)
	assert.True(t, isAlreadyRemoved(gone))

	invalid := errors.New(`patching project: HTTP 400: 400 Bad Request
Response body: {"error":{"code":400,"status":"Bad Request","reason":"field \"account_experience_custom_translations\" contains an invalid locale \"en_US\""}}`)
	assert.False(t, isAlreadyRemoved(invalid))
}
