package samlprovider

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func storageURL(content []byte) string {
	sum := sha512.Sum512(content)
	return "https://storage.googleapis.com/bucket/" + hex.EncodeToString(sum[:]) + ".xml"
}

func TestReadContentURL(t *testing.T) {
	content := []byte("<EntityDescriptor/>")
	inline := "base64://" + base64.StdEncoding.EncodeToString(content)
	url := storageURL(content)
	original := storageContentFetcher
	t.Cleanup(func() { storageContentFetcher = original })
	storageContentFetcher = func(_ context.Context, rawURL string) ([]byte, error) {
		if rawURL != url {
			return nil, errors.New("not found")
		}
		return content, nil
	}
	ctx := context.Background()

	assert.Equal(t, types.StringValue(inline), readContentURL(ctx, url, types.StringNull()), "import reads the payload back inline")
	assert.Equal(t, types.StringValue(inline), readContentURL(ctx, url, types.StringValue(inline)))
	assert.Equal(t, types.StringValue("https://idp.example.com/new"), readContentURL(ctx, "https://idp.example.com/new", types.StringValue("https://idp.example.com/old")))
	assert.Equal(t, types.StringNull(), readContentURL(ctx, "", types.StringNull()))

	failing := storageURL([]byte("other"))
	assert.Equal(t, types.StringValue(failing), readContentURL(ctx, failing, types.StringNull()), "a failed download keeps the URL")
	assert.Equal(t, types.StringValue(url), readContentURL(ctx, url, types.StringValue(url)), "a storage URL in state from an older import stays as is")
	unpadded := strings.TrimRight(inline, "=")
	assert.Equal(t, types.StringValue(unpadded), readContentURL(ctx, url, types.StringValue(unpadded)), "unpadded base64 is kept")

	changed := []byte("<EntityDescriptor changed/>")
	changedURL := storageURL(changed)
	storageContentFetcher = func(context.Context, string) ([]byte, error) { return changed, nil }
	assert.Equal(t, types.StringValue("base64://"+base64.StdEncoding.EncodeToString(changed)), readContentURL(ctx, changedURL, types.StringValue(inline)), "content changed outside Terraform is read back")

	// The download returns content whose hash does not match the URL.
	storageContentFetcher = func(context.Context, string) ([]byte, error) { return []byte("blocked"), nil }
	assert.Equal(t, types.StringValue(url), readContentURL(ctx, url, types.StringNull()), "content that does not match the hash keeps the URL")
}

func TestIsDefaultMapper(t *testing.T) {
	content, _ := base64.StdEncoding.DecodeString(defaultMapperURL[len("base64://"):])

	assert.True(t, isDefaultMapper(defaultMapperURL))
	assert.True(t, isDefaultMapper(storageURL(content)))
	assert.False(t, isDefaultMapper(storageURL([]byte("custom"))))
	assert.False(t, isDefaultMapper(""))
}
