package socialprovider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestReadOptionalString(t *testing.T) {
	tests := []struct {
		name     string
		provider map[string]interface{}
		want     types.String
	}{
		{name: "value", provider: map[string]interface{}{"subject_source": "me"}, want: types.StringValue("me")},
		{name: "missing key", provider: map[string]interface{}{}, want: types.StringNull()},
		{name: "empty string", provider: map[string]interface{}{"subject_source": ""}, want: types.StringNull()},
		{name: "wrong type", provider: map[string]interface{}{"subject_source": true}, want: types.StringNull()},
		{name: "nil value", provider: map[string]interface{}{"subject_source": nil}, want: types.StringNull()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, readOptionalString(tt.provider, "subject_source"))
		})
	}
}

func TestReadJSONObject(t *testing.T) {
	api := map[string]interface{}{"requested_claims": map[string]interface{}{
		"id_token": map[string]interface{}{"email": nil, "max": map[string]interface{}{"value": 1.5}},
	}}
	encoded := types.StringValue(`{"id_token":{"email":null,"max":{"value":1.5}}}`)
	heredoc := types.StringValue("{\n  \"id_token\": {\"max\": {\"value\": 1.50}, \"email\": null}\n}\n")

	tests := []struct {
		name     string
		provider map[string]interface{}
		prior    types.String
		want     types.String
	}{
		{name: "missing key", provider: map[string]interface{}{}, prior: encoded, want: types.StringNull()},
		{name: "not an object", provider: map[string]interface{}{"requested_claims": "x"}, prior: encoded, want: types.StringNull()},
		{name: "json null", provider: map[string]interface{}{"requested_claims": nil}, prior: types.StringNull(), want: types.StringNull()},
		{name: "import encodes like jsonencode", provider: api, prior: types.StringNull(), want: encoded},
		{name: "same object in other formatting keeps state", provider: api, prior: heredoc, want: heredoc},
		{name: "changed object replaces state", provider: api, prior: types.StringValue(`{"id_token":{"email":null}}`), want: encoded},
		{name: "invalid prior is replaced", provider: api, prior: types.StringValue("not json"), want: encoded},
		{name: "empty object", provider: map[string]interface{}{"requested_claims": map[string]interface{}{}}, prior: types.StringNull(), want: types.StringValue("{}")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, readJSONObject(tt.provider, "requested_claims", tt.prior))
		})
	}
}
