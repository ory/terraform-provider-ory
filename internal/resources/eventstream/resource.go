package eventstream

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"strings"

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
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &EventStreamResource{}
	_ resource.ResourceWithConfigure      = &EventStreamResource{}
	_ resource.ResourceWithImportState    = &EventStreamResource{}
	_ resource.ResourceWithValidateConfig = &EventStreamResource{}
)

const (
	typeSNS   = "sns"
	typeHTTPS = "https"

	authTypeNone   = "none"
	authTypeAPIKey = "api_key"
	authTypeBearer = "bearer"

	statusActive = "active"
	statusPaused = "paused"

	// redactionMask replaces every credential the API reports back.
	redactionMask = "xxxxx"
)

// NewResource returns a new EventStream resource.
func NewResource() resource.Resource {
	return &EventStreamResource{}
}

// EventStreamResource defines the resource implementation.
type EventStreamResource struct {
	client *client.OryClient
}

const eventStreamMarkdownDescription = `
Manages an Ory Network event stream.

Event streams allow you to receive real-time events from your Ory Network
project in an AWS SNS topic or at an HTTPS endpoint. Events include identity
lifecycle changes, authentication events, and more.

~> **Important:** Event streams require an Ory Network **Enterprise plan**.

## Example Usage

` + "```hcl" + `
resource "ory_event_stream" "example" {
  type      = "sns"
  topic_arn = "arn:aws:sns:us-east-1:123456789012:ory-events"
  role_arn  = "arn:aws:iam::123456789012:role/ory-event-stream"
}

resource "ory_event_stream" "webhook" {
  type              = "https"
  https_endpoint    = "https://events.example.com/ory"
  https_auth_type   = "bearer"
  https_auth_secret = var.event_stream_token
}
` + "```" + `

## Import

Event streams can be imported using the format ` + "`project_id/event_stream_id`" + `:

` + "```shell" + `
terraform import ory_event_stream.example <project-id>/<event-stream-id>
` + "```" + `
`

// EventStreamResourceModel describes the resource data model.
type EventStreamResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	ProjectID                types.String `tfsdk:"project_id"`
	Type                     types.String `tfsdk:"type"`
	TopicArn                 types.String `tfsdk:"topic_arn"`
	RoleArn                  types.String `tfsdk:"role_arn"`
	HTTPSEndpoint            types.String `tfsdk:"https_endpoint"`
	HTTPSAuthType            types.String `tfsdk:"https_auth_type"`
	HTTPSAuthHeaderName      types.String `tfsdk:"https_auth_header_name"`
	HTTPSAuthSecret          types.String `tfsdk:"https_auth_secret"`
	HTTPSAuthSecretWO        types.String `tfsdk:"https_auth_secret_wo"`
	HTTPSAuthSecretWOVersion types.String `tfsdk:"https_auth_secret_wo_version"`
	Status                   types.String `tfsdk:"status"`
	CreatedAt                types.String `tfsdk:"created_at"`
	UpdatedAt                types.String `tfsdk:"updated_at"`
}

func (r *EventStreamResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_event_stream"
}

