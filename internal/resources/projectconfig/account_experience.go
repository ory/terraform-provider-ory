package projectconfig

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/helpers"
)

// Account experience images (logo/favicon, light/dark) need custom handling
// that the codegen can't express:
//
//   - The OpenAPI spec's governs descriptions point at favicon_*/logo_* config
//     keys, but the API actually stores the values at favicon_*_url/logo_*_url.
//     Patching the governs-derived paths succeeds (HTTP 200) but is silently
//     ignored — the root cause of issue #250.
//   - Writes must be an inline data URI (data:image/png;base64,...); plain
//     remote URLs are rejected because the API sniffs the value's content type.
//   - Reads return a content-addressed storage URL whose filename is the
//     SHA-512 of the raw image bytes, so state (the data URI) never equals the
//     API value. Drift is detected by comparing that hash against the hash of
//     the data URI payload, mirroring ory_email_template.

// axImageMIMETypes are the image content types accepted by the Ory API for
// account experience logos and favicons (from the API's validation error).
const axImageMIMETypes = `png|svg\+xml|x-icon|vnd\.microsoft\.icon|gif|jpeg|webp`

// axImageValueRegex accepts the three shapes the Ory API allows for an account
// experience image:
//
//   - an empty string (clears the image);
//   - an inline data URI of a supported image type, which the API uploads;
//   - a storage URL the API has already returned, whose filename stem is the
//     lowercase hex SHA-512 of the image bytes (see helpers.HashFromURL).
//
// Plain remote URLs are rejected by the API (it sniffs the value's content
// type rather than fetching the URL), so we reject them here too — surfacing
// the error at plan time instead of deferring it to apply.
const axStorageURLPattern = `https://[^\s]+/[0-9a-fA-F]{128}(\.[A-Za-z0-9]+)?(\?[^\s]*)?`

var axImageValueRegex = regexp.MustCompile(
	`^(` +
		`|` + axStorageURLPattern +
		`|data:image/(` + axImageMIMETypes + `);base64,[A-Za-z0-9+/]+={0,2}` +
		`)$`,
)

// axImageField pairs a state/plan field with its config key under
// /services/account_experience/config/.
type axImageField struct {
	field *types.String
	key   string
}

func accountExperienceImageFields(m *ProjectConfigResourceModel) []axImageField {
	return []axImageField{
		{&m.AccountExperienceFaviconDark, "favicon_dark_url"},
		{&m.AccountExperienceFaviconLight, "favicon_light_url"},
		{&m.AccountExperienceLogoDark, "logo_dark_url"},
		{&m.AccountExperienceLogoLight, "logo_light_url"},
	}
}

// accountExperienceImageSchemaAttrs returns the hand-written schema attributes
// for the account experience image fields.
func accountExperienceImageSchemaAttrs() map[string]schema.Attribute {
	imageAttr := func(what, theme string) schema.StringAttribute {
		return schema.StringAttribute{
			Description: what + " for the hosted Account Experience UI (" + theme + " theme). " +
				"Must be an inline data URI (e.g. data:image/png;base64,...) or a storage URL " +
				"previously returned by the API. The API uploads the image and serves it from a " +
				"content-addressed storage URL; the provider matches that URL against the data URI " +
				"by content hash to detect drift. Set to an empty string to remove.",
			Optional: true,
			Validators: []validator.String{
				stringvalidator.RegexMatches(
					axImageValueRegex,
					"must be empty, a storage URL previously returned by the Ory API, or a "+
						"data:image/<type>;base64,<data> URI (supported types: png, svg+xml, "+
						"x-icon, vnd.microsoft.icon, gif, jpeg, webp). Plain remote URLs are not "+
						"accepted — embed the image as a data URI, e.g. with filebase64()",
				),
			},
		}
	}
	return map[string]schema.Attribute{
		"account_experience_favicon_dark":  imageAttr("Favicon", "dark"),
		"account_experience_favicon_light": imageAttr("Favicon", "light"),
		"account_experience_logo_dark":     imageAttr("Logo", "dark"),
		"account_experience_logo_light":    imageAttr("Logo", "light"),
	}
}

// accountExperienceImagePatches returns the JSON Patch operations for the
// account experience image fields. Values pass through unchanged: the API
// accepts a data URI (which it uploads) or one of its own storage URLs, and
// an empty string clears the image.
func accountExperienceImagePatches(plan *ProjectConfigResourceModel) []ory.JsonPatch {
	var patches []ory.JsonPatch
	for _, e := range accountExperienceImageFields(plan) {
		if !e.field.IsNull() && !e.field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  "/services/account_experience/config/" + e.key,
				Value: e.field.ValueString(),
			})
		}
	}
	return patches
}

// readAccountExperienceImages reads the image fields from the API response
// into state, using content-hash matching to avoid perpetual diffs.
func readAccountExperienceImages(project *ory.Project, state *ProjectConfigResourceModel) {
	if project.Services.AccountExperience == nil {
		return
	}
	axConfig := project.Services.AccountExperience.Config
	for _, e := range accountExperienceImageFields(state) {
		if e.field.IsNull() {
			continue // attribute not managed in this configuration
		}
		if apiValue, ok := getNestedString(axConfig, e.key); ok {
			*e.field = resolveAccountExperienceImage(apiValue, e.field.ValueString())
		}
	}
}

// resolveAccountExperienceImage returns the state value to store for an image
// field given the API value and the previous state value:
//
//   - API value equals state (storage-URL passthrough, or both empty): keep state.
//   - State holds a data URI whose payload hash matches the hash embedded in
//     the API's storage URL filename: the image is unchanged, keep state.
//   - Anything else (cleared in Console, replaced out-of-band, malformed state):
//     surface the API value so the next plan shows the drift.
func resolveAccountExperienceImage(apiValue, stateValue string) types.String {
	if apiValue == stateValue {
		return types.StringValue(stateValue)
	}
	if payload, ok := dataURIPayload(stateValue); ok && helpers.URLHashMatchesContent(apiValue, payload) {
		return types.StringValue(stateValue)
	}
	return types.StringValue(apiValue)
}

// dataURIPayload decodes the base64 payload of a data URI
// (data:<mediatype>;base64,<data>). Returns (nil, false) when the value is
// not a base64 data URI or the payload fails to decode.
func dataURIPayload(s string) ([]byte, bool) {
	if !strings.HasPrefix(s, "data:") {
		return nil, false
	}
	meta, data, found := strings.Cut(s[len("data:"):], ",")
	if !found || !strings.HasSuffix(meta, ";base64") {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, false
	}
	return decoded, true
}
