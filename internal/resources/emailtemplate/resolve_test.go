package emailtemplate

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for the storage-URL hash helpers live in internal/helpers
// (contenthash_test.go) since the logic moved there to be shared with
// the project_config account experience image handling.

func TestIsHTTPSURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{"https://example.com/x", true},
		{"https://storage.example.com/templates/aaa.txt", true},
		{"http://example.com/x", false},  // plain http rejected to keep SSRF surface small
		{"ftp://example.com/x", false},   // arbitrary schemes never fetched
		{"file:///etc/passwd", false},    // explicitly rejected
		{"base64://SGVsbG8=", false},     // base64 literal, callers decode separately
		{"Hello world", false},           // plain text
		{"https:///path-no-host", false}, // scheme-only, no host
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, isHTTPSURL(tc.in))
		})
	}
}

func TestResolveStoredTemplate(t *testing.T) {
	t.Parallel()

	content := "Hello there"
	sum := sha512.Sum512([]byte(content))
	matchingURL := "https://storage.example.com/templates/" + hex.EncodeToString(sum[:]) + ".txt"

	otherSum := sha512.Sum512([]byte("OTHER VALUE"))
	driftedURL := "https://storage.example.com/templates/" + hex.EncodeToString(otherSum[:]) + ".txt"

	t.Run("returns decoded base64 literal", func(t *testing.T) {
		got := resolveStoredTemplate(context.Background(), "base64://SGVsbG8=", "previous")
		assert.Equal(t, "Hello", got)
	})

	t.Run("returns plain literal as-is", func(t *testing.T) {
		got := resolveStoredTemplate(context.Background(), "literal value", "previous")
		assert.Equal(t, "literal value", got)
	})

	t.Run("non-https URL is treated as literal not fetched", func(t *testing.T) {
		// http://, ftp://, file:// must never trigger the network fetcher;
		// `isHTTPSURL` returns false so we surface the raw value.
		origFetcher := urlContentFetcher
		urlContentFetcher = func(_ context.Context, _ string) (string, error) {
			require.FailNow(t, "fetcher must not be called for non-HTTPS schemes")
			return "", nil
		}
		defer func() { urlContentFetcher = origFetcher }()

		got := resolveStoredTemplate(context.Background(), "http://example.com/foo", "previous")
		assert.Equal(t, "http://example.com/foo", got, "want literal http URL")
	})

	t.Run("hash matches state preserves state without fetching", func(t *testing.T) {
		origFetcher := urlContentFetcher
		fetchCalled := false
		urlContentFetcher = func(_ context.Context, _ string) (string, error) {
			fetchCalled = true
			return "should not be called", nil
		}
		defer func() { urlContentFetcher = origFetcher }()

		got := resolveStoredTemplate(context.Background(), matchingURL, content)
		assert.Equal(t, content, got)
		assert.False(t, fetchCalled, "fetcher should not be called when hash matches state")
	})

	t.Run("hash mismatch fetches and returns body", func(t *testing.T) {
		origFetcher := urlContentFetcher
		var fetchedURL string
		urlContentFetcher = func(_ context.Context, u string) (string, error) {
			fetchedURL = u
			return "OTHER VALUE", nil
		}
		defer func() { urlContentFetcher = origFetcher }()

		got := resolveStoredTemplate(context.Background(), driftedURL, content)
		assert.Equal(t, "OTHER VALUE", got)
		assert.Equal(t, driftedURL, fetchedURL, "fetcher should be called with drifted URL")
	})

	t.Run("fetch failure falls back to state", func(t *testing.T) {
		origFetcher := urlContentFetcher
		urlContentFetcher = func(_ context.Context, _ string) (string, error) {
			return "", errors.New("network down")
		}
		defer func() { urlContentFetcher = origFetcher }()

		got := resolveStoredTemplate(context.Background(), driftedURL, content)
		assert.Equal(t, content, got, "on fetch failure expected state")
	})

	t.Run("empty api value clears state", func(t *testing.T) {
		got := resolveStoredTemplate(context.Background(), "", "state value")
		assert.Empty(t, got, "Console reset must surface as drift")
	})
}
