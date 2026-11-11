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
