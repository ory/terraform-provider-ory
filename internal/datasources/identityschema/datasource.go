package identityschema

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
				Description: "The ID of the project. If not set, uses the provider's project_id. " +
					"The Kratos API is preferred when project_slug and project_api_key are configured " +
					"(returns canonical hash IDs with full schema content). When only a workspace key is " +
					"available, schemas are read from the project config via the console API.",
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

	// Resolve project_id: use config value if known, fall back to provider.
	projectID := d.resolveProjectID(data.ProjectID)

	// Determine which APIs are available for this lookup.
	//
	// Identity schemas are workspace-scoped: any project in the workspace can
	// access all schemas. The Kratos API (ListIdentitySchemas) returns canonical
	// hash-based IDs with full schema content and works regardless of which
	// project_id is specified, so we always prefer it when available.
	//
	// The console API (ListIdentitySchemasViaProject) reads from the project
	// config, which only contains schemas explicitly added to that project and
	// may return empty schema bodies when the API has transformed schema URLs
	// from base64:// to https://.
	canUseKratosAPI := d.client.HasProjectClient()
	canUseConsoleAPI := d.client.HasConsoleClient() && projectID != ""
	// The workspace endpoint (GET /identity-schemas) needs only a workspace API
	// key — no project_id and no project credentials. This is the bootstrap
	// case: a brand-new project has only preset://username in its config, but
	// the workspace-scoped custom schema the caller is looking up is visible
	// here.
	canUseWorkspaceAPI := d.client.HasConsoleClient()

	if !canUseKratosAPI && !canUseConsoleAPI && !canUseWorkspaceAPI {
		resp.Diagnostics.AddError("Missing Credentials",
			"Looking up an identity schema requires either project credentials "+
				"(project_slug and project_api_key) or a workspace_api_key. "+
				"Configure one on the provider, or set project_id on the data source.")
		return
	}

	var allSchemas []ory.IdentitySchemaContainer
	var found *ory.IdentitySchemaContainer

	for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
		// Strategy 1: Try Kratos API (preferred — canonical hash IDs + full content).
		if canUseKratosAPI && found == nil {
			schemas, err := d.client.ListIdentitySchemas(ctx)
			if err != nil {
				if !canUseConsoleAPI && !canUseWorkspaceAPI {
					resp.Diagnostics.AddError("Error Listing Identity Schemas", err.Error())
					return
				}
				// Kratos API failed but the console or workspace strategy is
				// available — continue to the fallbacks below.
			} else {
				allSchemas = schemas
				for i := range schemas {
					if schemas[i].GetId() == targetID {
						found = &schemas[i]
						break
					}
				}
			}
		}

		// Strategy 2: Try console API (works for any project, fallback for Kratos failures).
		if canUseConsoleAPI && found == nil {
			schemas, err := d.client.ListIdentitySchemasViaProject(ctx, projectID)
			if err != nil {
				if len(allSchemas) == 0 && !canUseWorkspaceAPI {
					resp.Diagnostics.AddError("Error Listing Identity Schemas", err.Error())
					return
				}
				// Console API failed but we already have Kratos results or can
				// still try the workspace endpoint — the schema just isn't here.
			} else {
				if len(allSchemas) == 0 {
					allSchemas = schemas
				}
				for i := range schemas {
					if schemas[i].GetId() == targetID {
						found = &schemas[i]
						break
					}
				}
			}
		}

		// Strategy 3: Try the workspace endpoint (bootstrap — workspace key
		// only, no project_id or project credentials required). This is the
		// case in issue #138: a new project sees only preset://username via the
		// project config, but the workspace-scoped schema is visible here.
		if canUseWorkspaceAPI && found == nil {
			schemas, err := d.client.ListWorkspaceIdentitySchemas(ctx)
			if err != nil {
				if len(allSchemas) == 0 {
					resp.Diagnostics.AddError("Error Listing Identity Schemas", err.Error())
					return
				}
				// Workspace endpoint failed but earlier strategies returned
				// results — the schema just isn't there.
			} else {
				if len(allSchemas) == 0 {
					allSchemas = schemas
				}
				for i := range schemas {
					if schemas[i].GetId() == targetID {
						found = &schemas[i]
						break
					}
				}
			}
		}

		if found != nil {
			break
		}

		if attempt < helpers.ReadRetryMaxAttempts-1 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}

	if found != nil {
		// If the schema was found via the console API, the content may be empty
		// (the API transforms base64:// URLs to https:// URLs, making the content
		// undecodable from the project config alone). Try the Kratos API to resolve
		// the full content when possible.
		schemaJSON, err := json.Marshal(found.GetSchema())
		if err != nil {
			resp.Diagnostics.AddError("Error Marshaling Schema",
				fmt.Sprintf("Could not marshal schema %s: %s", found.GetId(), err.Error()))
			return
		}
		if isEmptySchemaBody(schemaJSON) && canUseKratosAPI {
			kratosSchemas, err := d.client.ListIdentitySchemas(ctx)
			if err == nil {
				for i := range kratosSchemas {
					if kratosSchemas[i].GetId() == targetID {
						found = &kratosSchemas[i]
						break
					}
				}
			}
		}

		schemaJSON, err = json.Marshal(found.GetSchema())
		if err != nil {
			resp.Diagnostics.AddError("Error Marshaling Schema",
				fmt.Sprintf("Could not marshal schema %s: %s", found.GetId(), err.Error()))
			return
		}
		data.Schema = types.StringValue(string(schemaJSON))
		if projectID != "" {
			data.ProjectID = types.StringValue(projectID)
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	var sampleIDs []string
	for i, s := range allSchemas {
		if i >= 5 {
			sampleIDs = append(sampleIDs, fmt.Sprintf("... and %d more", len(allSchemas)-5))
			break
		}
		sampleIDs = append(sampleIDs, s.GetId())
	}
	var scopeMsg string
	if projectID != "" {
		scopeMsg = fmt.Sprintf(" in project %q", projectID)
	}
	verifyHint := "Verify that the schema exists in the workspace."
	if projectID != "" {
		verifyHint = fmt.Sprintf("Schemas are workspace-scoped. Verify that the schema exists in the workspace "+
			"associated with project %q, or check the project_id value.", projectID)
	}
	resp.Diagnostics.AddError(
		"Identity Schema Not Found",
		fmt.Sprintf("No identity schema found with id=%q%s. Available schema IDs (sample): %v\n\n"+
			"Use the ory_identity_schemas (plural) data source to discover all available schema IDs.\n"+
			"%s",
			targetID, scopeMsg, sampleIDs, verifyHint),
	)
}

// isEmptySchemaBody returns true if the JSON represents an empty or null schema body.
func isEmptySchemaBody(jsonBytes []byte) bool {
	s := strings.TrimSpace(string(jsonBytes))
	return s == "{}" || s == "null" || s == ""
}

func (d *IdentitySchemaDataSource) resolveProjectID(tfProjectID types.String) string {
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() {
		return tfProjectID.ValueString()
	}
	return d.client.ProjectID()
}
