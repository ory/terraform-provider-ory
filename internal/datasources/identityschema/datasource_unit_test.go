package identityschema

import "testing"

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
			if got := isEmptySchemaBody(tt.input); got != tt.want {
				t.Errorf("isEmptySchemaBody(%q) = %v, want %v", string(tt.input), got, tt.want)
			}
		})
	}
}
