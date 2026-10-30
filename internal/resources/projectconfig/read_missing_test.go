package projectconfig

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestServiceStateFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state *ProjectConfigResourceModel
		want  map[string]bool
	}{
		{"empty", &ProjectConfigResourceModel{}, map[string]bool{"account_experience": false, "identity": false, "oauth2": false, "permission": false}},
		{"identity string", &ProjectConfigResourceModel{SessionLifespan: types.StringValue("20m0s")}, map[string]bool{"account_experience": false, "identity": true, "oauth2": false, "permission": false}},
		{"identity unknown counts", &ProjectConfigResourceModel{SessionLifespan: types.StringUnknown()}, map[string]bool{"account_experience": false, "identity": true, "oauth2": false, "permission": false}},
		{"identity int64", &ProjectConfigResourceModel{SelfserviceMethodsPasswordConfigMinPasswordLength: types.Int64Value(8)}, map[string]bool{"account_experience": false, "identity": true, "oauth2": false, "permission": false}},
		{"oauth2", &ProjectConfigResourceModel{OAuth2TTLAccessToken: types.StringValue("1h")}, map[string]bool{"account_experience": false, "identity": false, "oauth2": true, "permission": false}},
		{"account experience", &ProjectConfigResourceModel{AccountExperienceLocale: types.StringValue("en")}, map[string]bool{"account_experience": true, "identity": false, "oauth2": false, "permission": false}},
		{"permission bool", &ProjectConfigResourceModel{KetoFeatureFlagsStrictMode: types.BoolValue(true)}, map[string]bool{"account_experience": false, "identity": false, "oauth2": false, "permission": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, serviceStateFields(tc.state))
		})
	}
}
