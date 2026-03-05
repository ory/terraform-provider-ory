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
	SchemaID types.String `tfsdk:"schema_id"`
	Schema   types.String `tfsdk:"schema"`
}

const identitySchemaDataSourceMarkdownDescription = `
Fetches a single identity schema by its ID.

This data source is useful for referencing an existing identity schema without recreating it.
For example, when you destroy and recreate resources via Terraform, you can use this data source
to look up schemas that already exist in the project rather than creating duplicates.

~> **Note:** Ory may assign hash-based IDs to schemas. Use the ` + "`ory_identity_schemas`" + ` (plural) data source
to discover available schema IDs, or use the ` + "`id`" + ` output from an ` + "`ory_identity_schema`" + ` resource.

## Example Usage

### Look up by schema ID

` + "```hcl" + `
data "ory_identity_schema" "customer" {
  schema_id = "preset://username"
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
  schema_id = ory_identity_schema.customer.id
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
			"schema_id": schema.StringAttribute{
				Description: "The schema ID to look up. This is the API-assigned ID (which may be a hash) or a preset ID like 'preset://username'.",
				Required:    true,
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

	targetID := data.SchemaID.ValueString()

	// Retry to handle eventual consistency — newly created schemas may not
	// appear in ListIdentitySchemas immediately.
	var schemas []ory.IdentitySchemaContainer
	for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
		var err error
		schemas, err = d.client.ListIdentitySchemas(ctx)
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

	// Schema not found after retries — list a sample of available IDs for debugging
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
		fmt.Sprintf("No identity schema found with schema_id=%q. Available schema IDs (sample): %v\n\n"+
			"Use the ory_identity_schemas (plural) data source to discover all available schema IDs.",
			targetID, sampleIDs),
	)
}
