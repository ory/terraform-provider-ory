package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MemberScope names the Console API collection that holds members and invites.
type MemberScope string

const (
	// ProjectScope holds the members and invites of a project.
	ProjectScope MemberScope = "projects"
	// WorkspaceScope holds the members and invites of a workspace.
	WorkspaceScope MemberScope = "workspaces"
)

// Invite statuses that still give or offer access. A closed invite has
// another status.
const (
	InviteStatusPending  = "pending"
	InviteStatusAccepted = "accepted"
)

// consoleUserHint explains the credential that the member and invite routes
// accept.
const consoleUserHint = "The Ory Console API accepts member and invite requests only from a signed-in " +
	"Console user and answers a workspace API key with HTTP 401. Run `ory auth` and set " +
	"workspace_api_key (ORY_WORKSPACE_API_KEY) to the access token it stores. See " +
	"\"Managing a project without a workspace API key\" in the provider documentation"

// MemberInvite is an invite to a project or a workspace.
type MemberInvite struct {
	ID string `json:"id"`
	// InviteeID is set once the invitee links the invite to a Console account.
	InviteeID    *string `json:"invitee_id"`
	InviteeEmail string  `json:"invitee_email"`
	Status       string  `json:"status"`
	// Role is null on invites created before invites carried a role.
	Role *string `json:"role"`
}

// Member is a person with access to a project or a workspace.
type Member struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	// Source is "project" or "workspace" for a project member: the scope that
	// gives the reported role. Workspace members have no source.
	Source string `json:"source"`
}

// ListMemberInvites returns every invite of a project or a workspace, whatever
// its status.
func (c *OryClient) ListMemberInvites(ctx context.Context, scope MemberScope, scopeID string) ([]MemberInvite, error) {
	var invites []MemberInvite
	err := c.memberRequest(ctx, scope, scopeID, "listing invites", http.MethodGet,
		"/"+string(scope)+"/"+url.PathEscape(scopeID)+"/invites", nil, &invites)
	return invites, err
}

// CreateMemberInvite invites email to a project or a workspace with role. Ory
// sends the invite by email.
func (c *OryClient) CreateMemberInvite(ctx context.Context, scope MemberScope, scopeID, email, role string) (*MemberInvite, error) {
	var created struct {
		Invite MemberInvite `json:"created_invite"`
	}
	err := c.memberRequest(ctx, scope, scopeID, "creating invite", http.MethodPost,
		"/"+string(scope)+"/"+url.PathEscape(scopeID)+"/invites",
		map[string]string{"invitee_email": email, "role": role}, &created)
	if err != nil {
		return nil, err
	}
	return &created.Invite, nil
}

// CancelMemberInvite cancels a pending invite.
func (c *OryClient) CancelMemberInvite(ctx context.Context, scope MemberScope, scopeID, inviteID string) error {
	return c.memberRequest(ctx, scope, scopeID, "canceling invite", http.MethodPut,
		"/invites/"+url.PathEscape(inviteID)+"/cancel", nil, nil)
}

// ListMembers returns the people with access to a project or a workspace. For a
// project, the list includes the workspace members, and reports one role per
// person: the strongest role, taken from the workspace on a tie.
func (c *OryClient) ListMembers(ctx context.Context, scope MemberScope, scopeID string) ([]Member, error) {
	var members []Member
	err := c.memberRequest(ctx, scope, scopeID, "listing members", http.MethodGet,
		"/"+string(scope)+"/"+url.PathEscape(scopeID)+"/members", nil, &members)
	return members, err
}

// RemoveMember removes a person from a project or a workspace and closes their
// invites to it. Removing a person who is not a member succeeds.
func (c *OryClient) RemoveMember(ctx context.Context, scope MemberScope, scopeID, memberID string) error {
	return c.memberRequest(ctx, scope, scopeID, "removing member", http.MethodDelete,
		"/"+string(scope)+"/"+url.PathEscape(scopeID)+"/members/"+url.PathEscape(memberID), nil, nil)
}

// memberRequest sends one member or invite request and decodes a successful
// JSON response into out, when out is not nil. None of these routes is in the
// generated client-go SDK.
func (c *OryClient) memberRequest(ctx context.Context, scope MemberScope, scopeID, operation, method, route string, in, out any) error {
	if c.consoleClient == nil {
		return fmt.Errorf("%s: %w. %s", operation, ErrConsoleClientNotConfigured, consoleUserHint)
	}
	if scope == ProjectScope {
		if err := c.checkProjectAllowed(scopeID); err != nil {
			return err
		}
	}

	var payload []byte
	if in != nil {
		var err error
		if payload, err = json.Marshal(in); err != nil {
			return fmt.Errorf("%s: encoding request: %w", operation, err)
		}
	}
	resp, err := c.doConsoleRawRequest(ctx, operation, method, strings.TrimRight(c.config.ConsoleAPIURL, "/")+route, payload)
	if err != nil {
		return wrapAPIError(err, operation)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: reading response: %w", operation, err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%s: unauthorized (401). %s.\nResponse body: %s",
			operation, consoleUserHint, truncateErrorBody(strings.TrimSpace(string(body))))
	case resp.StatusCode >= http.StatusMultipleChoices:
		return wrapAPIError(fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body))), operation)
	case out == nil:
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s: parsing response: %w", operation, err)
	}
	return nil
}
