package samlprovider

import (
	"context"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
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

// storageContentFetcher is a variable so unit tests can replace it.
var storageContentFetcher = func(ctx context.Context, rawURL string) ([]byte, error) {
	return client.FetchSafeHTTPS(ctx, "SAML provider content", rawURL)
}

// inlineStorageURL turns a storage URL the API reports for an uploaded
// base64:// value back into base64://. It returns the URL unchanged when it is
// not a storage URL, the download fails, or the downloaded content does not
// match the hash in the URL.
func inlineStorageURL(ctx context.Context, apiValue string) string {
	if _, ok := helpers.HashFromURL(apiValue); !ok {
		return apiValue
	}
	content, err := storageContentFetcher(ctx, apiValue)
	if err == nil && !helpers.URLHashMatchesContent(apiValue, content) {
		err = errors.New("the downloaded content does not match the hash in the URL")
	}
	if err != nil {
		tflog.Warn(ctx, "Keeping the storage URL in state", map[string]interface{}{"url": apiValue, "error": err.Error()})
		return apiValue
	}
	return "base64://" + base64.StdEncoding.EncodeToString(content)
}

// readContentURL maps an API value of mapper_url or raw_idp_metadata_xml to
// state. A base64:// state value is kept because the API reports it as a
// storage URL. A null state value is read back inline. State is null after an
// import, and for mapper_url on every refresh while the configuration leaves
// it unset.
func readContentURL(ctx context.Context, apiValue string, prior types.String) types.String {
	switch {
	case apiValue == "":
		return prior
	case prior.IsNull():
		return types.StringValue(inlineStorageURL(ctx, apiValue))
	case strings.HasPrefix(prior.ValueString(), "base64://"):
		return prior
	default:
		return types.StringValue(apiValue)
	}
}

// isDefaultMapper reports whether the API value is the mapper Create sends
// when mapper_url is not set.
func isDefaultMapper(apiValue string) bool {
	content, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(defaultMapperURL, "base64://"))
	return apiValue == defaultMapperURL || helpers.URLHashMatchesContent(apiValue, content)
}
