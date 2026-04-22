package samlprovider

import (
	"errors"
	"regexp"

	ory "github.com/ory/client-go"
)

// mapperURLRegex matches the URL prefixes Ory accepts for SAML mapper and IDP
// metadata values. The Ory API rejects any other prefix with a 400 error.
var mapperURLRegex = regexp.MustCompile(`^(base64://|https?://)`)

// apiErrorDetail extracts the API response body from an Ory client-go error.
// GenericOpenAPIError.Error() returns only the HTTP status — the JSON body
// (which carries the actual server-side reason) is on .Body().
func apiErrorDetail(err error) string {
	var openAPIErr *ory.GenericOpenAPIError
	if errors.As(err, &openAPIErr) {
		if body := string(openAPIErr.Body()); body != "" {
			return err.Error() + ": " + body
		}
	}
	return err.Error()
}
