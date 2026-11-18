package member

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

const (
	fakeProjectID   = "proj-1"
	fakeWorkspaceID = "ws-1"
	email           = "alice@example.com"
	// statusCancel is the status the API reports for a canceled invite.
	statusCancel = "cancelled" //nolint:misspell // the API spells the status this way
)

// fakeConsole imitates the member and invite routes of the Console API. It
// lists the newest invite first and closes the invites of a removed member,
// the way the API does.
type fakeConsole struct {
	mu       sync.Mutex
	invites  map[string][]client.MemberInvite // keyed by "projects/<id>" or "workspaces/<id>"
	members  map[string][]client.Member
	requests []string
	posted   []map[string]string
	nextID   int
	// status, when set, answers every request with this status.
	status int
}

func newFakeConsole() *fakeConsole {
	return &fakeConsole{invites: map[string][]client.MemberInvite{}, members: map[string][]client.Member{}}
}

func (f *fakeConsole) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(`{"error":{"code":401,"status":"Unauthorized","message":"The request could not be authorized"}}`))
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	key := parts[0] + "/" + parts[1]
	switch {
	case len(parts) == 3 && parts[2] == "invites" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, f.invites[key])
	case len(parts) == 3 && parts[2] == "invites" && r.Method == http.MethodPost:
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.posted = append(f.posted, body)
		f.nextID++
		role := body["role"]
		invite := client.MemberInvite{
			ID:           fmt.Sprintf("invite-%d", f.nextID),
			InviteeEmail: strings.ToLower(strings.TrimSpace(body["invitee_email"])),
			Status:       client.InviteStatusPending,
			Role:         &role,
		}
		f.invites[key] = append([]client.MemberInvite{invite}, f.invites[key]...)
		writeJSON(w, http.StatusCreated, map[string]interface{}{"created_invite": invite})
	case len(parts) == 3 && parts[0] == "invites" && parts[2] == "cancel" && r.Method == http.MethodPut:
		for k, list := range f.invites {
			for i := range list {
				if list[i].ID == parts[1] {
					f.invites[k][i].Status = statusCancel
					writeJSON(w, http.StatusOK, f.invites[k][i])
					return
				}
			}
		}
		w.WriteHeader(http.StatusNotFound)
	case len(parts) == 3 && parts[2] == "members" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, f.members[key])
	case len(parts) == 4 && parts[2] == "members" && r.Method == http.MethodDelete:
		var kept []client.Member
		for _, m := range f.members[key] {
			if m.ID != parts[3] {
				kept = append(kept, m)
			}
		}
		f.members[key] = kept
		for i, invite := range f.invites[key] {
			if invite.InviteeID != nil && *invite.InviteeID == parts[3] && invite.Status == client.InviteStatusAccepted {
				f.invites[key][i].Status = "removed"
			}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeConsole) countRequests(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func invite(id, inviteeEmail, status, role string, inviteeID *string) client.MemberInvite {
	return client.MemberInvite{ID: id, InviteeEmail: inviteeEmail, Status: status, Role: &role, InviteeID: inviteeID}
}

type harness struct {
	t       *testing.T
	console *fakeConsole
	r       *memberResource
	schema  schema.Schema
}

func newHarness(t *testing.T, newResource func() resource.Resource) *harness {
	t.Helper()
	console := newFakeConsole()
	srv := httptest.NewServer(console)
	t.Cleanup(srv.Close)

	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   srv.URL,
		ProjectID:       fakeProjectID,
		WorkspaceID:     fakeWorkspaceID,
	})
	require.NoError(t, err)

	r := newResource().(*memberResource)
	r.client = c
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())
	return &harness{t: t, console: console, r: r, schema: schemaResp.Schema}
}

func (h *harness) state(m memberModel) tfsdk.State {
	ctx := context.Background()
	s := tfsdk.State{Schema: h.schema, Raw: tftypes.NewValue(h.schema.Type().TerraformType(ctx), nil)}
	require.False(h.t, h.r.scope.set(ctx, &s, &m).HasError())
	return s
}

func (h *harness) model(s tfsdk.State) memberModel {
	var m memberModel
	require.False(h.t, h.r.scope.get(context.Background(), s, &m).HasError())
	return m
}

// planned is a plan for email with role. The scope ID is left to the provider.
func planned(role string) memberModel {
	return memberModel{
		ID:       types.StringUnknown(),
		ScopeID:  types.StringUnknown(),
		Email:    types.StringValue(email),
		Role:     types.StringValue(role),
		Status:   types.StringUnknown(),
		InviteID: types.StringUnknown(),
		MemberID: types.StringUnknown(),
	}
}

