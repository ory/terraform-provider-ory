package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-orynetwork/internal/client"
	projectds "github.com/ory/terraform-provider-orynetwork/internal/datasources/project"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/action"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/emailtemplate"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/identity"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/identityschema"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/jwk"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/oauth2client"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/organization"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/project"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/projectapikey"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/projectconfig"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/relationship"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/socialprovider"
	"github.com/ory/terraform-provider-orynetwork/internal/resources/workspace"
)

// Re-export client defaults for use in tests
const (
	DefaultConsoleAPIURL = client.DefaultConsoleAPIURL
	DefaultProjectAPIURL = client.DefaultProjectAPIURL
)

// Ensure OryProvider satisfies various provider interfaces.
var _ provider.Provider = &OryProvider{}

// OryProvider defines the provider implementation.
type OryProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// tests.
	version string
}

// OryProviderModel describes the provider data model.
type OryProviderModel struct {
	// API Keys
	WorkspaceAPIKey types.String `tfsdk:"workspace_api_key"`
	ProjectAPIKey   types.String `tfsdk:"project_api_key"`

	// Project/Workspace identifiers
	ProjectID   types.String `tfsdk:"project_id"`
	ProjectSlug types.String `tfsdk:"project_slug"`
	WorkspaceID types.String `tfsdk:"workspace_id"`

	// Optional: Override API URLs (for testing)
	ConsoleAPIURL types.String `tfsdk:"console_api_url"`
	ProjectAPIURL types.String `tfsdk:"project_api_url"`
}

// New returns a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OryProvider{
			version: version,
		}
	}
}

func (p *OryProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ory"
	resp.Version = p.version
}

func (p *OryProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provider for managing Ory Network resources.",
		MarkdownDescription: `
The Ory provider enables Terraform to manage [Ory Network](https://www.ory.sh/) resources.

## Authentication

Ory Network uses two types of API keys:

| API Key Type | Prefix | Used For |
|--------------|--------|----------|
| **Workspace API Key** | ` + "`ory_wak_...`" + ` | Projects, organizations, workspace management, project config, actions |
| **Project API Key** | ` + "`ory_pat_...`" + ` | Identities, OAuth2 clients, relationships |

### Configuration Options

You can configure the provider using **either** approach:

#### Option 1: Environment Variables (Recommended for CI/CD)

` + "```bash" + `
export ORY_WORKSPACE_API_KEY="ory_wak_..."
export ORY_WORKSPACE_ID="..."           # Required for creating new projects
export ORY_PROJECT_API_KEY="ory_pat_..."
export ORY_PROJECT_ID="..."
export ORY_PROJECT_SLUG="..."
` + "```" + `

` + "```hcl" + `
provider "ory" {}  # Uses environment variables
` + "```" + `

#### Option 2: Provider Block (with Terraform variables)

` + "```hcl" + `
provider "ory" {
  workspace_api_key = var.ory_workspace_key  # or ORY_WORKSPACE_API_KEY env var
  workspace_id      = var.ory_workspace_id   # or ORY_WORKSPACE_ID env var
  project_api_key   = var.ory_project_key    # or ORY_PROJECT_API_KEY env var
  project_id        = var.ory_project_id     # or ORY_PROJECT_ID env var
  project_slug      = var.ory_project_slug   # or ORY_PROJECT_SLUG env var
}
` + "```" + `

When using Terraform variables, you can set them via ` + "`TF_VAR_*`" + ` environment variables:

` + "```bash" + `
export TF_VAR_ory_workspace_key="ory_wak_..."
export TF_VAR_ory_project_key="ory_pat_..."
` + "```" + `

## Which Credentials Do You Need?

| Resource | Required Credentials |
|----------|---------------------|
| ` + "`ory_project`" + `, ` + "`ory_workspace`" + ` | ` + "`workspace_api_key`" + `, ` + "`workspace_id`" + ` |
| ` + "`ory_organization`" + ` | ` + "`workspace_api_key`" + `, ` + "`project_id`" + ` |
| ` + "`ory_project_config`" + `, ` + "`ory_action`" + `, ` + "`ory_social_provider`" + `, ` + "`ory_email_template`" + ` | ` + "`workspace_api_key`" + `, ` + "`project_id`" + ` |
| ` + "`ory_identity`" + `, ` + "`ory_oauth2_client`" + `, ` + "`ory_relationship`" + ` | ` + "`project_api_key`" + `, ` + "`project_slug`" + ` |

## Import Requirements

When importing existing resources, ensure you have the appropriate credentials configured **before** running ` + "`terraform import`" + `.
`,
		Attributes: map[string]schema.Attribute{
			"workspace_api_key": schema.StringAttribute{
				Description:         "Ory Workspace API Key (ory_wak_...). Used for organization and project management. Can also be set via ORY_WORKSPACE_API_KEY environment variable.",
				MarkdownDescription: "Ory Workspace API Key (`ory_wak_...`). Used for organization and project management. Can also be set via `ORY_WORKSPACE_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"project_api_key": schema.StringAttribute{
				Description:         "Ory Project API Key (ory_pat_...). Used for identity and OAuth2 operations. Can also be set via ORY_PROJECT_API_KEY environment variable.",
				MarkdownDescription: "Ory Project API Key (`ory_pat_...`). Used for identity and OAuth2 operations. Can also be set via `ORY_PROJECT_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"project_id": schema.StringAttribute{
				Description:         "Ory Project ID. Can also be set via ORY_PROJECT_ID environment variable.",
				MarkdownDescription: "Ory Project ID. Can also be set via `ORY_PROJECT_ID` environment variable.",
				Optional:            true,
			},
			"project_slug": schema.StringAttribute{
				Description:         "Ory Project Slug (e.g., 'vibrant-moore-abc123'). Required for identity and OAuth2 operations. Can also be set via ORY_PROJECT_SLUG environment variable.",
				MarkdownDescription: "Ory Project Slug (e.g., `vibrant-moore-abc123`). Required for identity and OAuth2 operations. Can also be set via `ORY_PROJECT_SLUG` environment variable.",
				Optional:            true,
			},
			"workspace_id": schema.StringAttribute{
				Description:         "Ory Workspace ID. Can also be set via ORY_WORKSPACE_ID environment variable.",
				MarkdownDescription: "Ory Workspace ID. Can also be set via `ORY_WORKSPACE_ID` environment variable.",
				Optional:            true,
			},
			"console_api_url": schema.StringAttribute{
				Description:         "Override the console API URL (default: https://api.console.ory.sh). Mainly for testing.",
				MarkdownDescription: "Override the console API URL (default: `https://api.console.ory.sh`). Mainly for testing.",
				Optional:            true,
			},
			"project_api_url": schema.StringAttribute{
				Description:         "Override the project API URL template (default: https://%s.projects.oryapis.com).",
				MarkdownDescription: "Override the project API URL template (default: `https://%s.projects.oryapis.com`).",
				Optional:            true,
			},
		},
	}
}

