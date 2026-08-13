package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ory/terraform-provider-ory/internal/client"
	identityds "github.com/ory/terraform-provider-ory/internal/datasources/identity"
	identityschemads "github.com/ory/terraform-provider-ory/internal/datasources/identityschema"
	identityschemasds "github.com/ory/terraform-provider-ory/internal/datasources/identityschemas"
	oauth2clientds "github.com/ory/terraform-provider-ory/internal/datasources/oauth2client"
	organizationds "github.com/ory/terraform-provider-ory/internal/datasources/organization"
	projectds "github.com/ory/terraform-provider-ory/internal/datasources/project"
	workspaceds "github.com/ory/terraform-provider-ory/internal/datasources/workspace"
	"github.com/ory/terraform-provider-ory/internal/resources/action"
	"github.com/ory/terraform-provider-ory/internal/resources/customdomain"
	"github.com/ory/terraform-provider-ory/internal/resources/emailtemplate"
	"github.com/ory/terraform-provider-ory/internal/resources/eventstream"
	"github.com/ory/terraform-provider-ory/internal/resources/identity"
	"github.com/ory/terraform-provider-ory/internal/resources/identityschema"
	"github.com/ory/terraform-provider-ory/internal/resources/jwk"
	"github.com/ory/terraform-provider-ory/internal/resources/oauth2client"
	"github.com/ory/terraform-provider-ory/internal/resources/oidcdynamicclient"
	"github.com/ory/terraform-provider-ory/internal/resources/organization"
	"github.com/ory/terraform-provider-ory/internal/resources/project"
	"github.com/ory/terraform-provider-ory/internal/resources/projectapikey"
	"github.com/ory/terraform-provider-ory/internal/resources/projectconfig"
	"github.com/ory/terraform-provider-ory/internal/resources/relationship"
	"github.com/ory/terraform-provider-ory/internal/resources/samlprovider"
	"github.com/ory/terraform-provider-ory/internal/resources/socialprovider"
	"github.com/ory/terraform-provider-ory/internal/resources/trustedjwtissuer"
	"github.com/ory/terraform-provider-ory/internal/resources/workspace"
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

	// Reuse the OryClient across Configure calls to preserve cached state.
	// Terraform calls ConfigureProvider for each operation (apply, plan/refresh),
	// which would otherwise create a new client and lose the project cache.
	oryClient  *client.OryClient
	lastConfig client.OryClientConfig
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

	// Optional safety guardrail: restrict which project IDs may be touched
	AllowedProjectIDs types.List `tfsdk:"allowed_project_ids"`

	// Optional: Override API URLs (for testing)
	ConsoleAPIURL types.String `tfsdk:"console_api_url"`
	ProjectAPIURL types.String `tfsdk:"project_api_url"`

	// Optional: how many times a rate-limited request is retried
	MaxRetries types.Int64 `tfsdk:"max_retries"`
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
| ` + "`ory_project_config`" + `, ` + "`ory_action`" + `, ` + "`ory_social_provider`" + `, ` + "`ory_saml_provider`" + `, ` + "`ory_email_template`" + ` | ` + "`workspace_api_key`" + `, ` + "`project_id`" + ` |
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
			"allowed_project_ids": schema.ListAttribute{
				Description:         "Optional safety guardrail. When set, the provider refuses any project-configuration operation (project config, actions, email templates, social/SAML providers, identity schema, custom domains, event streams, organizations, project API keys, and project create/delete) that targets a project ID not in this list. This bounds the blast radius of a workspace API key so a mis-pointed project_id, such as production, cannot be read or changed. When unset, no restriction is applied. Can also be set via the ORY_ALLOWED_PROJECT_IDS environment variable as a comma-separated list.",
				MarkdownDescription: "Optional safety guardrail. When set, the provider refuses any project-configuration operation (`ory_project_config`, `ory_action`, `ory_email_template`, `ory_social_provider`, `ory_saml_provider`, `ory_identity_schema`, `ory_custom_domain`, `ory_event_stream`, `ory_organization`, `ory_project_api_key`, and `ory_project` create/delete) that targets a project ID not in this list. This bounds the blast radius of a workspace API key so a mis-pointed `project_id` (such as production) cannot be read or changed. When unset, no restriction is applied. Can also be set via the `ORY_ALLOWED_PROJECT_IDS` environment variable as a comma-separated list.",
				Optional:            true,
				ElementType:         types.StringType,
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
			"max_retries": schema.Int64Attribute{
				Description: fmt.Sprintf(
					"How many times a request that the Ory API rejects with HTTP 429 Too Many Requests is retried before the error reaches Terraform (default: %d, maximum: %d). "+
						"The provider backs off exponentially, capped at 30 seconds, waits longer when the x-ratelimit-reset header reports a longer window, and adds random jitter so parallel workers do not retry together. "+
						"Raise it for a very large apply; set it to 0 to disable the retry. Can also be set via the ORY_MAX_RETRIES environment variable.",
					client.DefaultMaxRetries, client.MaxRetriesUpperBound),
				MarkdownDescription: fmt.Sprintf(
					"How many times a request that the Ory API rejects with `429 Too Many Requests` is retried before the error reaches Terraform (default: `%d`, maximum: `%d`). "+
						"The provider backs off exponentially, capped at 30 seconds, waits longer when the `x-ratelimit-reset` header reports a longer window, and adds random jitter so parallel workers do not retry together. "+
						"Raise it for a very large apply; set it to `0` to disable the retry. Can also be set via the `ORY_MAX_RETRIES` environment variable.",
					client.DefaultMaxRetries, client.MaxRetriesUpperBound),
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, client.MaxRetriesUpperBound),
				},
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
	allowedProjectIDs := resolveAllowedProjectIDs(ctx, config.AllowedProjectIDs, &resp.Diagnostics)
	maxRetries := resolveMaxRetries(config.MaxRetries, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

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
  - Workspace API Key (ory_wak_...): For ory_project, ory_workspace, ory_organization, ory_project_config, ory_action, ory_social_provider, ory_saml_provider, ory_email_template
  - Project API Key (ory_pat_...): For ory_identity, ory_oauth2_client, ory_relationship

For more information: https://www.ory.sh/docs/guides/api-keys`,
		)
		return
	}

	userAgent := buildUserAgent(req.TerraformVersion, p.version)

	// Reuse the existing client if the config hasn't changed.
	// This preserves cached project state across Terraform operations
	// (apply → plan/refresh) within the same provider server lifecycle.
	newConfig := client.OryClientConfig{
		WorkspaceAPIKey:   workspaceAPIKey,
		ProjectAPIKey:     projectAPIKey,
		ProjectID:         projectID,
		ProjectSlug:       projectSlug,
		WorkspaceID:       workspaceID,
		ConsoleAPIURL:     consoleAPIURL,
		ProjectAPIURL:     projectAPIURL,
		UserAgent:         userAgent,
		AllowedProjectIDs: allowedProjectIDs,
		MaxRetries:        maxRetries,
	}

	if p.oryClient == nil || !p.lastConfig.Equal(newConfig) {
		oryClient, err := client.NewOryClient(newConfig)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Create Ory Client",
				"An error occurred creating the Ory API client: "+err.Error(),
			)
			return
		}
		p.oryClient = oryClient
		p.lastConfig = newConfig
	}

	resp.DataSourceData = p.oryClient
	resp.ResourceData = p.oryClient
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
		samlprovider.NewResource,
		emailtemplate.NewResource,
		projectapikey.NewResource,
		jwk.NewResource,
		relationship.NewResource,
		eventstream.NewResource,
		trustedjwtissuer.NewResource,
		oidcdynamicclient.NewResource,
		customdomain.NewResource,
	}
}