func (r *EventStreamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Ory Network event stream.",
		MarkdownDescription: eventStreamMarkdownDescription,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the event stream.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "The project ID. If not set, uses the provider's project_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Description: "The type of the event stream: \"sns\" publishes events to an AWS SNS topic, and \"https\" sends each event to an HTTPS endpoint in a POST request. Changing the type updates the stream in place.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(typeSNS, typeHTTPS),
				},
			},
			"topic_arn": schema.StringAttribute{
				Description: "The AWS SNS topic ARN to publish events to. Required when type is \"sns\".",
				Optional:    true,
			},
			"role_arn": schema.StringAttribute{
				Description: "The AWS IAM role ARN to assume when publishing to the SNS topic. Required when type is \"sns\".",
				Optional:    true,
			},
			"https_endpoint": schema.StringAttribute{
				Description: "The HTTPS URL that receives the events. Required when type is \"https\". Ory sends the credentials in the userinfo of the URL as Basic authentication. The API masks the userinfo and every query value on read, so Terraform keeps the configured URL and compares the masked forms to detect a change made outside Terraform.",
				Optional:    true,
				Sensitive:   true,
			},
			"https_auth_type": schema.StringAttribute{
				Description: "How deliveries to the HTTPS endpoint authenticate: \"none\", \"api_key\" to send the secret in the header named by https_auth_header_name, or \"bearer\" to send it in the Authorization header as a bearer token. Only \"none\" is valid when type is \"sns\". Defaults to \"none\".",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(authTypeNone),
				Validators: []validator.String{
					stringvalidator.OneOf(authTypeNone, authTypeAPIKey, authTypeBearer),
				},
			},
			"https_auth_header_name": schema.StringAttribute{
				Description: "The header that carries the API key. Required when https_auth_type is \"api_key\", and not valid with any other scheme. Ory rejects a header it sets itself, such as Content-Type or User-Agent, and a name that starts with Ory- or X-Ory-.",
				Optional:    true,
			},
			"https_auth_secret": schema.StringAttribute{
				Description: "The API key or bearer token, at most 4096 bytes. Stored in Terraform state. The API never returns it, so a change made outside Terraform is not detected. Changing the value rotates the secret. When https_auth_type is \"api_key\" or \"bearer\", exactly one of https_auth_secret and https_auth_secret_wo must be set.",
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(path.MatchRoot("https_auth_secret_wo")),
				},
			},
			"https_auth_secret_wo": schema.StringAttribute{
				Description: "Write-only equivalent of https_auth_secret for Terraform 1.11 and later: the value is sent to Ory but never stored in Terraform state or plan. Use it to source the secret from an ephemeral resource such as a Vault secret. Because write-only values are not persisted, Terraform cannot detect when the value changes on its own. Change https_auth_secret_wo_version to rotate it. Mutually exclusive with https_auth_secret.",
				Optional:    true,
				WriteOnly:   true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(path.MatchRoot("https_auth_secret")),
				},
			},
			"https_auth_secret_wo_version": schema.StringAttribute{
				Description: "Version trigger for https_auth_secret_wo. Change this value whenever the write-only secret changes so Terraform sends the new value to Ory. Has no effect unless https_auth_secret_wo is set.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("https_auth_secret_wo")),
				},
			},
			"status": schema.StringAttribute{
				Description: "The status of the event stream: \"active\" or \"paused\". A paused stream does not forward events, and Ory holds them for up to seven days until the stream is active again. Ory tests the destination on every create and update of an active stream, and skips the test for a paused stream. Defaults to \"active\".",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(statusActive),
				Validators: []validator.String{
					stringvalidator.OneOf(statusActive, statusPaused),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Timestamp when the event stream was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Description: "Timestamp when the event stream was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *EventStreamResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	oryClient, ok := req.ProviderData.(*client.OryClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.OryClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = oryClient
}

// ValidateConfig checks the settings that each type and authentication scheme
// accept. The API silently drops a setting that does not apply, so an apply
// that sent one would fail with an inconsistent result. A value that stays
// unknown until apply passes every check.
func (r *EventStreamResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config EventStreamResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	require := func(attr string, value types.String, when string) {
		if value.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(attr), "Missing Required Attribute",
				fmt.Sprintf("%s must be set when %s.", attr, when))
		}
	}
	forbid := func(attr string, value types.String, when string) {
		if !value.IsNull() && !value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(attr), "Invalid Attribute Combination",
				fmt.Sprintf("%s cannot be set when %s, because the API discards it.", attr, when))
		}
	}

	streamType := config.Type.ValueString()
	switch streamType {
	case typeSNS:
		require("topic_arn", config.TopicArn, `type is "sns"`)
		require("role_arn", config.RoleArn, `type is "sns"`)
		forbid("https_endpoint", config.HTTPSEndpoint, `type is "sns"`)
	case typeHTTPS:
		require("https_endpoint", config.HTTPSEndpoint, `type is "https"`)
		forbid("topic_arn", config.TopicArn, `type is "https"`)
		forbid("role_arn", config.RoleArn, `type is "https"`)
	}

	if config.HTTPSAuthType.IsUnknown() {
		return
	}
	authType := cmp.Or(config.HTTPSAuthType.ValueString(), authTypeNone)
	if streamType == typeSNS && authType != authTypeNone {
		resp.Diagnostics.AddAttributeError(path.Root("https_auth_type"), "Invalid Attribute Combination",
			`https_auth_type must be "none" when type is "sns", because the API discards the authentication of an SNS stream.`)
		return
	}
	when := fmt.Sprintf("https_auth_type is %q", authType)
	switch authType {
	case authTypeNone:
		forbid("https_auth_header_name", config.HTTPSAuthHeaderName, when)
		forbid("https_auth_secret", config.HTTPSAuthSecret, when)
		forbid("https_auth_secret_wo", config.HTTPSAuthSecretWO, when)
		return
	case authTypeAPIKey:
		require("https_auth_header_name", config.HTTPSAuthHeaderName, when)
	case authTypeBearer:
		forbid("https_auth_header_name", config.HTTPSAuthHeaderName, when)
	default:
		return
	}
	if config.HTTPSAuthSecret.IsNull() && config.HTTPSAuthSecretWO.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("https_auth_secret"), "Missing Required Attribute",
			fmt.Sprintf("https_auth_secret or https_auth_secret_wo must be set when %s.", when))
	}
}

