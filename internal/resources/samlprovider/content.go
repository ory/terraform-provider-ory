package samlprovider

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

const base64ContentPrefix = "base64://"

// storageContentFetcher downloads the payload behind a storage URL the API
// reported. It is a package-level variable so unit tests can replace it.
var storageContentFetcher = func(ctx context.Context, rawURL string) ([]byte, error) {
	return client.FetchSafeHTTPS(ctx, "SAML provider content", rawURL)
}

// defaultMapperContent is the payload of defaultMapperURL. Create and Update
// send it when mapper_url is not configured.
var defaultMapperContent = func() []byte {
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(defaultMapperURL, base64ContentPrefix))
	if err != nil {
		panic(err)
	}
	return b
}()

// inlineStorageURLContent maps a storage URL the API reports back to the
// `base64://` form a configuration holds. The API uploads a `base64://` value
// and reports an https URL whose filename is the SHA-512 of the payload. It
// returns the API value unchanged when it is not a storage URL, or when the
// download fails, so a read never fails on a transient network error.
func inlineStorageURLContent(ctx context.Context, apiValue string) string {
	if _, ok := helpers.HashFromURL(apiValue); !ok {
		return apiValue
	}
	content, err := storageContentFetcher(ctx, apiValue)
	if err != nil {
		tflog.Warn(ctx, "Keeping the storage URL in state: downloading the stored payload failed",
			map[string]interface{}{"url": apiValue, "error": err.Error()})
		return apiValue
	}
	return base64ContentPrefix + base64.StdEncoding.EncodeToString(content)
}

// readMapperURL maps the API mapper_url to the state value.
//
// With a `base64://` value in state, the state value is kept, because the API
// reports the uploaded payload as a storage URL. With a URL in state, the API
// value is copied. With a null state value (after import, or when mapper_url
// is not configured), the default mapper stays null and any other mapper is
// read back, inlined as `base64://` when the API reports a storage URL, so an
// import of a provider with a custom mapper matches its configuration.
func readMapperURL(ctx context.Context, apiValue string, prior types.String) types.String {
	if prior.IsUnknown() {
		prior = types.StringNull()
	}
	if apiValue == "" {
		return prior
	}
	if !prior.IsNull() {
		if strings.HasPrefix(prior.ValueString(), base64ContentPrefix) {
			return prior
		}
		return types.StringValue(apiValue)
	}
	if apiValue == defaultMapperURL || helpers.URLHashMatchesContent(apiValue, defaultMapperContent) {
		return prior
	}
	return types.StringValue(inlineStorageURLContent(ctx, apiValue))
}

// readRawIDPMetadataXML maps the API raw_idp_metadata_xml to the state value.
// A `base64://` value in state is kept, a URL in state is replaced by the API
// value, and a null state value (after import) is read back, inlined as
// `base64://` when the API reports a storage URL.
func readRawIDPMetadataXML(ctx context.Context, apiValue string, prior types.String) types.String {
	if apiValue == "" {
		return prior
	}
	if prior.IsNull() || prior.IsUnknown() {
		return types.StringValue(inlineStorageURLContent(ctx, apiValue))
	}
	if strings.HasPrefix(prior.ValueString(), base64ContentPrefix) {
		return prior
	}
	return types.StringValue(apiValue)
}