func (p *OryProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		projectds.NewDataSource,
		workspaceds.NewDataSource,
		identityds.NewDataSource,
		oauth2clientds.NewDataSource,
		organizationds.NewDataSource,
		identityschemads.NewDataSource,
		identityschemasds.NewDataSource,
	}
}

// Helper functions

// buildUserAgent constructs the User-Agent string for API requests.
func buildUserAgent(tfVersion, providerVersion string) string {
	ua := "Terraform/" + tfVersion + " (+https://www.terraform.io) terraform-provider-ory/" + providerVersion
	if appendUA := os.Getenv("TF_APPEND_USER_AGENT"); appendUA != "" {
		ua += " " + appendUA
	}
	return ua
}

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

// resolveAllowedProjectIDs resolves the allowed_project_ids list with a
// comma-separated ORY_ALLOWED_PROJECT_IDS environment variable fallback. When
// the Terraform value is set it takes precedence; otherwise the env var is split
// on commas. Entries are trimmed and empty entries dropped. Returns nil when
// neither source yields any value.
func resolveAllowedProjectIDs(ctx context.Context, tfValue types.List, diags *diag.Diagnostics) []string {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		var out []string
		diags.Append(tfValue.ElementsAs(ctx, &out, false)...)
		return normalizeStringList(out)
	}
	if v := os.Getenv("ORY_ALLOWED_PROJECT_IDS"); v != "" {
		return normalizeStringList(strings.Split(v, ","))
	}
	return nil
}

// resolveMaxRetries resolves max_retries with an ORY_MAX_RETRIES environment
// variable fallback. It returns nil when neither source sets a value, which
// leaves the client on its default. A pointer to 0 is a real setting that turns
// the retry off, so the two cases must stay apart.
func resolveMaxRetries(tfValue types.Int64, diags *diag.Diagnostics) *int {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		value := int(tfValue.ValueInt64())
		return &value
	}
	raw := strings.TrimSpace(os.Getenv("ORY_MAX_RETRIES"))
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > client.MaxRetriesUpperBound {
		diags.AddError(
			"Invalid ORY_MAX_RETRIES Value",
			fmt.Sprintf("ORY_MAX_RETRIES must be a whole number between 0 and %d, got %q. "+
				"It sets how many times a request rejected with HTTP 429 is retried.",
				client.MaxRetriesUpperBound, raw),
		)
		return nil
	}
	return &value
}

// normalizeStringList trims whitespace from each entry and drops empty strings.
// Returns nil when nothing remains.
func normalizeStringList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
