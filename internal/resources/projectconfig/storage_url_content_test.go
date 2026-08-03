package projectconfig

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// jsonnetPayload stands in for a courier request-body template.
	jsonnetPayload = "{\n  \"recipient\": {{ .recipient }},\n  \"body\": {{ .body }}\n}"
	// otherPayload is what someone edits the template to in the Ory Console.
	otherPayload = "{\n  \"changed\": true\n}"
)

// storageURL builds the URL shape the Ory API reports for a stored payload:
// the filename is the lowercase hex SHA-512 of the payload.
func storageURL(payload string) string {
	sum := sha512.Sum512([]byte(payload))
	return "https://storage.googleapis.com/bac-gcs-staging/" + hex.EncodeToString(sum[:]) + ".jsonnet"
}

// base64Value renders a payload the way a user writes it in HCL.
func base64Value(payload string) string {
	return base64ContentPrefix + base64.StdEncoding.EncodeToString([]byte(payload))
}

// stubFetcher replaces storageURLContentFetcher for one test and records how
// often it ran, so a test can assert the hash fast path avoided the network.
func stubFetcher(t *testing.T, payload string, err error) *int {
	t.Helper()
	calls := 0
	original := storageURLContentFetcher
	storageURLContentFetcher = func(_ context.Context, _ string) ([]byte, error) {
		calls++
		if err != nil {
			return nil, err
		}
		return []byte(payload), nil
	}
	t.Cleanup(func() { storageURLContentFetcher = original })
	return &calls
}

func TestDecodeBase64Content(t *testing.T) {
	padded := base64.StdEncoding.EncodeToString([]byte(jsonnetPayload))
	unpadded := base64.RawStdEncoding.EncodeToString([]byte(jsonnetPayload))

	tests := []struct {
		name    string
		value   string
		want    string
		wantOK  bool
		comment string
	}{
		{"padded", base64ContentPrefix + padded, jsonnetPayload, true, "the form the docs and examples use"},
		{"unpadded", base64ContentPrefix + unpadded, jsonnetPayload, true, "the Ory API accepts this too"},
		{"empty payload", base64ContentPrefix, "", true, "an empty payload is still the inline form"},
		{"no prefix", "https://example.com/body.jsonnet", "", false, "a URL is not an inline payload"},
		{"bad base64", base64ContentPrefix + "not base64!!", "", false, "undecodable, so not usable for a hash compare"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeBase64Content(tt.value)
			assert.Equal(t, tt.wantOK, ok, tt.comment)
			if tt.wantOK {
				assert.Equal(t, tt.want, string(got))
			}
		})
	}
}

// TestResolveStorageURLContent_KeepsConfiguredValueOnHashMatch is the direct
// guard for issue #315: the API replaces the configured `base64://` value with a
// storage URL, and copying that URL into state makes every plan show a change
// that no apply settles.
func TestResolveStorageURLContent_KeepsConfiguredValueOnHashMatch(t *testing.T) {
	configured := base64Value(jsonnetPayload)
	calls := stubFetcher(t, "", errors.New("the hash fast path must not download anything"))

	got := resolveStorageURLContent(context.Background(), storageURL(jsonnetPayload), configured)

	assert.Equal(t, configured, got, "the configured base64 value must stay in state")
	assert.Zero(t, *calls, "a matching hash must settle the read without a download")
}

// The Ory API accepts unpadded base64, so a user who writes it must get the same
// no-diff behavior as one who writes the padded form.
func TestResolveStorageURLContent_KeepsUnpaddedConfiguredValue(t *testing.T) {
	configured := base64ContentPrefix + base64.RawStdEncoding.EncodeToString([]byte(jsonnetPayload))
	calls := stubFetcher(t, "", errors.New("the hash fast path must not download anything"))

	got := resolveStorageURLContent(context.Background(), storageURL(jsonnetPayload), configured)

	assert.Equal(t, configured, got, "an unpadded base64 value must stay in state")
	assert.Zero(t, *calls, "a matching hash must settle the read without a download")
}

// A payload changed outside Terraform must reach state, so the plan shows the
// drift. The downloaded payload is re-encoded as `base64://` to keep the diff
// comparable with the configuration.
func TestResolveStorageURLContent_ReportsOutOfBandChange(t *testing.T) {
	calls := stubFetcher(t, otherPayload, nil)

	got := resolveStorageURLContent(context.Background(), storageURL(otherPayload), base64Value(jsonnetPayload))

	assert.Equal(t, base64Value(otherPayload), got, "the changed payload must surface as drift")
	assert.Equal(t, 1, *calls, "a hash mismatch must download the stored payload once")
}

