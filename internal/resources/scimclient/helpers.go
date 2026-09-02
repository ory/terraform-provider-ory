package scimclient

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// clientsFromRevision returns the scim_clients array of a normalized project
// revision. A revision without the array, or with a value of another type,
// yields an empty slice. An element that is not an object is kept as an empty
// map, so every index in the result matches the index the API addresses.
func clientsFromRevision(revision map[string]interface{}) []map[string]interface{} {
	raw, _ := revision["scim_clients"].([]interface{})
	clients := make([]map[string]interface{}, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]interface{})
		if !ok {
			m = map[string]interface{}{}
		}
		clients = append(clients, m)
	}
	return clients
}

// findClientIndex returns the index of the client with the given client_id,
// or -1. client_id is the stable identity: the row id changes on every
// revision the project writes.
func findClientIndex(clients []map[string]interface{}, clientID string) int {
	for i, c := range clients {
		if id, _ := c["client_id"].(string); id == clientID {
			return i
		}
	}
	return -1
}

// clientPath is the JSON Pointer of the client at index.
func clientPath(index int) string {
	return fmt.Sprintf("%s/%d", scimClientsPath, index)
}

// rowID returns the database id of a client read from the revision, or "".
func rowID(c map[string]interface{}) string {
	id, _ := c["id"].(string)
	return id
}

// buildClientObject renders the plan as the object the revision array
// stores. state is always sent: the API answers HTTP 500 when it is absent.
// A non-empty id makes the server match the stored row strictly by id.
func buildClientObject(plan *SCIMClientResourceModel, secret, id string) map[string]interface{} {
	state := stateEnabled
	if !plan.State.IsNull() && !plan.State.IsUnknown() && plan.State.ValueString() != "" {
		state = plan.State.ValueString()
	}
	obj := map[string]interface{}{
		"client_id":                   plan.ClientID.ValueString(),
		"label":                       plan.Label.ValueString(),
		"organization_id":             plan.OrganizationID.ValueString(),
		"mapper_url":                  plan.MapperURL.ValueString(),
		"authorization_header_secret": secret,
		"state":                       state,
	}
	if id != "" {
		obj["id"] = id
	}
	return obj
}

// verifyWritten checks that the revision the API returned after a write
// carries the client. It reports false after adding the diagnostic.
func verifyWritten(revision map[string]interface{}, clientID string, diags *diag.Diagnostics) bool {
	if findClientIndex(clientsFromRevision(revision), clientID) >= 0 {
		return true
	}
	diags.AddError("Error Verifying SCIM Client",
		fmt.Sprintf("SCIM client %q was not found in the project revision the API returned.", clientID))
	return false
}

// stringOrNull converts a decoded JSON value to a string attribute. A missing
// key, a non-string, or an empty string becomes null.
func stringOrNull(v interface{}) types.String {
	if s, ok := v.(string); ok && s != "" {
		return types.StringValue(s)
	}
	return types.StringNull()
}

// splitImportID splits <project-id>/<client-id>. A value without a slash is
// a client_id alone.
func splitImportID(id string) (projectID, clientID string) {
	if i := strings.Index(id, "/"); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
}

// apiErrorDetail explains the two errors the API reports as raw database
// failures. A missing organization is an HTTP 500 foreign key violation, and
// a malformed organization ID is a UUID parse error inside an HTTP 400.
func apiErrorDetail(err error, projectID string) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "project_revision_scim_clients_organization_fkey"):
		return msg + "\n\norganization_id must be the ID of an organization in project " + projectID +
			". The API reports a missing organization as a foreign key violation."
	case strings.Contains(msg, "incorrect UUID length"):
		return msg + "\n\norganization_id must be the UUID of an organization in project " + projectID + "."
	}
	return msg
}
