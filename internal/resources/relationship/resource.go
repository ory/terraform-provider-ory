package relationship

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
	_ resource.Resource                = &RelationshipResource{}
	_ resource.ResourceWithConfigure   = &RelationshipResource{}
	_ resource.ResourceWithImportState = &RelationshipResource{}
)

// NewResource returns a new Relationship resource.
func NewResource() resource.Resource {
	return &RelationshipResource{}
}

// RelationshipResource defines the resource implementation.
type RelationshipResource struct {
	client *client.OryClient
}

// RelationshipResourceModel describes the resource data model.
type RelationshipResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Namespace           types.String `tfsdk:"namespace"`
	Object              types.String `tfsdk:"object"`
	Relation            types.String `tfsdk:"relation"`
	SubjectID           types.String `tfsdk:"subject_id"`
	SubjectSetNamespace types.String `tfsdk:"subject_set_namespace"`
	SubjectSetObject    types.String `tfsdk:"subject_set_object"`
	SubjectSetRelation  types.String `tfsdk:"subject_set_relation"`
}

func (r *RelationshipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_relationship"
}

func (r *RelationshipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Ory Keto relationship tuple.",
		MarkdownDescription: `
Manages an Ory Keto relationship tuple for fine-grained authorization.

Relationships are the foundation of Ory Keto's permission system. They define
who (subject) has what relation to which resource (object) in a namespace.

## Relationship Tuple Structure

A relationship tuple consists of:
- **namespace**: The type of resource (e.g., "documents", "folders")
- **object**: The specific resource ID
- **relation**: The type of relationship (e.g., "viewer", "editor", "owner")
- **subject**: Either a user ID or another relationship tuple (subject set)

## Example Usage

### Simple User Permission

` + "```hcl" + `
resource "ory_relationship" "user_can_view" {
  namespace  = "documents"
  object     = "doc-123"
  relation   = "viewer"
  subject_id = "user-456"
}
` + "```" + `

### Subject Set (Inherited Permission)

Grant all editors of a folder access to view documents in that folder:

` + "```hcl" + `
resource "ory_relationship" "editors_can_view" {
  namespace             = "documents"
  object                = "doc-123"
  relation              = "viewer"
  subject_set_namespace = "folders"
  subject_set_object    = "folder-789"
  subject_set_relation  = "editor"
}
` + "```" + `

This means: anyone who is an "editor" of "folder-789" in the "folders" namespace
is automatically a "viewer" of "doc-123" in the "documents" namespace.

## Note

You must configure Ory Keto namespaces and permissions in your project
configuration before creating relationships. Use ` + "`ory_project_config`" + ` or
configure via the Ory Console.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Internal Terraform ID (composite of namespace:object#relation@subject).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace": schema.StringAttribute{
				Description: "The namespace of the relationship (e.g., 'documents', 'folders').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"object": schema.StringAttribute{
				Description: "The object ID in the namespace.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"relation": schema.StringAttribute{
				Description: "The relation type (e.g., 'viewer', 'editor', 'owner').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject_id": schema.StringAttribute{
				Description: "The subject ID (user ID). Mutually exclusive with subject_set_* attributes.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject_set_namespace": schema.StringAttribute{
				Description: "The namespace for a subject set. Use with subject_set_object and subject_set_relation.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject_set_object": schema.StringAttribute{
				Description: "The object ID for a subject set.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject_set_relation": schema.StringAttribute{
				Description: "The relation for a subject set.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *RelationshipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RelationshipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RelationshipResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that either subject_id or subject_set is provided, not both
	hasSubjectID := !plan.SubjectID.IsNull() && !plan.SubjectID.IsUnknown()
	hasSubjectSet := (!plan.SubjectSetNamespace.IsNull() && !plan.SubjectSetNamespace.IsUnknown()) ||
		(!plan.SubjectSetObject.IsNull() && !plan.SubjectSetObject.IsUnknown()) ||
		(!plan.SubjectSetRelation.IsNull() && !plan.SubjectSetRelation.IsUnknown())

	if hasSubjectID && hasSubjectSet {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Cannot specify both subject_id and subject_set_* attributes. Use one or the other.",
		)
		return
	}

	if !hasSubjectID && !hasSubjectSet {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Must specify either subject_id or all subject_set_* attributes.",
		)
		return
	}

	if hasSubjectSet {
		// Validate all subject_set fields are provided
		if plan.SubjectSetNamespace.IsNull() || plan.SubjectSetObject.IsNull() || plan.SubjectSetRelation.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"When using subject_set, all of subject_set_namespace, subject_set_object, and subject_set_relation must be provided.",
			)
			return
		}
	}

	body := ory.CreateRelationshipBody{
		Namespace: ory.PtrString(plan.Namespace.ValueString()),
		Object:    ory.PtrString(plan.Object.ValueString()),
		Relation:  ory.PtrString(plan.Relation.ValueString()),
	}

	if hasSubjectID {
		body.SubjectId = ory.PtrString(plan.SubjectID.ValueString())
	} else {
		body.SubjectSet = &ory.SubjectSet{
			Namespace: plan.SubjectSetNamespace.ValueString(),
			Object:    plan.SubjectSetObject.ValueString(),
			Relation:  plan.SubjectSetRelation.ValueString(),
		}
	}

	rel, err := r.client.CreateRelationship(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Relationship",
			"Could not create relationship: "+err.Error(),
		)
		return
	}

	// Generate composite ID
	plan.ID = types.StringValue(generateRelationshipID(rel))

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *RelationshipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RelationshipResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query for the specific relationship
	var subjectID *string
	if !state.SubjectID.IsNull() && !state.SubjectID.IsUnknown() {
		s := state.SubjectID.ValueString()
		subjectID = &s
	}

	object := state.Object.ValueString()
	relation := state.Relation.ValueString()

	rels, err := r.client.GetRelationships(ctx, state.Namespace.ValueString(), &object, &relation, subjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Relationship",
			"Could not read relationship: "+err.Error(),
		)
		return
	}

	// Find our specific relationship
	found := false
	if rels != nil && rels.RelationTuples != nil {
		for _, rel := range rels.RelationTuples {
			if matchesRelationship(&state, &rel) {
				found = true
				break
			}
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RelationshipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Relationships are immutable - all changes require replacement
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Relationships cannot be updated. Changes require resource replacement.",
	)
}

func (r *RelationshipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RelationshipResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var subjectID *string
	if !state.SubjectID.IsNull() && !state.SubjectID.IsUnknown() {
		s := state.SubjectID.ValueString()
		subjectID = &s
	}

	object := state.Object.ValueString()
	relation := state.Relation.ValueString()

	err := r.client.DeleteRelationships(ctx, state.Namespace.ValueString(), &object, &relation, subjectID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Relationship",
			"Could not delete relationship: "+err.Error(),
		)
		return
	}
}

// ImportState imports an existing relationship into Terraform state.
// Import ID format: namespace:object#relation@subject
// Where subject is either:
// - A simple subject_id (e.g., "user-456")
// - A subject_set in format "namespace:object#relation" (e.g., "folders:folder-789#editor")
//
// Examples:
// - Simple: documents:doc-123#viewer@user-456
// - Subject set: documents:doc-123#viewer@folders:folder-789#editor
func (r *RelationshipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID

	// Parse: namespace:object#relation@subject
	// First split by @ to get subject
	atIdx := strings.LastIndex(id, "@")
	if atIdx == -1 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in format: namespace:object#relation@subject. Missing '@' separator.",
		)
		return
	}

	prefix := id[:atIdx]
	subject := id[atIdx+1:]

	// Parse prefix: namespace:object#relation
	hashIdx := strings.LastIndex(prefix, "#")
	if hashIdx == -1 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in format: namespace:object#relation@subject. Missing '#' separator.",
		)
		return
	}

	namespaceObject := prefix[:hashIdx]
	relation := prefix[hashIdx+1:]

	// Parse namespaceObject: namespace:object
	colonIdx := strings.Index(namespaceObject, ":")
	if colonIdx == -1 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in format: namespace:object#relation@subject. Missing ':' separator in namespace:object.",
		)
		return
	}

	namespace := namespaceObject[:colonIdx]
	object := namespaceObject[colonIdx+1:]

	// Set the basic attributes
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), namespace)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("object"), object)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("relation"), relation)...)

	// Check if subject is a subject_set (contains : and #) or simple subject_id
	subjectColonIdx := strings.Index(subject, ":")
	subjectHashIdx := strings.Index(subject, "#")
	if subjectColonIdx > 0 && subjectHashIdx > subjectColonIdx {
		// Subject set format: namespace:object#relation
		subjectNamespaceObject := subject[:subjectHashIdx]
		subjectRelation := subject[subjectHashIdx+1:]

		subjectNamespace := subjectNamespaceObject[:subjectColonIdx]
		subjectObject := subjectNamespaceObject[subjectColonIdx+1:]

		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subject_set_namespace"), subjectNamespace)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subject_set_object"), subjectObject)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subject_set_relation"), subjectRelation)...)
	} else {
		// Simple subject_id
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subject_id"), subject)...)
	}
}

// generateRelationshipID creates a composite ID for the relationship
func generateRelationshipID(rel *ory.Relationship) string {
	subject := ""
	if rel.SubjectId != nil {
		subject = *rel.SubjectId
	} else if rel.SubjectSet != nil {
		subject = fmt.Sprintf("%s:%s#%s", rel.SubjectSet.Namespace, rel.SubjectSet.Object, rel.SubjectSet.Relation)
	}
	return fmt.Sprintf("%s:%s#%s@%s", rel.Namespace, rel.Object, rel.Relation, subject)
}

// matchesRelationship checks if a relationship matches the state
func matchesRelationship(state *RelationshipResourceModel, rel *ory.Relationship) bool {
	if rel.Namespace != state.Namespace.ValueString() ||
		rel.Object != state.Object.ValueString() ||
		rel.Relation != state.Relation.ValueString() {
		return false
	}

	if !state.SubjectID.IsNull() && !state.SubjectID.IsUnknown() {
		return rel.SubjectId != nil && *rel.SubjectId == state.SubjectID.ValueString()
	}

	if rel.SubjectSet != nil {
		return rel.SubjectSet.Namespace == state.SubjectSetNamespace.ValueString() &&
			rel.SubjectSet.Object == state.SubjectSetObject.ValueString() &&
			rel.SubjectSet.Relation == state.SubjectSetRelation.ValueString()
	}

	return false
}
