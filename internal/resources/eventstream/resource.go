package eventstream

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &EventStreamResource{}
	_ resource.ResourceWithConfigure   = &EventStreamResource{}
	_ resource.ResourceWithImportState = &EventStreamResource{}
)

// NewResource returns a new EventStream resource.
func NewResource() resource.Resource {
	return &EventStreamResource{}
}

// EventStreamResource defines the resource implementation.
type EventStreamResource struct {
	client *client.OryClient
}

// EventStreamResourceModel describes the resource data model.
type EventStreamResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Type      types.String `tfsdk:"type"`
	TopicArn  types.String `tfsdk:"topic_arn"`
	RoleArn   types.String `tfsdk:"role_arn"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (r *EventStreamResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_event_stream"
}

func (r *EventStreamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Network event stream.",
		MarkdownDescription: `
Manages an Ory Network event stream.

Event streams allow you to receive real-time events from your Ory Network
project via AWS SNS. Events include identity lifecycle changes, authentication
events, and more.

~> **Important:** Event streams require an Ory Network **Enterprise plan**.

## Example Usage

` + "```hcl" + `
resource "ory_event_stream" "example" {
  type      = "sns"
  topic_arn = "arn:aws:sns:us-east-1:123456789012:ory-events"
  role_arn  = "arn:aws:iam::123456789012:role/ory-event-stream"
}
` + "```" + `

## Import

Event streams can be imported using the format ` + "`project_id/event_stream_id`" + `:

` + "```shell" + `
terraform import ory_event_stream.example <project-id>/<event-stream-id>
` + "```" + `
`,
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
				Description: "The type of the event stream (e.g., \"sns\").",
				Required:    true,
			},
			"topic_arn": schema.StringAttribute{
				Description: "The AWS SNS topic ARN to publish events to.",
				Required:    true,
			},
			"role_arn": schema.StringAttribute{
				Description: "The AWS IAM role ARN to assume when publishing to the SNS topic.",
				Required:    true,
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

	body := ory.CreateEventStreamBody{
		Type:     plan.Type.ValueString(),
		TopicArn: plan.TopicArn.ValueString(),
		RoleArn:  plan.RoleArn.ValueString(),
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
	plan.Type = types.StringValue(stream.GetType())
	plan.TopicArn = types.StringValue(stream.GetTopicArn())
	plan.RoleArn = types.StringValue(stream.GetRoleArn())
	plan.CreatedAt = types.StringValue(stream.GetCreatedAt().String())
	plan.UpdatedAt = types.StringValue(stream.GetUpdatedAt().String())

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
		resp.Diagnostics.AddError(
			"Error Reading Event Stream",
			"Could not read event stream ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.ProjectID = types.StringValue(projectID)
	state.Type = types.StringValue(stream.GetType())
	state.TopicArn = types.StringValue(stream.GetTopicArn())
	state.RoleArn = types.StringValue(stream.GetRoleArn())
	state.CreatedAt = types.StringValue(stream.GetCreatedAt().String())
	state.UpdatedAt = types.StringValue(stream.GetUpdatedAt().String())

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

	body := ory.SetEventStreamBody{
		Type:     plan.Type.ValueString(),
		TopicArn: plan.TopicArn.ValueString(),
		RoleArn:  plan.RoleArn.ValueString(),
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
	plan.Type = types.StringValue(stream.GetType())
	plan.TopicArn = types.StringValue(stream.GetTopicArn())
	plan.RoleArn = types.StringValue(stream.GetRoleArn())
	// Preserve created_at from state - the API may return a slightly different timestamp format
	plan.CreatedAt = state.CreatedAt
	plan.UpdatedAt = types.StringValue(stream.GetUpdatedAt().String())

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
