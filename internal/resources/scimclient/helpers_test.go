package scimclient

import (
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every caller branches on the length of the result to choose between
// writing the whole array and appending, and indexes it to address a patch.
func TestClientsFromRevision(t *testing.T) {
	tests := []struct {
		name     string
		revision map[string]interface{}
		wantLen  int
		wantIDs  []string
	}{
		{"nil revision", nil, 0, nil},
		{"no scim_clients key", map[string]interface{}{"id": "rev-1"}, 0, nil},
		{"scim_clients is null", map[string]interface{}{"scim_clients": nil}, 0, nil},
		{"scim_clients is not an array", map[string]interface{}{"scim_clients": "okta"}, 0, nil},
		{"empty array", map[string]interface{}{"scim_clients": []interface{}{}}, 0, nil},
		{
			"two clients",
			map[string]interface{}{"scim_clients": []interface{}{
				map[string]interface{}{"client_id": "okta"},
				map[string]interface{}{"client_id": "azure"},
			}},
			2, []string{"okta", "azure"},
		},
		{
			// A non-object element keeps its slot so later indexes still match
			// the positions the API addresses.
			"non-object element keeps its index",
			map[string]interface{}{"scim_clients": []interface{}{
				"garbage",
				map[string]interface{}{"client_id": "azure"},
			}},
			2, []string{"", "azure"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clientsFromRevision(tt.revision)
			require.NotNil(t, got, "the result must always be a usable slice")
			require.Len(t, got, tt.wantLen)
			for i, want := range tt.wantIDs {
				id, _ := got[i]["client_id"].(string)
				assert.Equal(t, want, id)
			}
		})
	}
}

func TestFindClientIndex(t *testing.T) {
	clients := []map[string]interface{}{
		{"client_id": "okta", "id": "row-1"},
		{},
		{"client_id": "azure", "id": "row-2"},
	}
	assert.Equal(t, 0, findClientIndex(clients, "okta"))
	assert.Equal(t, 2, findClientIndex(clients, "azure"))
	assert.Equal(t, -1, findClientIndex(clients, "google"))
	assert.Equal(t, -1, findClientIndex(nil, "okta"))
	assert.Equal(t, "/scim_clients/2", clientPath(2))
	assert.Equal(t, "row-2", rowID(clients[2]))
	assert.Equal(t, "", rowID(clients[1]))
}

func TestBuildClientObject(t *testing.T) {
	plan := &SCIMClientResourceModel{
		OrganizationID: types.StringValue("org-1"),
		ClientID:       types.StringValue("okta"),
		Label:          types.StringValue("Okta"),
		MapperURL:      types.StringValue("base64://abc"),
		State:          types.StringValue(stateDisabled),
	}

	got := buildClientObject(plan, "s3cret", "row-1")
	assert.Equal(t, map[string]interface{}{
		"id":                          "row-1",
		"client_id":                   "okta",
		"label":                       "Okta",
		"organization_id":             "org-1",
		"mapper_url":                  "base64://abc",
		"authorization_header_secret": "s3cret",
		"state":                       "disabled",
	}, got)

	// A fresh create carries no row id, and the API rejects a missing state
	// with HTTP 500, so an unset state must still be sent as enabled.
	plan.State = types.StringNull()
	got = buildClientObject(plan, "s3cret", "")
	_, hasID := got["id"]
	assert.False(t, hasID, "a create must not invent a row id")
	assert.Equal(t, "enabled", got["state"])
}

func TestVerifyWritten(t *testing.T) {
	revision := map[string]interface{}{"scim_clients": []interface{}{
		map[string]interface{}{"client_id": "okta"},
	}}

	var diags diag.Diagnostics
	assert.True(t, verifyWritten(revision, "okta", &diags))
	assert.False(t, diags.HasError())

	assert.False(t, verifyWritten(revision, "azure", &diags))
	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), `"azure"`)
}

func TestStringOrNull(t *testing.T) {
	assert.Equal(t, types.StringValue("x"), stringOrNull("x"))
	assert.True(t, stringOrNull("").IsNull(), "an empty string is the API's way of omitting a value")
	assert.True(t, stringOrNull(nil).IsNull())
	assert.True(t, stringOrNull(42).IsNull())
}

func TestSplitImportID(t *testing.T) {
	projectID, clientID := splitImportID("proj-1/okta")
	assert.Equal(t, "proj-1", projectID)
	assert.Equal(t, "okta", clientID)

	projectID, clientID = splitImportID("okta")
	assert.Equal(t, "", projectID)
	assert.Equal(t, "okta", clientID)

	_, clientID = splitImportID("proj-1/")
	assert.Equal(t, "", clientID, "a trailing slash carries no client id")
}

// The API reports a missing organization as a raw foreign key violation
// inside an HTTP 500. The message must name the attribute that is wrong.
func TestAPIErrorDetail(t *testing.T) {
	fk := errors.New(`patching project revision: HTTP 500: insert on table "project_revision_scim_clients" violates foreign key constraint "project_revision_scim_clients_organization_fkey"`)
	got := apiErrorDetail(fk, "proj-1")
	assert.Contains(t, got, "project_revision_scim_clients_organization_fkey", "the raw cause must stay visible")
	assert.Contains(t, got, "organization_id must be the ID of an organization in project proj-1")

	uuid := errors.New(`patching project revision: HTTP 400: uuid: incorrect UUID length 10 in string "not-a-uuid"`)
	assert.Contains(t, apiErrorDetail(uuid, "proj-1"), "organization_id must be the UUID")

	other := errors.New("patching project revision: HTTP 400: does not match pattern")
	assert.Equal(t, other.Error(), apiErrorDetail(other, "proj-1"), "an unrelated error passes through unchanged")
}