func (p *OryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config OryProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve configuration with environment variable fallbacks
	workspaceAPIKey := resolveString(config.WorkspaceAPIKey, "ORY_WORKSPACE_API_KEY")
	projectAPIKey := resolveString(config.ProjectAPIKey, "ORY_PROJECT_API_KEY")
	projectID := resolveString(config.ProjectID, "ORY_PROJECT_ID")
	projectSlug := resolveString(config.ProjectSlug, "ORY_PROJECT_SLUG")
	workspaceID := resolveString(config.WorkspaceID, "ORY_WORKSPACE_ID")
	consoleAPIURL := resolveStringDefault(config.ConsoleAPIURL, "ORY_CONSOLE_API_URL", DefaultConsoleAPIURL)
	projectAPIURL := resolveStringDefault(config.ProjectAPIURL, "ORY_PROJECT_API_URL", DefaultProjectAPIURL)

	// Validate required configuration
	if workspaceAPIKey == "" && projectAPIKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspace_api_key"),
			"Missing Ory API Configuration",
			`At least one of workspace_api_key or project_api_key must be configured.

Configure via provider block:

  provider "ory" {
    workspace_api_key = var.ory_workspace_key  # For project/workspace/org operations
    project_api_key   = var.ory_project_key    # For identity/OAuth2 operations
  }

Or via environment variables:

  export ORY_WORKSPACE_API_KEY="ory_wak_..."
  export ORY_PROJECT_API_KEY="ory_pat_..."

Which API key do you need?
  - Workspace API Key (ory_wak_...): For ory_project, ory_workspace, ory_organization, ory_project_config, ory_action
  - Project API Key (ory_pat_...): For ory_identity, ory_oauth2_client, ory_relationship

For more information: https://www.ory.sh/docs/guides/api-keys`,
		)
		return
	}

	// Create the Ory client
	oryClient, err := client.NewOryClient(client.OryClientConfig{
		WorkspaceAPIKey: workspaceAPIKey,
		ProjectAPIKey:   projectAPIKey,
		ProjectID:       projectID,
		ProjectSlug:     projectSlug,
		WorkspaceID:     workspaceID,
		ConsoleAPIURL:   consoleAPIURL,
		ProjectAPIURL:   projectAPIURL,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Ory Client",
			"An error occurred creating the Ory API client: "+err.Error(),
		)
		return
	}

	// Make client available to resources and data sources
	resp.DataSourceData = oryClient
	resp.ResourceData = oryClient
}

func (p *OryProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		project.NewResource,
		workspace.NewResource,
		organization.NewResource,
		identity.NewResource,
		oauth2client.NewResource,
		projectconfig.NewResource,
		action.NewResource,
		identityschema.NewResource,
		socialprovider.NewResource,
		emailtemplate.NewResource,
		projectapikey.NewResource,
		jwk.NewResource,
		relationship.NewResource,
	}
}

func (p *OryProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		projectds.NewDataSource,
	}
}

// Helper functions

func resolveString(tfValue types.String, envVar string) string {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		return tfValue.ValueString()
	}
	return os.Getenv(envVar)
}

func resolveStringDefault(tfValue types.String, envVar, defaultValue string) string {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		return tfValue.ValueString()
	}
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return defaultValue
}
