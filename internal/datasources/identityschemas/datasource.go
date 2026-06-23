package identityschemas

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ datasource.DataSource              = &IdentitySchemasDataSource{}
	_ datasource.DataSourceWithConfigure = &IdentitySchemasDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &IdentitySchemasDataSource{}
}

type IdentitySchemasDataSource struct {
	client *client.OryClient
}

type IdentitySchemasDataSourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Schemas   types.List   `tfsdk:"schemas"`
}

var schemaObjectAttrTypes = map[string]attr.Type{
	"id":     types.StringType,
	"schema": types.StringType,
}

func (d *IdentitySchemasDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_schemas"
}

func (d *IdentitySchemasDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the list of identity schemas for the project.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Description: "The ID of the project to list schemas from. If not set, uses the provider's project_id. " +
					"The Kratos API is preferred when project_slug and project_api_key are configured " +
					"(returns canonical hash IDs with full schema content). When only a workspace key is " +
					"available, schemas are read from the project config via the console API.",
				Optional: true,
				Computed: true,
			},
			"schemas": schema.ListAttribute{
				Description: "List of identity schemas. Each schema has an `id` and a `schema` (JSON string of the schema content).",
				Computed:    true,
				ElementType: types.ObjectType{
					AttrTypes: schemaObjectAttrTypes,
				},
			},
		},
	}
}

func (d *IdentitySchemasDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IdentitySchemasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IdentitySchemasDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve project_id: use config value if known, fall back to provider.
	projectID := d.resolveProjectID(data.ProjectID)

	// Determine which APIs are available for this lookup.
	// Identity schemas are workspace-scoped — see identityschema (singular)
	// data source for detailed rationale on API selection.
	canUseKratosAPI := d.client.HasProjectClient()
	canUseConsoleAPI := d.client.HasConsoleClient() && projectID != ""
	// The workspace endpoint (GET /identity-schemas) needs only a workspace API
	// key. The Kratos API already returns every workspace-scoped schema, so we
	// only fall back to / merge the workspace endpoint when Kratos is not
	// available (the bootstrap case in issue #138).
	canUseWorkspaceAPI := d.client.HasConsoleClient()

	if !canUseKratosAPI && !canUseConsoleAPI && !canUseWorkspaceAPI {
		resp.Diagnostics.AddError("Missing Credentials",
			"Listing identity schemas requires either project credentials "+
				"(project_slug and project_api_key) or a workspace_api_key. "+
				"Configure one on the provider, or set project_id on the data source.")
		return
	}

	var schemas []ory.IdentitySchemaContainer
	var err error
	// Whether the results came from the project config, which only lists schemas
	// explicitly added to the project. When true we merge in workspace-scoped
	// schemas below. Tracked per-run (not from static capability) so a Kratos
	// failure that falls back to the project config still triggers the merge.
	usedProjectConfigBase := false
	for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
		usedProjectConfigBase = false
		// Prefer Kratos API (canonical IDs + full content) when available, then
		// fall back to the project config, then the workspace endpoint.
		switch {
		case canUseKratosAPI:
			schemas, err = d.client.ListIdentitySchemas(ctx)
			if err != nil && canUseConsoleAPI {
				schemas, err = d.client.ListIdentitySchemasViaProject(ctx, projectID)
				usedProjectConfigBase = err == nil
			}
			if err != nil && canUseWorkspaceAPI {
				schemas, err = d.client.ListWorkspaceIdentitySchemas(ctx)
			}
		case canUseConsoleAPI:
			schemas, err = d.client.ListIdentitySchemasViaProject(ctx, projectID)
			usedProjectConfigBase = err == nil
		default:
			// Workspace key only (bootstrap): no project to read config from.
			schemas, err = d.client.ListWorkspaceIdentitySchemas(ctx)
		}
		if err == nil {
			break
		}
		if attempt < helpers.ReadRetryMaxAttempts-1 {
			select {
			case <-ctx.Done():
				resp.Diagnostics.AddError("Error Listing Identity Schemas", ctx.Err().Error())
				return
			case <-time.After(helpers.EventualConsistencyDelay):
			}
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Listing Identity Schemas", err.Error())
		return
	}

	// When the base came from the project config, the list only contains schemas
	// explicitly added to the project — a new project sees just preset://username.
	// Merge in the workspace-scoped schemas so they are discoverable during
	// bootstrap. Kratos already returns the full set, and the workspace-only path
	// already used this endpoint, so we only merge when the project config was
	// the actual base.
	if usedProjectConfigBase && canUseWorkspaceAPI {
		wsSchemas, wsErr := d.client.ListWorkspaceIdentitySchemas(ctx)
		if wsErr != nil {
			// Only hard-fail if we have nothing else; otherwise keep the
			// project-config results we already gathered.
			if len(schemas) == 0 {
				resp.Diagnostics.AddError("Error Listing Identity Schemas", wsErr.Error())
				return
			}
		} else {
			indexByID := make(map[string]int, len(schemas))
			for i := range schemas {
				indexByID[schemas[i].GetId()] = i
			}
			for i := range wsSchemas {
				if idx, ok := indexByID[wsSchemas[i].GetId()]; ok {
					// On an ID collision, prefer the workspace entry when the
					// existing (project-config) body is empty — the workspace
					// endpoint fetches the full schema content, whereas the
					// project config may return an empty body.
					if len(schemas[idx].GetSchema()) == 0 && len(wsSchemas[i].GetSchema()) > 0 {
						schemas[idx] = wsSchemas[i]
					}
					continue
				}
				indexByID[wsSchemas[i].GetId()] = len(schemas)
				schemas = append(schemas, wsSchemas[i])
			}
		}
	}

	schemaObjects := make([]attr.Value, 0, len(schemas))
	for _, s := range schemas {
		schemaJSON, err := json.Marshal(s.GetSchema())
		if err != nil {
			resp.Diagnostics.AddError("Error Marshaling Schema", fmt.Sprintf("Could not marshal schema %s: %s", s.GetId(), err.Error()))
			return
		}
		obj, diags := types.ObjectValue(schemaObjectAttrTypes, map[string]attr.Value{
			"id":     types.StringValue(s.GetId()),
			"schema": types.StringValue(string(schemaJSON)),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		schemaObjects = append(schemaObjects, obj)
	}

	schemaList, diags := types.ListValue(types.ObjectType{AttrTypes: schemaObjectAttrTypes}, schemaObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if projectID != "" {
		data.ProjectID = types.StringValue(projectID)
	}
	data.Schemas = schemaList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *IdentitySchemasDataSource) resolveProjectID(tfProjectID types.String) string {
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() {
		return tfProjectID.ValueString()
	}
	return d.client.ProjectID()
}
