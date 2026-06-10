package helpers

import (
	"crypto/sha512"
	"encoding/hex"
	"net/url"
	pathpkg "path"
	"strings"
)

// HashFromURL extracts the hex hash portion from a storage URL filename.
// The Ory backoffice stores uploaded content (email templates, account
// experience images) at storage URLs whose filename is the lowercase hex
// SHA-512 of the raw content plus an extension (.txt/.html/.png/...).
// Returns ("", false) if the URL is not in the expected shape.
func HashFromURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	base := pathpkg.Base(u.Path)
	ext := pathpkg.Ext(base)
	hash := strings.TrimSuffix(base, ext)
	if len(hash) != 128 {
		return "", false
	}
	for _, c := range hash {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return "", false
		}
	}
	return hash, true
}

// URLHashMatchesContent reports whether the SHA-512 hex hash embedded in the
// URL's filename matches the SHA-512 of the supplied content. When the hashes
// match we know the API value equals `content` without an extra fetch.
func URLHashMatchesContent(rawURL string, content []byte) bool {
	hash, ok := HashFromURL(rawURL)
	if !ok {
		return false
	}
	sum := sha512.Sum512(content)
	return strings.EqualFold(hash, hex.EncodeToString(sum[:]))
}
