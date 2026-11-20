// Package members implements the ory_project_members and ory_workspace_members
// data sources.
package members

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
)

var (
	_ datasource.DataSource              = &membersDataSource{}
	_ datasource.DataSourceWithConfigure = &membersDataSource{}
)

// scope holds what differs between the two data sources.
type scope struct {
	api    client.MemberScope
	noun   string
	idAttr string
	// withSource adds the source of each role. Only project members have one.
	withSource bool
	defaultID  func(*client.OryClient) string
}

// NewProjectMembersDataSource returns the ory_project_members data source.
func NewProjectMembersDataSource() datasource.DataSource {
	return &membersDataSource{scope: scope{
		api:        client.ProjectScope,
		noun:       "project",
		idAttr:     "project_id",
		withSource: true,
		defaultID:  (*client.OryClient).ProjectID,
	}}
}

// NewWorkspaceMembersDataSource returns the ory_workspace_members data source.
func NewWorkspaceMembersDataSource() datasource.DataSource {
	return &membersDataSource{scope: scope{
		api:       client.WorkspaceScope,
		noun:      "workspace",
		idAttr:    "workspace_id",
		defaultID: (*client.OryClient).WorkspaceID,
	}}
}

type membersDataSource struct {
	scope  scope
	client *client.OryClient
}

func (s scope) memberType() types.ObjectType {
	attrs := map[string]attr.Type{
		"id":    types.StringType,
		"email": types.StringType,
		"name":  types.StringType,
		"role":  types.StringType,
	}
	if s.withSource {
		attrs["source"] = types.StringType
	}
	return types.ObjectType{AttrTypes: attrs}
}

func (d *membersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.scope.noun + "_members"
}

func (d *membersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	s := d.scope
	description := "The people with access to the " + s.noun + ". Each member has an `id`, the Ory Network user ID, " +
		"an `email`, a `name`, and a `role`: `owner`, `developer`, or `viewer`."
	if s.withSource {
		description += " A project member also has a `source`: `project` or `workspace`, the scope that gives the role. " +
			"Ory reports one role per person, the strongest one, and takes the workspace role on a tie."
	}
	resp.Schema = schema.Schema{
		Description: "Lists the members of an Ory Network " + s.noun + ".",
		Attributes: map[string]schema.Attribute{
			s.idAttr: schema.StringAttribute{
				Description: "The " + s.noun + " ID. If not set, the provider's `" + s.idAttr + "` is used.",
				Optional:    true,
				Computed:    true,
			},
			"members": schema.ListAttribute{
				Description: description,
				Computed:    true,
				ElementType: s.memberType(),
			},
		},
	}
}

func (d *membersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T", req.ProviderData))
		return
	}
	d.client = oryClient
}

func (d *membersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	s := d.scope
	var configured types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(s.idAttr), &configured)...)
	if resp.Diagnostics.HasError() {
		return
	}
	scopeID := configured.ValueString()
	if scopeID == "" {
		scopeID = s.defaultID(d.client)
	}
	if scopeID == "" {
		resp.Diagnostics.AddError("Missing "+s.idAttr, "Set "+s.idAttr+" on the data source or on the provider.")
		return
	}

	members, err := d.client.ListMembers(ctx, s.api, scopeID)
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Members", err.Error())
		return
	}

	memberType := s.memberType()
	values := make([]attr.Value, 0, len(members))
	for _, m := range members {
		attrs := map[string]attr.Value{
			"id":    types.StringValue(m.ID),
			"email": types.StringValue(m.Email),
			"name":  types.StringValue(m.Name),
			"role":  types.StringValue(m.Role),
		}
		if s.withSource {
			attrs["source"] = types.StringValue(m.Source)
		}
		value, diags := types.ObjectValue(memberType.AttrTypes, attrs)
		resp.Diagnostics.Append(diags...)
		values = append(values, value)
	}
	list, diags := types.ListValue(memberType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(s.idAttr), scopeID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("members"), list)...)
}
