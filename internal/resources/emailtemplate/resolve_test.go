package emailtemplate

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"testing"
)

func TestHashFromURL(t *testing.T) {
	t.Parallel()

	hash128 := "3b22557ea88e8c133219b0706af495382d0b6df202966213ca9d72768f33cd22f58fa28297a0292b6dcc528705a43382d053f401f0568a77df96deeee3ef1e7d"

	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{
			name: "txt-filename",
			in:   "https://storage.example.com/templates/" + hash128 + ".txt",
			want: hash128,
			ok:   true,
		},
		{
			name: "html-filename",
			in:   "https://storage.example.com/templates/" + hash128 + ".html",
			want: hash128,
			ok:   true,
		},
		{
			name: "uppercase-hex-accepted",
			in:   "https://storage.example.com/" + "AB" + hash128[2:] + ".txt",
			want: "AB" + hash128[2:],
			ok:   true,
		},
		{
			name: "not-a-url-style-string",
			in:   "Hello world",
			ok:   false,
		},
		{
			name: "url-without-hash-filename",
			in:   "https://storage.example.com/templates/some-other-file.txt",
			ok:   false,
		},
		{
			name: "short-hash",
			in:   "https://storage.example.com/templates/" + hash128[:64] + ".txt",
			ok:   false,
		},
		{
			name: "non-hex-character",
			in:   "https://storage.example.com/templates/" + "zz" + hash128[2:] + ".txt",
			ok:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := hashFromURL(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok mismatch: got %v want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("hash mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestUrlHashMatchesContent(t *testing.T) {
	t.Parallel()

	content := "Hello {{ .Code }}"
	sum := sha512.Sum512([]byte(content))
	hexHash := hex.EncodeToString(sum[:])
	url := "https://storage.example.com/templates/" + hexHash + ".txt"

	if !urlHashMatchesContent(url, content) {
		t.Fatal("expected matching hash for known content")
	}
	if urlHashMatchesContent(url, "different content") {
		t.Fatal("expected mismatch for different content")
	}
	if urlHashMatchesContent("not-a-storage-url", content) {
		t.Fatal("expected mismatch for non-URL input")
	}
}

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
			if got := isHTTPSURL(tc.in); got != tc.want {
				t.Fatalf("isHTTPSURL(%q) = %v, want %v", tc.in, got, tc.want)
			}
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
		if got != "Hello" {
			t.Fatalf("got %q want %q", got, "Hello")
		}
	})

	t.Run("returns plain literal as-is", func(t *testing.T) {
		got := resolveStoredTemplate(context.Background(), "literal value", "previous")
		if got != "literal value" {
			t.Fatalf("got %q want %q", got, "literal value")
		}
	})

	t.Run("non-https URL is treated as literal not fetched", func(t *testing.T) {
		// http://, ftp://, file:// must never trigger the network fetcher;
		// `isHTTPSURL` returns false so we surface the raw value.
		origFetcher := urlContentFetcher
		urlContentFetcher = func(_ context.Context, _ string) (string, error) {
			t.Fatal("fetcher must not be called for non-HTTPS schemes")
			return "", nil
		}
		defer func() { urlContentFetcher = origFetcher }()

		got := resolveStoredTemplate(context.Background(), "http://example.com/foo", "previous")
		if got != "http://example.com/foo" {
			t.Fatalf("got %q want literal http URL", got)
		}
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
		if got != content {
			t.Fatalf("got %q want %q", got, content)
		}
		if fetchCalled {
			t.Fatal("fetcher should not be called when hash matches state")
		}
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
		if got != "OTHER VALUE" {
			t.Fatalf("got %q want %q", got, "OTHER VALUE")
		}
		if fetchedURL != driftedURL {
			t.Fatalf("fetcher called with %q, want %q", fetchedURL, driftedURL)
		}
	})

	t.Run("fetch failure falls back to state", func(t *testing.T) {
		origFetcher := urlContentFetcher
		urlContentFetcher = func(_ context.Context, _ string) (string, error) {
			return "", errors.New("network down")
		}
		defer func() { urlContentFetcher = origFetcher }()

		got := resolveStoredTemplate(context.Background(), driftedURL, content)
		if got != content {
			t.Fatalf("on fetch failure expected state %q got %q", content, got)
		}
	})

	t.Run("empty api value clears state", func(t *testing.T) {
		got := resolveStoredTemplate(context.Background(), "", "state value")
		if got != "" {
			t.Fatalf("got %q want empty (Console reset must surface as drift)", got)
		}
	})
}
