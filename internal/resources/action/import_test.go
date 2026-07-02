package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProjectID = "550e8400-e29b-41d4-a716-446655440000"

// TestParseActionImportID_valid covers the import ID formats the provider
// documents, including the issue #280 regression: urls whose own colons (and
// nested schemes in query params) must not be mistaken for segment separators
// or an invalid HTTP method.
func TestParseActionImportID_valid(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want actionImportID
	}{
		{
			name: "after with explicit method",
			id:   testProjectID + ":registration:after:password:POST:https://api.example.com/webhook",
			want: actionImportID{projectID: testProjectID, flow: "registration", timing: "after", authMethod: "password", httpMethod: "POST", url: "https://api.example.com/webhook"},
		},
		{
			name: "after legacy without method defaults to POST",
			id:   testProjectID + ":registration:after:password:https://api.example.com/webhook",
			want: actionImportID{projectID: testProjectID, flow: "registration", timing: "after", authMethod: "password", httpMethod: "POST", url: "https://api.example.com/webhook"},
		},
		{
			name: "after with placeholder auth_method",
			id:   testProjectID + ":verification:after:_:PATCH:https://api.example.com/hook",
			want: actionImportID{projectID: testProjectID, flow: "verification", timing: "after", authMethod: "password", httpMethod: "PATCH", url: "https://api.example.com/hook"},
		},
		{
			name: "before with explicit method",
			id:   testProjectID + ":login:before:PATCH:https://api.example.com/validate",
			want: actionImportID{projectID: testProjectID, flow: "login", timing: "before", authMethod: "password", httpMethod: "PATCH", url: "https://api.example.com/validate"},
		},
		{
			name: "before legacy without method defaults to POST",
			id:   testProjectID + ":login:before:https://api.example.com/validate",
			want: actionImportID{projectID: testProjectID, flow: "login", timing: "before", authMethod: "password", httpMethod: "POST", url: "https://api.example.com/validate"},
		},
		{
			name: "lowercase method is normalized",
			id:   testProjectID + ":login:before:post:https://api.example.com/validate",
			want: actionImportID{projectID: testProjectID, flow: "login", timing: "before", authMethod: "password", httpMethod: "POST", url: "https://api.example.com/validate"},
		},
		{
			name: "url with port and no method",
			id:   testProjectID + ":login:before:https://api.example.com:8443/validate",
			want: actionImportID{projectID: testProjectID, flow: "login", timing: "before", authMethod: "password", httpMethod: "POST", url: "https://api.example.com:8443/validate"},
		},
		{
			// Regression for the CodeRabbit finding on #280: a nested scheme in a
			// query param must not be read as an invalid method when the method
			// segment is omitted.
			name: "before without method, url embeds a nested scheme",
			id:   testProjectID + ":login:before:https://api.example.com/cb?redirect=https://other.example.com",
			want: actionImportID{projectID: testProjectID, flow: "login", timing: "before", authMethod: "password", httpMethod: "POST", url: "https://api.example.com/cb?redirect=https://other.example.com"},
		},
		{
			name: "after without method, url embeds a nested scheme",
			id:   testProjectID + ":registration:after:password:https://api.example.com/cb?redirect=https://other.example.com",
			want: actionImportID{projectID: testProjectID, flow: "registration", timing: "after", authMethod: "password", httpMethod: "POST", url: "https://api.example.com/cb?redirect=https://other.example.com"},
		},
		{
			name: "before with method, url embeds a nested scheme",
			id:   testProjectID + ":login:before:PUT:https://api.example.com/cb?redirect=https://other.example.com",
			want: actionImportID{projectID: testProjectID, flow: "login", timing: "before", authMethod: "password", httpMethod: "PUT", url: "https://api.example.com/cb?redirect=https://other.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, detail := parseActionImportID(tt.id)
			require.Empty(t, detail, "expected no error detail")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseActionImportID_invalid verifies that malformed IDs return a
// descriptive diagnostic detail instead of parsing into bogus fields.
func TestParseActionImportID_invalid(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantDetail string
	}{
		{
			name:       "too few segments",
			id:         testProjectID + ":login",
			wantDetail: "Import ID must be in one of these formats",
		},
		{
			name:       "invalid timing",
			id:         testProjectID + ":login:sideways:https://api.example.com/hook",
			wantDetail: "Invalid timing 'sideways'",
		},
		{
			name:       "after missing url after auth_method",
			id:         testProjectID + ":registration:after:password",
			wantDetail: "Missing url after auth_method",
		},
		{
			name:       "invalid http method with url following",
			id:         testProjectID + ":login:before:BOGUS:https://api.example.com/hook",
			wantDetail: "Invalid HTTP method 'BOGUS'",
		},
		{
			name:       "empty url",
			id:         testProjectID + ":login:before:",
			wantDetail: "Missing url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, detail := parseActionImportID(tt.id)
			require.NotEmpty(t, detail, "expected an error detail")
			assert.Contains(t, detail, tt.wantDetail)
			assert.Equal(t, actionImportID{}, got, "no fields should be returned on error")
		})
	}
}
