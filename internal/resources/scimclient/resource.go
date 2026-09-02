package scimclient

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ resource.Resource                   = &SCIMClientResource{}
	_ resource.ResourceWithConfigure      = &SCIMClientResource{}
	_ resource.ResourceWithImportState    = &SCIMClientResource{}
	_ resource.ResourceWithValidateConfig = &SCIMClientResource{}
)

const (
	// scimClientsPath is the JSON Pointer of the SCIM clients array on the
	// normalized project revision. SCIM clients have no Console endpoint of
	// their own: the array is written through
	// PATCH /normalized/projects/{id}/revision/{rev} and read through
	// GET /normalized/projects/{id}.
	scimClientsPath = "/scim_clients"

	stateEnabled  = "enabled"
	stateDisabled = "disabled"
)

var (
	// clientIDRegex is the pattern the API enforces on client_id. The value is
	// a path segment of the SCIM URL, and the API rejects anything else with
	// HTTP 400.
	clientIDRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

	// mapperURLRegex matches the prefixes the API accepts for a mapper.
	mapperURLRegex = regexp.MustCompile(`^(base64://|https?://)`)

	// errClientNotFound reports that an update targets a client_id the
	// revision no longer carries.
	errClientNotFound = errors.New("SCIM client not found")

	// readRetryBaseDelay is the first backoff step Read waits when the client
	// is missing from the revision. Tests shorten it.
	readRetryBaseDelay = time.Second
)

func NewResource() resource.Resource {
	return &SCIMClientResource{}
}

type SCIMClientResource struct {
	client *client.OryClient
}

type SCIMClientResourceModel struct {
	ID                                 types.String `tfsdk:"id"`
	ProjectID                          types.String `tfsdk:"project_id"`
	OrganizationID                     types.String `tfsdk:"organization_id"`
	ClientID                           types.String `tfsdk:"client_id"`
	Label                              types.String `tfsdk:"label"`
	MapperURL                          types.String `tfsdk:"mapper_url"`
	AuthorizationHeaderSecret          types.String `tfsdk:"authorization_header_secret"`
	AuthorizationHeaderSecretWO        types.String `tfsdk:"authorization_header_secret_wo"`
	AuthorizationHeaderSecretWOVersion types.String `tfsdk:"authorization_header_secret_wo_version"`
	State                              types.String `tfsdk:"state"`
}

func (r *SCIMClientResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_client"
}

func (r *SCIMClientResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network SCIM client that lets an external identity provider provision identities into an organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID. The same value as client_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "Project ID. If not set, uses the provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "ID of the organization the SCIM client provisions identities into. It must be an organization in the same project. Deleting the organization deletes the SCIM client server-side, and the next plan then recreates both.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"client_id": schema.StringAttribute{
				Description: "Unique identifier of the SCIM client within the project. It is the path segment of the SCIM base URL, https://<project-slug>.projects.oryapis.com/scim/<client_id>/v2, and must match ^[a-z0-9_-]+$. Changing it forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(clientIDRegex, "must contain only lowercase letters, digits, underscores, and hyphens"),
				},
			},
			"label": schema.StringAttribute{
				Description: "Human-readable name of the SCIM client, shown in the Ory Console.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"mapper_url": schema.StringAttribute{
				Description: "Jsonnet mapper that maps the SCIM user payload to identity traits. Accepts a base64-encoded Jsonnet snippet prefixed with base64://, or an http or https URL that Ory downloads at write time. Ory stores the payload at a content-addressed object-storage URL and reports that URL on read. The provider keeps the configured value in state and reads the stored URL only on import, so the value never shows a spurious diff.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(mapperURLRegex, "must start with base64://, http://, or https://"),
				},
			},
			"authorization_header_secret": schema.StringAttribute{
				Description: "Bearer token the identity provider sends in the Authorization header of every SCIM request. Stored in Terraform state. The API never returns it, so a change made outside Terraform is not detected. Changing the value rotates the secret. Exactly one of authorization_header_secret and authorization_header_secret_wo must be set.",
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(path.MatchRoot("authorization_header_secret_wo")),
				},
			},
			"authorization_header_secret_wo": schema.StringAttribute{
				Description: "Write-only equivalent of authorization_header_secret for Terraform 1.11 and later: the value is sent to Ory but never stored in Terraform state or plan. Use it to source the secret from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own. Change authorization_header_secret_wo_version to rotate it. Mutually exclusive with authorization_header_secret.",
				Optional:    true,
				WriteOnly:   true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(path.MatchRoot("authorization_header_secret")),
				},
			},
			"authorization_header_secret_wo_version": schema.StringAttribute{
				Description: "Version trigger for authorization_header_secret_wo. Change this value whenever the write-only secret changes so Terraform sends the new value to Ory. Has no effect unless authorization_header_secret_wo is set.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("authorization_header_secret_wo")),
				},
			},
			"state": schema.StringAttribute{
				Description: "State of the SCIM client, enabled or disabled. Only an enabled client serves SCIM requests. A disabled client answers HTTP 404 on its SCIM endpoint. Defaults to enabled.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(stateEnabled),
				Validators: []validator.String{
					stringvalidator.OneOf(stateEnabled, stateDisabled),
				},
			},
		},
	}
}

