package projectconfig

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// base64ContentPrefix marks a config value that carries its payload inline,
// base64-encoded, instead of pointing at a URL. The Ory API accepts this form
// for Jsonnet payloads such as the courier HTTP request body.
const base64ContentPrefix = "base64://"

// storageURLContentFetcher downloads the payload behind a storage URL the API
// reported. It is a package-level variable so unit tests can replace it. The
// default implementation delegates to client.FetchSafeHTTPS, which rejects
// non-HTTPS schemes, refuses private and loopback hosts, limits redirects, and
// caps the response body at 1 MiB.
var storageURLContentFetcher = func(ctx context.Context, rawURL string) ([]byte, error) {
	return client.FetchSafeHTTPS(ctx, "project config content", rawURL)
}

// resolveStorageURLContent maps an API value back to the form the configuration
// holds, for an attribute whose payload the Ory API uploads to object storage.
//
// A write sends `base64://<payload>`. The API stores the decoded payload and
// reports an https URL whose filename is the lowercase hex SHA-512 of that
// payload. Copying the URL into state makes every later plan show a change,
// because the configuration still holds the base64 value, and no apply settles
// it. See https://github.com/ory/terraform-provider-ory/issues/315.
//
// The function returns:
//   - the API value, when it is empty, when it already equals the state value,
//     or when it is not a storage URL;
//   - the state value, when the hash in the storage URL matches the payload the
//     state value carries, because the stored payload is the configured one;
//   - the downloaded payload re-encoded as `base64://`, when the hash differs,
//     because the payload changed outside Terraform and the plan must show it;
//   - the API value, when the download fails, so a transient network error
//     still surfaces drift instead of hiding it.
func resolveStorageURLContent(ctx context.Context, apiValue, stateValue string) string {
	if apiValue == "" || apiValue == stateValue {
		return apiValue
	}
	if _, ok := helpers.HashFromURL(apiValue); !ok {
		return apiValue
	}
	payload, ok := decodeBase64Content(stateValue)
	if !ok {
		// State holds a URL rather than an inline payload, so the API value is
		// already comparable with it.
		return apiValue
	}
	if helpers.URLHashMatchesContent(apiValue, payload) {
		return stateValue
	}
	fetched, err := storageURLContentFetcher(ctx, apiValue)
	if err != nil {
		tflog.Warn(ctx, "Keeping the storage URL in state: downloading the stored payload failed",
			map[string]interface{}{"url": apiValue, "error": err.Error()})
		return apiValue
	}
	return base64ContentPrefix + base64.StdEncoding.EncodeToString(fetched)
}

// decodeBase64Content decodes a `base64://` config value to its payload. It
// reports false when the value does not use that form, or when the encoded part
// is not valid base64. The Ory API accepts both padded and unpadded base64, so
// both are decoded here.
func decodeBase64Content(value string) ([]byte, bool) {
	if !strings.HasPrefix(value, base64ContentPrefix) {
		return nil, false
	}
	encoded := strings.TrimPrefix(value, base64ContentPrefix)
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return decoded, true
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return decoded, true
	}
	return nil, false
}

// getNestedValue safely traverses nested maps to extract a value.
func getNestedValue(config map[string]interface{}, keys ...string) interface{} {
	current := interface{}(config)
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[key]
		if !ok {
			return nil
		}
	}
	return current
}

// getNestedString extracts a string from nested maps, returning ("", false) if not found.
func getNestedString(config map[string]interface{}, keys ...string) (string, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// getNestedBool extracts a bool from nested maps, returning (false, false) if not found.
func getNestedBool(config map[string]interface{}, keys ...string) (bool, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// getNestedFloat extracts a number from nested maps (JSON numbers are float64).
func getNestedFloat(config map[string]interface{}, keys ...string) (float64, bool) {
	v := getNestedValue(config, keys...)
	if v == nil {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}
