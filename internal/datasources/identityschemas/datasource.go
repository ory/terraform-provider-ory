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

	if !canUseKratosAPI && !canUseConsoleAPI {
		resp.Diagnostics.AddError("Missing Project ID",
			"project_id is required when project_slug and project_api_key are not configured. "+
				"Set project_id on the data source or configure the provider with project_slug and project_api_key.")
		return
	}

	var schemas []ory.IdentitySchemaContainer
	var err error
	for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
		// Prefer Kratos API (canonical IDs + full content) when available.
		if canUseKratosAPI {
			schemas, err = d.client.ListIdentitySchemas(ctx)
		} else {
			schemas, err = d.client.ListIdentitySchemasViaProject(ctx, projectID)
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
