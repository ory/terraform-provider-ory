// Package member implements the ory_project_member and ory_workspace_member
// resources. Both give one person access through an invite: Ory emails the
// invite and the person accepts it with their own Console account.
package member

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ resource.Resource                = &memberResource{}
	_ resource.ResourceWithConfigure   = &memberResource{}
	_ resource.ResourceWithImportState = &memberResource{}
)

// lowercaseEmail matches the form the API stores. Ory trims and lowercases the
// invitee email, so a mixed-case value would never match its invite again.
var lowercaseEmail = regexp.MustCompile(`^[^\sA-Z@]+@[^\sA-Z@]+$`)

// scope holds what differs between the two resources.
type scope struct {
	api       client.MemberScope
	noun      string
	idAttr    string
	roles     []string
	defaultID func(*client.OryClient) string
}

var (
	projectScope = scope{
		api:       client.ProjectScope,
		noun:      "project",
		idAttr:    "project_id",
		roles:     []string{"developer", "viewer"},
		defaultID: (*client.OryClient).ProjectID,
	}
	workspaceScope = scope{
		api:       client.WorkspaceScope,
		noun:      "workspace",
		idAttr:    "workspace_id",
		roles:     []string{"owner", "developer", "viewer"},
		defaultID: (*client.OryClient).WorkspaceID,
	}
)

// NewProjectMemberResource returns the ory_project_member resource.
func NewProjectMemberResource() resource.Resource {
	return &memberResource{scope: projectScope}
}

// NewWorkspaceMemberResource returns the ory_workspace_member resource.
func NewWorkspaceMemberResource() resource.Resource {
	return &memberResource{scope: workspaceScope}
}

type memberResource struct {
	scope  scope
	client *client.OryClient
}

// memberModel holds the attributes of both resources. ScopeID is project_id or
// workspace_id, so the model is read and written one attribute at a time.
type memberModel struct {
	ID, ScopeID, Email, Role, Status, InviteID, MemberID types.String
}

func (s scope) attributes(m *memberModel) map[string]*types.String {
	return map[string]*types.String{
		"id":        &m.ID,
		s.idAttr:    &m.ScopeID,
		"email":     &m.Email,
		"role":      &m.Role,
		"status":    &m.Status,
		"invite_id": &m.InviteID,
		"member_id": &m.MemberID,
	}
}

type attributeGetter interface {
	GetAttribute(context.Context, path.Path, interface{}) diag.Diagnostics
}

func (s scope) get(ctx context.Context, src attributeGetter, m *memberModel) diag.Diagnostics {
	var diags diag.Diagnostics
	for name, value := range s.attributes(m) {
		diags.Append(src.GetAttribute(ctx, path.Root(name), value)...)
	}
	return diags
}

func (s scope) set(ctx context.Context, state *tfsdk.State, m *memberModel) diag.Diagnostics {
	var diags diag.Diagnostics
	for name, value := range s.attributes(m) {
		diags.Append(state.SetAttribute(ctx, path.Root(name), *value)...)
	}
	return diags
}

func (r *memberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.scope.noun + "_member"
}

func (r *memberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := r.scope
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Gives one person access to an Ory Network " + s.noun + " through an invite.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The resource ID, `<" + s.idAttr + ">/<email>`.",
				Computed:      true,
				PlanModifiers: keep,
			},
			s.idAttr: schema.StringAttribute{
				Description: "The " + s.noun + " ID. If not set, the provider's `" + s.idAttr + "` is used.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Description: "The email address of the person, in lowercase. Ory sends the invite to this address. " +
					"The person must accept it with a Console account that uses the same address.",
				Required:      true,
				PlanModifiers: replace,
				Validators: []validator.String{
					stringvalidator.RegexMatches(lowercaseEmail,
						"must be a lowercase email address, because Ory stores invitee emails in lowercase"),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role of the person. One of `" + strings.Join(s.roles, "`, `") + "`. " +
					"A change removes the person and sends a new invite, because the API cannot change the role of a member.",
				Required:      true,
				PlanModifiers: replace,
				Validators:    []validator.String{stringvalidator.OneOf(s.roles...)},
			},
			"status": schema.StringAttribute{
				Description:   "`pending` until the person accepts the invite, then `accepted`.",
				Computed:      true,
				PlanModifiers: keep,
			},
			"invite_id": schema.StringAttribute{
				Description:   "The ID of the invite. Null for a member who joined without an invite.",
				Computed:      true,
				PlanModifiers: keep,
			},
			"member_id": schema.StringAttribute{
				Description: "The Ory Network user ID of the person. Ory sets it when the person links the invite " +
					"to their Console account, at the latest when they accept the invite.",
				Computed:      true,
				PlanModifiers: keep,
			},
		},
	}
}

func (r *memberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = oryClient
}

// scopeID returns the configured project or workspace ID, or the provider's.
func (r *memberResource) scopeID(m *memberModel) string {
	if id := m.ScopeID.ValueString(); id != "" {
		return id
	}
	return r.scope.defaultID(r.client)
}

