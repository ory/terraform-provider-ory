package samlprovider

import "regexp"

// mapperURLRegex matches the URL prefixes Ory accepts for SAML mapper and IDP
// metadata values. The Ory API rejects any other prefix with a 400 error.
var mapperURLRegex = regexp.MustCompile(`^(base64://|https?://)`)
