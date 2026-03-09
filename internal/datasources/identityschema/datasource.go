package identityschema

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ datasource.DataSource              = &IdentitySchemaDataSource{}
	_ datasource.DataSourceWithConfigure = &IdentitySchemaDataSource{}
)

func NewDataSource() datasource.DataSource {
	return &IdentitySchemaDataSource{}
}

type IdentitySchemaDataSource struct {
	client *client.OryClient
}

type IdentitySchemaDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Schema    types.String `tfsdk:"schema"`
}

const identitySchemaDataSourceMarkdownDescription = `
Fetches a single identity schema by its ID.

This data source is useful for referencing an existing identity schema without recreating it.
For example, when you destroy and recreate resources via Terraform, you can use this data source
to look up schemas that already exist in the project rather than creating duplicates.

~> **Note:** Ory may assign hash-based IDs to schemas. Use the ` + "`ory_identity_schemas`" + ` (plural) data source
to discover available schema IDs, or use the ` + "`id`" + ` output from an ` + "`ory_identity_schema`" + ` resource.

## Example Usage

### Look up by ID

` + "```hcl" + `
data "ory_identity_schema" "customer" {
  id         = "preset://username"
  project_id = "your-project-uuid"
}

output "schema_content" {
  value = data.ory_identity_schema.customer.schema
}
` + "```" + `

### Reference from a resource

` + "```hcl" + `
resource "ory_identity_schema" "customer" {
  schema_id = "customer"
  schema    = jsonencode({ ... })
}

data "ory_identity_schema" "customer" {
  id = ory_identity_schema.customer.id
}
` + "```" + `
`

func (d *IdentitySchemaDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_schema"
}

func (d *IdentitySchemaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Fetches a single identity schema by its ID.",
		MarkdownDescription: identitySchemaDataSourceMarkdownDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of the schema to look up. This is the API-assigned ID (which may be a hash) or a preset ID like 'preset://username'.",
				Required:    true,
			},
			"project_id": schema.StringAttribute{
				Description: "The ID of the project to look up schemas from. If not set, uses the provider's project_id. " +
					"When set, schemas are read from the project config via the console API (workspace key), " +
					"which does not require project_slug or project_api_key.",
				Optional: true,
				Computed: true,
			},
			"schema": schema.StringAttribute{
				Description: "The JSON Schema definition for the identity traits.",
				Computed:    true,
			},
		},
	}
}

func (d *IdentitySchemaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IdentitySchemaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IdentitySchemaDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetID := data.ID.ValueString()

	// Determine how to list schemas: use the console API (GetProject) when
	// project_id is explicitly set or when the project client is not
	// configured. Fall back to the project API otherwise.
	if data.ProjectID.IsUnknown() {
		resp.Diagnostics.AddError("Missing Project ID",
			"project_id is not yet known. Use depends_on to ensure the project is created first.")
		return
	}
	projectID := data.ProjectID.ValueString()
	if projectID == "" {
		projectID = d.client.ProjectID()
	}
	useConsoleAPI := !data.ProjectID.IsNull() || !d.client.HasProjectClient()

	var schemas []ory.IdentitySchemaContainer
	for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
		var err error
		if useConsoleAPI && projectID != "" {
			schemas, err = d.client.ListIdentitySchemasViaProject(ctx, projectID)
		} else {
			schemas, err = d.client.ListIdentitySchemas(ctx)
		}
		if err != nil {
			resp.Diagnostics.AddError("Error Listing Identity Schemas", err.Error())
			return
		}

		for _, s := range schemas {
			if s.GetId() == targetID {
				schemaJSON, err := json.Marshal(s.GetSchema())
				if err != nil {
					resp.Diagnostics.AddError("Error Marshaling Schema",
						fmt.Sprintf("Could not marshal schema %s: %s", s.GetId(), err.Error()))
					return
				}
				data.Schema = types.StringValue(string(schemaJSON))
				data.ProjectID = types.StringValue(projectID)
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}

		if attempt < helpers.ReadRetryMaxAttempts-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}

	var sampleIDs []string
	for i, s := range schemas {
		if i >= 5 {
			sampleIDs = append(sampleIDs, fmt.Sprintf("... and %d more", len(schemas)-5))
			break
		}
		sampleIDs = append(sampleIDs, s.GetId())
	}
	resp.Diagnostics.AddError(
		"Identity Schema Not Found",
		fmt.Sprintf("No identity schema found with id=%q. Available schema IDs (sample): %v\n\n"+
			"Use the ory_identity_schemas (plural) data source to discover all available schema IDs.",
			targetID, sampleIDs),
	)
}