func TestResolveStorageURLContent_FallsBackToURLWhenDownloadFails(t *testing.T) {
	apiValue := storageURL(otherPayload)
	calls := stubFetcher(t, "", errors.New("connection reset"))

	got := resolveStorageURLContent(context.Background(), apiValue, base64Value(jsonnetPayload))

	assert.Equal(t, apiValue, got, "a failed download must still surface drift, not hide it")
	assert.Equal(t, 1, *calls)
}

func TestResolveStorageURLContent_PassesThroughNonStorageValues(t *testing.T) {
	sameURL := storageURL(jsonnetPayload)

	tests := []struct {
		name     string
		apiValue string
		state    string
		want     string
		comment  string
	}{
		{"cleared out of band", "", base64Value(jsonnetPayload), "", "an empty value must null the attribute so removal shows as drift"},
		{"identical to state", sameURL, sameURL, sameURL, "nothing to resolve when the API echoes the state value"},
		{"state holds a URL", storageURL(otherPayload), sameURL, storageURL(otherPayload), "two URLs already compare directly"},
		{"not a storage URL", "https://example.com/body.jsonnet", base64Value(jsonnetPayload), "https://example.com/body.jsonnet", "no hash to compare, so report the value verbatim"},
		{"state is empty", sameURL, "", sameURL, "nothing in state to preserve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := stubFetcher(t, "", errors.New("no download expected on this path"))

			got := resolveStorageURLContent(context.Background(), tt.apiValue, tt.state)

			assert.Equal(t, tt.want, got, tt.comment)
			assert.Zero(t, *calls, "this path must not download anything")
		})
	}
}

// courierBodyProject nests a courier HTTP request-body value the way the API
// reports it.
func courierBodyProject(body string) map[string]interface{} {
	return map[string]interface{}{
		"courier": map[string]interface{}{
			"http": map[string]interface{}{
				"request_config": map[string]interface{}{
					"body": body,
				},
			},
		},
	}
}

// TestReadSimpleFields_ResolvesCourierBodyStorageURL wires the resolver check
// through the generated read path, which is what a refresh actually runs.
func TestReadSimpleFields_ResolvesCourierBodyStorageURL(t *testing.T) {
	configured := base64Value(jsonnetPayload)
	calls := stubFetcher(t, "", errors.New("the hash fast path must not download anything"))

	state := &ProjectConfigResourceModel{
		CourierHTTPRequestConfigBody: types.StringValue(configured),
	}
	readSimpleFields(context.Background(), projectWithIdentityConfig(courierBodyProject(storageURL(jsonnetPayload))), state)

	assert.Equal(t, configured, state.CourierHTTPRequestConfigBody.ValueString(),
		"a refresh must leave the configured courier body in state")
	assert.Zero(t, *calls)
}

func TestReadSimpleFields_NullsCourierBodyRemovedOutOfBand(t *testing.T) {
	state := &ProjectConfigResourceModel{
		CourierHTTPRequestConfigBody: types.StringValue(base64Value(jsonnetPayload)),
	}
	readSimpleFields(context.Background(), projectWithIdentityConfig(map[string]interface{}{
		"courier": map[string]interface{}{"http": map[string]interface{}{"request_config": map[string]interface{}{}}},
	}), state)

	assert.True(t, state.CourierHTTPRequestConfigBody.IsNull(),
		"a courier body removed outside Terraform must null state so the removal shows as drift")
}

// TestGeneratedReadEntries_StorageURLFlag pins the codegen wiring: exactly the
// attributes marked `storage_url_content` in mappings.yaml get the resolver.
func TestGeneratedReadEntries_StorageURLFlag(t *testing.T) {
	state := &ProjectConfigResourceModel{}

	var storageURLKeys []string
	for _, e := range identityStringReadEntries(state) {
		if e.StorageURL {
			storageURLKeys = append(storageURLKeys, strings.Join(e.Keys, "."))
		}
		if e.Field == &state.CourierHTTPRequestConfigBody {
			assert.True(t, e.StorageURL,
				"courier_http_request_config_body must resolve storage URLs — see issue #315")
		}
	}
	require.Equal(t, []string{"courier.http.request_config.body"}, storageURLKeys,
		"unexpected set of storage-URL attributes; update this test when mappings.yaml changes")

	// No other service reports storage-backed content today. A new one showing up
	// here means the mapping changed and the resolver behavior needs a look.
	for name, entries := range map[string][]StringReadEntry{
		"oauth2":             oauth2StringReadEntries(state),
		"account_experience": account_experienceStringReadEntries(state),
		"permission":         permissionStringReadEntries(state),
	} {
		for _, e := range entries {
			assert.False(t, e.StorageURL, fmt.Sprintf("%s has an unexpected storage-URL attribute", name))
		}
	}
}