func (r *memberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m memberModel
	resp.Diagnostics.Append(r.scope.get(ctx, req.Plan, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	scopeID := r.scopeID(&m)
	email := m.Email.ValueString()

	// The API accepts a second invite for the same email and sends a second
	// email, so existing access or an existing invite is reported instead.
	existing, err := r.lookup(ctx, scopeID, email, "")
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Members", err.Error())
		return
	}
	if existing != nil {
		has := "access to"
		if existing.Status == client.InviteStatusPending {
			has = "a pending invite to"
		}
		resp.Diagnostics.AddError("Member Already Exists", fmt.Sprintf(
			"%s already has %s %s %s. Import it instead of sending a second invite:\n\n"+
				"  terraform import <resource address> %s/%s",
			email, has, r.scope.noun, scopeID, scopeID, email))
		return
	}

	invite, err := r.client.CreateMemberInvite(ctx, r.scope.api, scopeID, email, m.Role.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Invite", err.Error())
		return
	}
	readInvite(&m, scopeID, invite)
	resp.Diagnostics.Append(r.scope.set(ctx, &resp.State, &m)...)
}

func (r *memberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m memberModel
	resp.Diagnostics.Append(r.scope.get(ctx, req.State, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	scopeID := r.scopeID(&m)

	found, err := r.lookup(ctx, scopeID, m.Email.ValueString(), m.InviteID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Members", err.Error())
		return
	}
	if found == nil {
		// The invite was declined, expired, or canceled, or the person was
		// removed. The next apply invites the person again.
		resp.State.RemoveResource(ctx)
		return
	}
	readInvite(&m, scopeID, found)
	resp.Diagnostics.Append(r.scope.set(ctx, &resp.State, &m)...)
}

// Update is never called with a change, because every configurable attribute
// forces a new resource.
func (r *memberResource) Update(_ context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.State.Raw
}

func (r *memberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m memberModel
	resp.Diagnostics.Append(r.scope.get(ctx, req.State, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	scopeID := r.scopeID(&m)

	// The person can accept the invite at any time, so the current status
	// decides between canceling the invite and removing the member. Removing
	// the member also closes their open invites.
	found, err := r.lookup(ctx, scopeID, m.Email.ValueString(), m.InviteID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Members", err.Error())
		return
	}
	switch {
	case found == nil:
		return
	case found.Status == client.InviteStatusPending:
		err = r.client.CancelMemberInvite(ctx, r.scope.api, scopeID, found.ID)
	default:
		// lookup reports access only for a person in the members list, so
		// InviteeID is set.
		err = r.client.RemoveMember(ctx, r.scope.api, scopeID, *found.InviteeID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Removing Member", err.Error())
	}
}

func (r *memberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	scopeID, email, ok := strings.Cut(req.ID, "/")
	if !ok {
		scopeID, email = "", req.ID
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf(
			"Expected <%s>/<email>, or <email> with %s set on the provider, got %q.",
			r.scope.idAttr, r.scope.idAttr, req.ID))
		return
	}
	if scopeID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.scope.idAttr), scopeID)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), email)...)
}

// lookup returns how email has access to the project or workspace, or nil. It
// prefers access over an offer: an accepted invite whose invitee is still a
// member, then a membership without an invite, then a pending invite. The
// invite in state wins over other invites with the same status, and otherwise
// the newest invite wins, because the API lists the newest invite first.
//
// A membership without an invite is returned as an accepted invite without an
// ID. Some members have no invite, such as the creator of a workspace.
//
// An accepted invite wins over the members list, because the project members
// list reports one effective role per person and hides a direct project role
// behind an equal or stronger workspace role.
func (r *memberResource) lookup(ctx context.Context, scopeID, email, inviteID string) (*client.MemberInvite, error) {
	if scopeID == "" {
		return nil, fmt.Errorf("%s is not set: set it on the resource or on the provider", r.scope.idAttr)
	}
	invites, err := r.client.ListMemberInvites(ctx, r.scope.api, scopeID)
	if err != nil {
		return nil, err
	}
	members, err := r.client.ListMembers(ctx, r.scope.api, scopeID)
	if err != nil {
		return nil, err
	}
	isMember := func(id *string) bool {
		return id != nil && slices.ContainsFunc(members, func(m client.Member) bool { return m.ID == *id })
	}

	var accepted, pending *client.MemberInvite
	for i := range invites {
		invite := &invites[i]
		if !strings.EqualFold(invite.InviteeEmail, email) {
			continue
		}
		switch {
		case invite.Status == client.InviteStatusAccepted && isMember(invite.InviteeID):
			if accepted == nil || invite.ID == inviteID {
				accepted = invite
			}
		case invite.Status == client.InviteStatusPending:
			if pending == nil || invite.ID == inviteID {
				pending = invite
			}
		}
	}
	if accepted != nil {
		return accepted, nil
	}

	// A project member list also reports the workspace members. Only a direct
	// role that the resource can grant counts.
	for _, m := range members {
		if strings.EqualFold(m.Email, email) && m.Source != "workspace" && slices.Contains(r.scope.roles, m.Role) {
			return &client.MemberInvite{
				InviteeID:    &m.ID,
				InviteeEmail: m.Email,
				Status:       client.InviteStatusAccepted,
				Role:         &m.Role,
			}, nil
		}
	}
	return pending, nil
}

// readInvite copies the access that lookup found into the model.
func readInvite(m *memberModel, scopeID string, invite *client.MemberInvite) {
	// Ory grants developer for an invite without a role.
	role := "developer"
	if invite.Role != nil && *invite.Role != "" {
		role = *invite.Role
	}
	m.ID = types.StringValue(scopeID + "/" + m.Email.ValueString())
	m.ScopeID = types.StringValue(scopeID)
	m.Role = types.StringValue(role)
	m.Status = types.StringValue(invite.Status)
	m.InviteID = types.StringNull()
	if invite.ID != "" {
		m.InviteID = types.StringValue(invite.ID)
	}
	m.MemberID = types.StringPointerValue(invite.InviteeID)
}