// stored is the state of the viewer invite "inv" that Terraform created.
func stored(scopeID string) memberModel {
	return memberModel{
		ID:       types.StringValue(scopeID + "/" + email),
		ScopeID:  types.StringValue(scopeID),
		Email:    types.StringValue(email),
		Role:     types.StringValue("viewer"),
		Status:   types.StringValue(client.InviteStatusPending),
		InviteID: types.StringValue("inv"),
		MemberID: types.StringNull(),
	}
}

func (h *harness) create(m memberModel) (*resource.CreateResponse, memberModel) {
	plan := h.state(m)
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: h.schema, Raw: tftypes.NewValue(h.schema.Type().TerraformType(context.Background()), nil)}}
	h.r.Create(context.Background(), resource.CreateRequest{Plan: tfsdk.Plan{Schema: h.schema, Raw: plan.Raw}}, resp)
	if resp.Diagnostics.HasError() {
		return resp, memberModel{}
	}
	return resp, h.model(resp.State)
}

func (h *harness) read(m memberModel) (*resource.ReadResponse, memberModel) {
	s := h.state(m)
	resp := &resource.ReadResponse{State: s}
	h.r.Read(context.Background(), resource.ReadRequest{State: s}, resp)
	if resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		return resp, memberModel{}
	}
	return resp, h.model(resp.State)
}

func (h *harness) delete(m memberModel) *resource.DeleteResponse {
	resp := &resource.DeleteResponse{State: h.state(m)}
	h.r.Delete(context.Background(), resource.DeleteRequest{State: h.state(m)}, resp)
	return resp
}

func TestCreate_InvitesWithProviderProjectID(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)

	resp, got := h.create(planned("viewer"))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	require.Len(t, h.console.posted, 1)
	assert.Equal(t, map[string]string{"invitee_email": email, "role": "viewer"}, h.console.posted[0])
	assert.Equal(t, 1, h.console.countRequests("POST /projects/"+fakeProjectID+"/invites"))
	assert.Equal(t, fakeProjectID+"/"+email, got.ID.ValueString())
	assert.Equal(t, fakeProjectID, got.ScopeID.ValueString())
	assert.Equal(t, "viewer", got.Role.ValueString())
	assert.Equal(t, client.InviteStatusPending, got.Status.ValueString())
	assert.Equal(t, "invite-1", got.InviteID.ValueString())
	assert.True(t, got.MemberID.IsNull(), "an unlinked invite has no member ID")
}

func TestCreate_WorkspaceMemberUsesWorkspaceRoutes(t *testing.T) {
	h := newHarness(t, NewWorkspaceMemberResource)

	resp, got := h.create(planned("owner"))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	assert.Equal(t, 1, h.console.countRequests("POST /workspaces/"+fakeWorkspaceID+"/invites"))
	assert.Equal(t, fakeWorkspaceID+"/"+email, got.ID.ValueString())
	assert.Equal(t, "owner", got.Role.ValueString())
}

func TestCreate_RefusesEmailWithOpenInvite(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	h.console.invites["projects/"+fakeProjectID] = []client.MemberInvite{
		invite("existing", email, client.InviteStatusPending, "developer", nil),
	}

	resp, _ := h.create(planned("viewer"))
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics[0].Detail(), "terraform import <resource address> "+fakeProjectID+"/"+email)
	assert.Empty(t, h.console.posted, "no second invite is sent")
}

func TestCreate_RefusesDirectMember(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	h.console.members["projects/"+fakeProjectID] = []client.Member{{ID: "user-1", Email: email, Role: "viewer", Source: "project"}}

	resp, _ := h.create(planned("viewer"))
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics[0].Detail(), email+" already has access to project "+fakeProjectID)
	assert.Empty(t, h.console.posted, "no invite is sent")
}

func TestCreate_IgnoresClosedInvites(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	h.console.invites["projects/"+fakeProjectID] = []client.MemberInvite{
		invite("old", email, statusCancel, "viewer", nil),
		invite("older", email, "expired", "viewer", nil),
	}

	resp, got := h.create(planned("viewer"))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, "invite-1", got.InviteID.ValueString())
}

