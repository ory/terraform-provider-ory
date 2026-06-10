package projectconfig

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tinyPNG is a 1x1 transparent PNG used as image content in tests.
var tinyPNG = mustDecodeBase64("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

func mustDecodeBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func storageURLFor(content []byte) string {
	sum := sha512.Sum512(content)
	return "https://storage.googleapis.com/bac-gcs-production/" + hex.EncodeToString(sum[:]) + ".png"
}

func dataURIFor(content []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(content)
}

func TestDataURIPayload(t *testing.T) {
	t.Parallel()

	t.Run("valid png data uri decodes", func(t *testing.T) {
		payload, ok := dataURIPayload(dataURIFor(tinyPNG))
		require.True(t, ok)
		assert.Equal(t, tinyPNG, payload)
	})

	t.Run("not a data uri", func(t *testing.T) {
		_, ok := dataURIPayload("https://example.com/logo.png")
		assert.False(t, ok)
	})

	t.Run("missing comma", func(t *testing.T) {
		_, ok := dataURIPayload("data:image/png;base64")
		assert.False(t, ok)
	})

	t.Run("not base64 encoded marker", func(t *testing.T) {
		_, ok := dataURIPayload("data:image/svg+xml,<svg/>")
		assert.False(t, ok)
	})

	t.Run("invalid base64 payload", func(t *testing.T) {
		_, ok := dataURIPayload("data:image/png;base64,!!!not-base64!!!")
		assert.False(t, ok)
	})

	t.Run("empty string", func(t *testing.T) {
		_, ok := dataURIPayload("")
		assert.False(t, ok)
	})
}

func TestResolveAccountExperienceImage(t *testing.T) {
	t.Parallel()

	dataURI := dataURIFor(tinyPNG)
	matchingURL := storageURLFor(tinyPNG)
	otherURL := storageURLFor([]byte("different image bytes"))

	t.Run("api equals state keeps state", func(t *testing.T) {
		got := resolveAccountExperienceImage(matchingURL, matchingURL)
		assert.Equal(t, types.StringValue(matchingURL), got)
	})

	t.Run("both empty keeps empty", func(t *testing.T) {
		got := resolveAccountExperienceImage("", "")
		assert.Equal(t, types.StringValue(""), got)
	})

	t.Run("storage url hash matches data uri keeps state", func(t *testing.T) {
		got := resolveAccountExperienceImage(matchingURL, dataURI)
		assert.Equal(t, types.StringValue(dataURI), got, "matching content hash must not produce drift")
	})

	t.Run("storage url hash mismatch surfaces api value", func(t *testing.T) {
		got := resolveAccountExperienceImage(otherURL, dataURI)
		assert.Equal(t, types.StringValue(otherURL), got, "image replaced out-of-band must surface drift")
	})

	t.Run("cleared in console surfaces empty", func(t *testing.T) {
		got := resolveAccountExperienceImage("", dataURI)
		assert.Equal(t, types.StringValue(""), got, "image cleared out-of-band must surface drift")
	})

	t.Run("state url differs from api url surfaces api value", func(t *testing.T) {
		got := resolveAccountExperienceImage(otherURL, matchingURL)
		assert.Equal(t, types.StringValue(otherURL), got)
	})

	t.Run("malformed state value surfaces api value", func(t *testing.T) {
		got := resolveAccountExperienceImage(matchingURL, "data:image/png;base64,!!!")
		assert.Equal(t, types.StringValue(matchingURL), got)
	})
}

func TestAccountExperienceImagePatches(t *testing.T) {
	t.Parallel()

	t.Run("null fields produce no patches", func(t *testing.T) {
		plan := &ProjectConfigResourceModel{}
		assert.Empty(t, accountExperienceImagePatches(plan))
	})

	t.Run("set fields patch the *_url config keys", func(t *testing.T) {
		dataURI := dataURIFor(tinyPNG)
		plan := &ProjectConfigResourceModel{
			AccountExperienceLogoLight:   types.StringValue(dataURI),
			AccountExperienceFaviconDark: types.StringValue(""),
		}

		patches := accountExperienceImagePatches(plan)
		require.Len(t, patches, 2)

		byPath := map[string]interface{}{}
		for _, p := range patches {
			assert.Equal(t, "replace", p.Op)
			byPath[p.Path] = p.Value
		}
		assert.Equal(t, dataURI, byPath["/services/account_experience/config/logo_light_url"])
		assert.Equal(t, "", byPath["/services/account_experience/config/favicon_dark_url"])
	})
}

func TestAXImageValueRegex(t *testing.T) {
	t.Parallel()

	valid := []string{
		"",
		"data:image/png;base64," + base64.StdEncoding.EncodeToString(tinyPNG),
		"data:image/svg+xml;base64,PHN2Zy8+",
		"data:image/x-icon;base64,AAAB",
		"data:image/vnd.microsoft.icon;base64,AAAB",
		"data:image/jpeg;base64,/9j/4A==",
		"data:image/webp;base64,UklGRg==",
		"data:image/gif;base64,R0lGODsbAAAA",
		"https://storage.googleapis.com/bucket/abc.png",
	}
	for _, v := range valid {
		assert.True(t, axImageValueRegex.MatchString(v), "expected valid: %q", v)
	}

	invalid := []string{
		"http://example.com/logo.png",            // plain http
		"iVBORw0KGgo=",                           // raw base64 without data: prefix
		"data:image/png,plain",                   // not base64-encoded
		"data:application/json;base64,e30=",      // not an image type
		"data:image/tiff;base64,AAAB",            // unsupported image type
		"data:image/png;base64,not base64 data!", // invalid base64 alphabet
		"ftp://example.com/logo.png",
	}
	for _, v := range invalid {
		assert.False(t, axImageValueRegex.MatchString(v), "expected invalid: %q", v)
	}
}
