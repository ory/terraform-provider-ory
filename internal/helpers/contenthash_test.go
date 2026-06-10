package helpers

import (
	"crypto/sha512"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name: "png-filename",
			in:   "https://storage.googleapis.com/bac-gcs-production/" + hash128 + ".png",
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
			got, ok := HashFromURL(tc.in)
			require.Equal(t, tc.ok, ok, "ok mismatch")
			assert.Equal(t, tc.want, got, "hash mismatch")
		})
	}
}

func TestURLHashMatchesContent(t *testing.T) {
	t.Parallel()

	content := []byte("Hello {{ .Code }}")
	sum := sha512.Sum512(content)
	hexHash := hex.EncodeToString(sum[:])
	url := "https://storage.example.com/templates/" + hexHash + ".txt"

	assert.True(t, URLHashMatchesContent(url, content), "expected matching hash for known content")
	assert.False(t, URLHashMatchesContent(url, []byte("different content")), "expected mismatch for different content")
	assert.False(t, URLHashMatchesContent("not-a-storage-url", content), "expected mismatch for non-URL input")
}
