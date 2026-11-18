package members

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/testutil"
)

const membersJSON = `[
  {"id":"user-1","email":"alice@example.com","name":"Alice","role":"owner","source":"workspace"},
  {"id":"user-2","email":"bob@example.com","name":"Bob","role":"viewer","source":"project"}
]`

// read runs the data source against a console that answers every member list
// with membersJSON, and returns the requested path and the resulting state.
func read(t *testing.T, newDataSource func() datasource.DataSource) (string, tfsdk.State) {
	t.Helper()
	var requested string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = r.URL.Path
		_, _ = w.Write([]byte(membersJSON))
	}))
	t.Cleanup(srv.Close)
	c, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   srv.URL,
		ProjectID:       "proj-1",
		WorkspaceID:     "ws-1",
	})
	require.NoError(t, err)

	ctx := context.Background()
	d := newDataSource().(*membersDataSource)
	d.client = c
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}}
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objectType, nil)}
	d.Read(ctx, datasource.ReadRequest{Config: config}, resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	return requested, resp.State
}

func TestRead_ProjectMembersIncludeSource(t *testing.T) {
	requested, state := read(t, NewProjectMembersDataSource)
	assert.Equal(t, "/projects/proj-1/members", requested)

	var projectID types.String
	require.False(t, state.GetAttribute(context.Background(), path.Root("project_id"), &projectID).HasError())
	assert.Equal(t, "proj-1", projectID.ValueString(), "the provider's project_id is the default")

	var members []struct {
		ID     string `tfsdk:"id"`
		Email  string `tfsdk:"email"`
		Name   string `tfsdk:"name"`
		Role   string `tfsdk:"role"`
		Source string `tfsdk:"source"`
	}
	require.False(t, state.GetAttribute(context.Background(), path.Root("members"), &members).HasError())
	require.Len(t, members, 2)
	assert.Equal(t, "bob@example.com", members[1].Email)
	assert.Equal(t, "viewer", members[1].Role)
	assert.Equal(t, "project", members[1].Source)
}

func TestRead_WorkspaceMembersHaveNoSource(t *testing.T) {
	requested, state := read(t, NewWorkspaceMembersDataSource)
	assert.Equal(t, "/workspaces/ws-1/members", requested)

	var members []struct {
		ID    string `tfsdk:"id"`
		Email string `tfsdk:"email"`
		Name  string `tfsdk:"name"`
		Role  string `tfsdk:"role"`
	}
	require.False(t, state.GetAttribute(context.Background(), path.Root("members"), &members).HasError())
	require.Len(t, members, 2)
	assert.Equal(t, "alice@example.com", members[0].Email)
	assert.Equal(t, "owner", members[0].Role)
}