// resolveSecret returns the secret to send, preferring the write-only value
// when the configuration carries one. A write-only value is nullified in the
// plan and the state, so it is read from the configuration at Create and
// Update. The result only builds the API payload and never reaches state.
func resolveSecret(ctx context.Context, config tfsdk.Config, plan *EventStreamResourceModel, diags *diag.Diagnostics) *string {
	var secretWO types.String
	diags.Append(config.GetAttribute(ctx, path.Root("https_auth_secret_wo"), &secretWO)...)
	if !secretWO.IsNull() && !secretWO.IsUnknown() {
		return secretWO.ValueStringPointer()
	}
	return plan.HTTPSAuthSecret.ValueStringPointer()
}

func (r *EventStreamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EventStreamResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(plan.ProjectID)
	if projectID == "" {
		resp.Diagnostics.AddError(
			"Missing Project ID",
			"project_id must be set either in the resource or provider configuration.",
		)
		return
	}

	secret := resolveSecret(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	body := ory.CreateEventStreamBody{
		Type:                plan.Type.ValueString(),
		TopicArn:            plan.TopicArn.ValueStringPointer(),
		RoleArn:             plan.RoleArn.ValueStringPointer(),
		HttpsEndpoint:       plan.HTTPSEndpoint.ValueStringPointer(),
		HttpsAuthType:       plan.HTTPSAuthType.ValueStringPointer(),
		HttpsAuthHeaderName: plan.HTTPSAuthHeaderName.ValueStringPointer(),
		HttpsAuthSecret:     secret,
		Status:              plan.Status.ValueStringPointer(),
	}

	stream, err := r.client.CreateEventStream(ctx, projectID, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Event Stream",
			"Could not create event stream: "+err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(stream.GetId())
	plan.ProjectID = types.StringValue(projectID)
	readStream(stream, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *EventStreamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EventStreamResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(state.ProjectID)

	stream, err := r.client.GetEventStream(ctx, projectID, state.ID.ValueString())
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(strings.ToLower(errStr), "not found") {
			resp.Diagnostics.AddWarning(
				"Event Stream Not Found",
				fmt.Sprintf("Event stream %s was not found (possibly deleted outside Terraform). Removing from state.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Event Stream",
			"Could not read event stream ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.ProjectID = types.StringValue(projectID)
	readStream(stream, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *EventStreamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EventStreamResourceModel
	var state EventStreamResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use state's project_id if plan doesn't have one (e.g., after import)
	projectID := r.resolveProjectID(plan.ProjectID)
	if projectID == "" {
		projectID = r.resolveProjectID(state.ProjectID)
	}

	secret := resolveSecret(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Every configured value is sent, the secret included: the API keeps a
	// stored secret only while the scheme and the endpoint stay the same.
	body := ory.SetEventStreamBody{
		Type:                plan.Type.ValueStringPointer(),
		TopicArn:            plan.TopicArn.ValueStringPointer(),
		RoleArn:             plan.RoleArn.ValueStringPointer(),
		HttpsEndpoint:       plan.HTTPSEndpoint.ValueStringPointer(),
		HttpsAuthType:       plan.HTTPSAuthType.ValueStringPointer(),
		HttpsAuthHeaderName: plan.HTTPSAuthHeaderName.ValueStringPointer(),
		HttpsAuthSecret:     secret,
		Status:              plan.Status.ValueStringPointer(),
	}

	stream, err := r.client.SetEventStream(ctx, projectID, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Event Stream",
			"Could not update event stream: "+err.Error(),
		)
		return
	}

	plan.ID = state.ID
	plan.ProjectID = types.StringValue(projectID)
	readStream(stream, &plan)
	// Preserve created_at from state - the API may return a slightly different timestamp format
	plan.CreatedAt = state.CreatedAt

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *EventStreamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EventStreamResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := r.resolveProjectID(state.ProjectID)

	err := r.client.DeleteEventStream(ctx, projectID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Event Stream",
			"Could not delete event stream: "+err.Error(),
		)
		return
	}
}

func (r *EventStreamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: project_id/event_stream_id or just event_stream_id (uses provider's project_id)
	id := req.ID
	var projectID, streamID string

	if strings.Contains(id, "/") {
		parts := strings.SplitN(id, "/", 2)
		projectID = parts[0]
		streamID = parts[1]
	} else {
		streamID = id
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), streamID)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}

func (r *EventStreamResource) resolveProjectID(tfProjectID types.String) string {
	if !tfProjectID.IsNull() && !tfProjectID.IsUnknown() {
		return tfProjectID.ValueString()
	}
	return r.client.ProjectID()
}

// readStream copies an API response onto the model. The API masks the
// credentials in https_endpoint and reports https_auth_secret only as a
// placeholder, so the model keeps the configured values while the response
// still matches them.
func readStream(stream *ory.EventStream, m *EventStreamResourceModel) {
	m.Type = types.StringValue(stream.GetType())
	m.TopicArn = types.StringPointerValue(stream.TopicArn)
	m.RoleArn = types.StringPointerValue(stream.RoleArn)
	m.HTTPSEndpoint = readEndpoint(m.HTTPSEndpoint, stream.HttpsEndpoint.Get())
	// The API reports no scheme for an SNS stream.
	m.HTTPSAuthType = types.StringValue(cmp.Or(stream.GetHttpsAuthType(), authTypeNone))
	m.HTTPSAuthHeaderName = types.StringPointerValue(stream.HttpsAuthHeaderName)
	if stream.HttpsAuthSecret == nil {
		// No secret is stored, for example after the scheme was set to none
		// outside Terraform.
		m.HTTPSAuthSecret = types.StringNull()
	}
	m.Status = types.StringValue(stream.GetStatus())
	m.CreatedAt = types.StringValue(stream.GetCreatedAt().String())
	m.UpdatedAt = types.StringValue(stream.GetUpdatedAt().String())
}

// readEndpoint keeps the configured endpoint while it masks to the reported
// one, and otherwise returns the reported endpoint so the plan shows the
// drift. After an import nothing is configured yet, so state takes the
// masked endpoint until the next apply writes the configured one.
func readEndpoint(configured types.String, reported *string) types.String {
	if reported == nil {
		return types.StringNull()
	}
	if !configured.IsNull() && maskEndpoint(configured.ValueString()) == *reported {
		return configured
	}
	return types.StringValue(*reported)
}

// maskEndpoint masks a URL the way the API reports it: the whole userinfo and
// every query value become xxxxx, and the query is re-encoded in key order.
func maskEndpoint(endpoint string) string {
	u, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return endpoint
	}
	if u.User != nil {
		u.User = url.User(redactionMask)
	}
	if u.RawQuery != "" {
		query := u.Query()
		for _, values := range query {
			for i := range values {
				values[i] = redactionMask
			}
		}
		u.RawQuery = query.Encode()
	}
	return u.String()
}
