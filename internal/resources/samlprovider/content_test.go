package samlprovider

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func storageURL(content []byte, ext string) string {
	sum := sha512.Sum512(content)
	return "https://storage.googleapis.com/bucket/" + hex.EncodeToString(sum[:]) + ext
}

// stubFetcher serves content for one storage URL and fails for every other URL.
func stubFetcher(t *testing.T, url string, content []byte) {
	t.Helper()
	original := storageContentFetcher
	storageContentFetcher = func(_ context.Context, rawURL string) ([]byte, error) {
		if rawURL != url {
			return nil, errors.New("unexpected fetch of " + rawURL)
		}
		return content, nil
	}
	t.Cleanup(func() { storageContentFetcher = original })
}

func TestReadMapperURL(t *testing.T) {
	ctx := context.Background()
	custom := []byte("local claims = std.extVar('claims'); { identity: { traits: { email: claims.email } } }")
	customInline := "base64://" + base64.StdEncoding.EncodeToString(custom)
	customURL := storageURL(custom, ".jsonnet")
	stubFetcher(t, customURL, custom)

	cases := map[string]struct {
		api   string
		prior types.String
		want  types.String
	}{
		"import reads a custom mapper back inline": {
			api: customURL, prior: types.StringNull(), want: types.StringValue(customInline),
		},
		"import keeps the default mapper null": {
			api: storageURL(defaultMapperContent, ".jsonnet"), prior: types.StringNull(), want: types.StringNull(),
		},
		"unconfigured mapper with the inline default stays null": {
			api: defaultMapperURL, prior: types.StringNull(), want: types.StringNull(),
		},
		"import reads a hosted mapper URL as is": {
			api: "https://example.com/mapper.jsonnet", prior: types.StringNull(), want: types.StringValue("https://example.com/mapper.jsonnet"),
		},
		"configured base64 value is kept": {
			api: customURL, prior: types.StringValue(customInline), want: types.StringValue(customInline),
		},
		"configured URL is refreshed": {
			api: "https://example.com/new.jsonnet", prior: types.StringValue("https://example.com/old.jsonnet"), want: types.StringValue("https://example.com/new.jsonnet"),
		},
		"missing API value keeps state": {
			api: "", prior: types.StringValue("https://example.com/old.jsonnet"), want: types.StringValue("https://example.com/old.jsonnet"),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, readMapperURL(ctx, tc.api, tc.prior))
		})
	}
}

func TestReadRawIDPMetadataXML(t *testing.T) {
	ctx := context.Background()
	xml := []byte(`<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com"/>`)
	inline := "base64://" + base64.StdEncoding.EncodeToString(xml)
	url := storageURL(xml, ".xml")
	stubFetcher(t, url, xml)

	assert.Equal(t, types.StringValue(inline), readRawIDPMetadataXML(ctx, url, types.StringNull()), "import reads the metadata back inline")
	assert.Equal(t, types.StringValue(inline), readRawIDPMetadataXML(ctx, url, types.StringValue(inline)), "configured base64 value is kept")
	assert.Equal(t, types.StringValue("https://idp.example.com/metadata"),
		readRawIDPMetadataXML(ctx, "https://idp.example.com/metadata", types.StringValue("https://idp.example.com/old")), "configured URL is refreshed")
	assert.Equal(t, types.StringNull(), readRawIDPMetadataXML(ctx, "", types.StringNull()))
}

// A failed download must not fail the read: the storage URL is kept, so the
// plan still shows the difference.
func TestInlineStorageURLContent_DownloadFails(t *testing.T) {
	url := storageURL([]byte("payload"), ".xml")
	stubFetcher(t, "https://other.example.com", nil)

	assert.Equal(t, url, inlineStorageURLContent(context.Background(), url))
}