func (r *SCIMClientResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T", req.ProviderData))
		return
	}
	r.client = oryClient
}

// ValidateConfig requires a secret from one of the two sources. The mutual
// exclusion and the non-empty checks are schema validators. IsNull is false
// for an unknown value, so a secret that is not known until apply passes.
func (r *SCIMClientResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config SCIMClientResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.AuthorizationHeaderSecret.IsNull() && config.AuthorizationHeaderSecretWO.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("authorization_header_secret"),
			"Missing Required Attribute",
			"Either authorization_header_secret or authorization_header_secret_wo must be set. The SCIM server rejects every request for a client without a secret.",
		)
	}
}

// resolveSecret returns the secret to send, preferring the write-only value
// when the configuration carries one. A write-only value is nullified in the
// plan and the state, so it is read from the configuration at Create and
// Update. The result only builds the API payload and never reaches state.
func (r *SCIMClientResource) resolveSecret(ctx context.Context, config tfsdk.Config, plan *SCIMClientResourceModel, diags *diag.Diagnostics) string {
	var secretWO types.String
	diags.Append(config.GetAttribute(ctx, path.Root("authorization_header_secret_wo"), &secretWO)...)
	if !secretWO.IsNull() && !secretWO.IsUnknown() {
		return secretWO.ValueString()
	}
	return plan.AuthorizationHeaderSecret.ValueString()
}

