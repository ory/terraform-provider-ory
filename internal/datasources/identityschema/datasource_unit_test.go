package identityschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEmptySchemaBody(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{name: "empty object", input: []byte("{}"), want: true},
		{name: "null", input: []byte("null"), want: true},
		{name: "empty string", input: []byte(""), want: true},
		{name: "valid schema", input: []byte(`{"type":"object"}`), want: false},
		{name: "nested schema", input: []byte(`{"$id":"test","properties":{}}`), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isEmptySchemaBody(tt.input), "isEmptySchemaBody(%q)", string(tt.input))
		})
	}
}