func TestRead(t *testing.T) {
	memberID := "user-1"
	pending := client.InviteStatusPending
	accepted := client.InviteStatusAccepted

	tests := []struct {
		name    string
		invites []client.MemberInvite
		members []client.Member
		// gone means the resource leaves the state.
		gone       bool
		wantInvite string // "" means invite_id is null
		wantStatus string
		wantRole   string
		wantMember string // "" means member_id is null
	}{
		{
			name:       "pending invite",
			invites:    []client.MemberInvite{invite("inv", email, pending, "viewer", nil)},
			wantInvite: "inv", wantStatus: pending, wantRole: "viewer",
		},
		{
			name:       "accepted invite hidden behind a workspace role",
			invites:    []client.MemberInvite{invite("inv", email, accepted, "viewer", &memberID)},
			members:    []client.Member{{ID: memberID, Email: email, Role: "owner", Source: "workspace"}},
			wantInvite: "inv", wantStatus: accepted, wantRole: "viewer", wantMember: memberID,
		},
		{
			name:    "accepted invite of a person who is no longer a member",
			invites: []client.MemberInvite{invite("inv", email, accepted, "viewer", &memberID)},
			gone:    true,
		},
		{
			name:    "canceled invite",
			invites: []client.MemberInvite{invite("inv", email, statusCancel, "viewer", nil)},
			gone:    true,
		},
		{
			name:    "invite for another email",
			invites: []client.MemberInvite{invite("inv", "bob@example.com", pending, "viewer", nil)},
			gone:    true,
		},
		{
			name: "a new invite replaces a canceled one",
			invites: []client.MemberInvite{
				invite("resent", email, pending, "developer", nil),
				invite("inv", email, statusCancel, "viewer", nil),
			},
			wantInvite: "resent", wantStatus: pending, wantRole: "developer",
		},
		{
			name:       "an invite without a role grants developer",
			invites:    []client.MemberInvite{{ID: "inv", InviteeEmail: email, Status: pending}},
			wantInvite: "inv", wantStatus: pending, wantRole: "developer",
		},
		{
			name:       "a direct member without an invite",
			members:    []client.Member{{ID: memberID, Email: email, Role: "developer", Source: "project"}},
			wantStatus: accepted, wantRole: "developer", wantMember: memberID,
		},
		{
			name:       "a direct member wins over a pending invite",
			invites:    []client.MemberInvite{invite("inv", email, pending, "viewer", nil)},
			members:    []client.Member{{ID: memberID, Email: email, Role: "developer", Source: "project"}},
			wantStatus: accepted, wantRole: "developer", wantMember: memberID,
		},
		{
			name:    "a role through the workspace is not a project member",
			members: []client.Member{{ID: memberID, Email: email, Role: "developer", Source: "workspace"}},
			gone:    true,
		},
		{
			name:    "a project owner cannot be managed",
			members: []client.Member{{ID: memberID, Email: email, Role: "owner", Source: "project"}},
			gone:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, NewProjectMemberResource)
			key := "projects/" + fakeProjectID
			h.console.invites[key] = tc.invites
			h.console.members[key] = tc.members

			resp, got := h.read(stored(fakeProjectID))
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			if tc.gone {
				assert.True(t, resp.State.Raw.IsNull(), "the resource leaves the state")
				return
			}
			assert.Equal(t, tc.wantInvite, got.InviteID.ValueString())
			assert.Equal(t, tc.wantInvite == "", got.InviteID.IsNull(), "invite_id")
			assert.Equal(t, tc.wantStatus, got.Status.ValueString())
			assert.Equal(t, tc.wantRole, got.Role.ValueString())
			assert.Equal(t, tc.wantMember, got.MemberID.ValueString())
			assert.Equal(t, tc.wantMember == "", got.MemberID.IsNull(), "member_id")
		})
	}
}

func TestRead_WorkspaceMemberWithoutInvite(t *testing.T) {
	h := newHarness(t, NewWorkspaceMemberResource)
	h.console.members["workspaces/"+fakeWorkspaceID] = []client.Member{{ID: "user-1", Email: email, Role: "owner"}}

	resp, got := h.read(stored(fakeWorkspaceID))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, "owner", got.Role.ValueString())
	assert.Equal(t, client.InviteStatusAccepted, got.Status.ValueString())
	assert.True(t, got.InviteID.IsNull())
}