func (r *SCIMClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SCIMClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	secret := r.resolveSecret(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	clientID := plan.ClientID.ValueString()
	revision, err := r.client.PatchProjectRevisionWith(ctx, projectID, func(revision map[string]interface{}) ([]ory.JsonPatch, error) {
		clients := clientsFromRevision(revision)
		if index := findClientIndex(clients, clientID); index >= 0 {
			// A client with this client_id already exists, for example one
			// created in the Console or left by an apply whose response was
			// lost. Take it over and rewrite it, as ory_social_provider and
			// ory_saml_provider do for their providers.
			return []ory.JsonPatch{{
				Op:    "replace",
				Path:  clientPath(index),
				Value: buildClientObject(&plan, secret, rowID(clients[index])),
			}}, nil
		}
		value := buildClientObject(&plan, secret, "")
		if len(clients) == 0 {
			// The first client writes the whole array. A JSON Patch "add" on
			// an existing member replaces it (RFC 6902), so this also covers a
			// revision that carries an empty array.
			return []ory.JsonPatch{{Op: "add", Path: scimClientsPath, Value: []interface{}{value}}}, nil
		}
		return []ory.JsonPatch{{Op: "add", Path: scimClientsPath + "/-", Value: value}}, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Creating SCIM Client", apiErrorDetail(err, projectID))
		return
	}

	if !verifyWritten(revision, clientID, &resp.Diagnostics) {
		return
	}
	plan.ID = types.StringValue(clientID)
	plan.ProjectID = types.StringValue(projectID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *SCIMClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SCIMClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	clientID := state.ClientID.ValueString()
	if clientID == "" {
		resp.Diagnostics.AddError("Missing Client ID",
			"client_id is empty in state. This is a bug, please report it.")
		return
	}

	found, err := r.findClient(ctx, projectID, clientID)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading SCIM Client",
			fmt.Sprintf("Failed to read the SCIM clients of project %s: %v", projectID, err))
		return
	}
	if found == nil {
		// Deleted in the Console, or removed together with its organization.
		resp.State.RemoveResource(ctx)
		return
	}

	state.Label = stringOrNull(found["label"])
	state.OrganizationID = stringOrNull(found["organization_id"])
	state.State = stringOrNull(found["state"])

	// Ory rewrites mapper_url into a content-addressed object-storage URL on
	// every write, so the reported value never equals the configured one.
	// Keep the configured value and read the stored URL only when state has
	// none, which is the case after an import.
	if state.MapperURL.IsNull() {
		state.MapperURL = stringOrNull(found["mapper_url"])
	}

	// authorization_header_secret is redacted to "" in every response. State
	// keeps the configured value, and stays null for the write-only variant.

	state.ID = types.StringValue(clientID)
	state.ProjectID = types.StringValue(projectID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// findClient returns the revision entry for clientID, or nil when the project
// has no such client. A missing client is re-read with backoff first, because
// a read right after a write can still report the previous revision.
func (r *SCIMClientResource) findClient(ctx context.Context, projectID, clientID string) (map[string]interface{}, error) {
	for attempt := 0; attempt < helpers.ReadRetryMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("read canceled while retrying project %s: %w", projectID, ctx.Err())
			case <-time.After(readRetryBaseDelay << (attempt - 1)):
			}
		}
		revision, err := r.client.GetProjectNormalizedRevision(ctx, projectID)
		if err != nil {
			return nil, err
		}
		clients := clientsFromRevision(revision)
		if index := findClientIndex(clients, clientID); index >= 0 {
			return clients[index], nil
		}
	}
	return nil, nil
}

func (r *SCIMClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SCIMClientResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	secret := r.resolveSecret(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	clientID := plan.ClientID.ValueString()
	revision, err := r.client.PatchProjectRevisionWith(ctx, projectID, func(revision map[string]interface{}) ([]ory.JsonPatch, error) {
		clients := clientsFromRevision(revision)
		index := findClientIndex(clients, clientID)
		if index < 0 {
			return nil, fmt.Errorf("%w: %q in project %s", errClientNotFound, clientID, projectID)
		}
		// The row id is the identity the server matches on when it carries
		// the stored secret over. An empty secret is never sent here, but the
		// id keeps the match strict when a client_id is reused.
		return []ory.JsonPatch{{
			Op:    "replace",
			Path:  clientPath(index),
			Value: buildClientObject(&plan, secret, rowID(clients[index])),
		}}, nil
	})
	if err != nil {
		if errors.Is(err, errClientNotFound) {
			resp.Diagnostics.AddError("SCIM Client Not Found",
				err.Error()+". It was removed outside Terraform. The next plan recreates it.")
			return
		}
		resp.Diagnostics.AddError("Error Updating SCIM Client", apiErrorDetail(err, projectID))
		return
	}

	if !verifyWritten(revision, clientID, &resp.Diagnostics) {
		return
	}
	plan.ID = types.StringValue(clientID)
	plan.ProjectID = types.StringValue(projectID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *SCIMClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SCIMClientResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	clientID := state.ClientID.ValueString()
	_, err := r.client.PatchProjectRevisionWith(ctx, projectID, func(revision map[string]interface{}) ([]ory.JsonPatch, error) {
		index := findClientIndex(clientsFromRevision(revision), clientID)
		if index < 0 {
			// Already gone, for example removed together with its organization.
			return nil, nil
		}
		return []ory.JsonPatch{{Op: "remove", Path: clientPath(index)}}, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting SCIM Client", apiErrorDetail(err, projectID))
	}
}

// ImportState accepts <project-id>/<client-id>, or <client-id> alone to use
// the provider's project_id.
func (r *SCIMClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, clientID := splitImportID(req.ID)
	if clientID == "" {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Expected <project-id>/<client-id> or <client-id>, got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), clientID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_id"), clientID)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}