func TestRead_ImportPrefersAcceptedInvite(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	memberID := "user-1"
	key := "projects/" + fakeProjectID
	h.console.invites[key] = []client.MemberInvite{
		invite("pending", email, client.InviteStatusPending, "developer", nil),
		invite("accepted", email, client.InviteStatusAccepted, "viewer", &memberID),
	}
	h.console.members[key] = []client.Member{{ID: memberID, Email: email, Role: "viewer", Source: "project"}}

	// An import leaves only project_id and email in state.
	imported := memberModel{
		ID: types.StringNull(), ScopeID: types.StringValue(fakeProjectID), Email: types.StringValue(email),
		Role: types.StringNull(), Status: types.StringNull(), InviteID: types.StringNull(), MemberID: types.StringNull(),
	}
	resp, got := h.read(imported)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, "accepted", got.InviteID.ValueString())
	assert.Equal(t, memberID, got.MemberID.ValueString())
	assert.Equal(t, fakeProjectID+"/"+email, got.ID.ValueString())
}

func TestDelete_CancelsPendingInvite(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	key := "projects/" + fakeProjectID
	h.console.invites[key] = []client.MemberInvite{invite("inv", email, client.InviteStatusPending, "viewer", nil)}

	resp := h.delete(stored(fakeProjectID))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, 1, h.console.countRequests("PUT /invites/inv/cancel"))
	assert.Equal(t, statusCancel, h.console.invites[key][0].Status)
}

func TestDelete_RemovesMemberOfAcceptedInvite(t *testing.T) {
	h := newHarness(t, NewWorkspaceMemberResource)
	memberID := "user-1"
	key := "workspaces/" + fakeWorkspaceID
	h.console.invites[key] = []client.MemberInvite{invite("inv", email, client.InviteStatusAccepted, "viewer", &memberID)}
	h.console.members[key] = []client.Member{{ID: memberID, Email: email, Role: "viewer"}}

	resp := h.delete(stored(fakeWorkspaceID))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, 1, h.console.countRequests("DELETE /workspaces/"+fakeWorkspaceID+"/members/"+memberID))
	assert.Empty(t, h.console.members[key])
	assert.Equal(t, "removed", h.console.invites[key][0].Status)
}

func TestDelete_RemovesMemberWithoutInvite(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	key := "projects/" + fakeProjectID
	h.console.members[key] = []client.Member{{ID: "user-1", Email: email, Role: "viewer", Source: "project"}}

	resp := h.delete(stored(fakeProjectID))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, 1, h.console.countRequests("DELETE /projects/"+fakeProjectID+"/members/user-1"))
	assert.Empty(t, h.console.members[key])
}

func TestDelete_ClosedInviteIsNoOp(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	h.console.invites["projects/"+fakeProjectID] = []client.MemberInvite{invite("inv", email, "declined", "viewer", nil)}

	resp := h.delete(stored(fakeProjectID))
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Zero(t, h.console.countRequests("PUT "))
	assert.Zero(t, h.console.countRequests("DELETE "))
}

func TestRead_WorkspaceAPIKeyGetsConsoleUserHint(t *testing.T) {
	h := newHarness(t, NewProjectMemberResource)
	h.console.status = http.StatusUnauthorized

	resp, _ := h.read(stored(fakeProjectID))
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics[0].Detail(), "Run `ory auth`")
}

func TestImportState(t *testing.T) {
	tests := []struct {
		id        string
		wantScope string
		wantEmail string
		wantError bool
	}{
		{id: fakeProjectID + "/Alice@Example.com ", wantScope: fakeProjectID, wantEmail: email},
		{id: email, wantEmail: email},
		{id: fakeProjectID + "/", wantError: true},
		{id: "", wantError: true},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			h := newHarness(t, NewProjectMemberResource)
			ctx := context.Background()
			resp := &resource.ImportStateResponse{State: tfsdk.State{Schema: h.schema, Raw: tftypes.NewValue(h.schema.Type().TerraformType(ctx), nil)}}
			h.r.ImportState(ctx, resource.ImportStateRequest{ID: tc.id}, resp)
			if tc.wantError {
				assert.True(t, resp.Diagnostics.HasError())
				return
			}
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			got := h.model(resp.State)
			assert.Equal(t, tc.wantEmail, got.Email.ValueString())
			assert.Equal(t, tc.wantScope, got.ScopeID.ValueString())
		})
	}
}

func TestEmailValidator(t *testing.T) {
	assert.True(t, lowercaseEmail.MatchString("alice+tf@example.com"))
	assert.False(t, lowercaseEmail.MatchString("Alice@example.com"))
	assert.False(t, lowercaseEmail.MatchString("alice@example.com "))
	assert.False(t, lowercaseEmail.MatchString("alice"))
}
