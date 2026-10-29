package accountexperiencetranslation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ resource.Resource                = &AccountExperienceTranslationResource{}
	_ resource.ResourceWithConfigure   = &AccountExperienceTranslationResource{}
	_ resource.ResourceWithImportState = &AccountExperienceTranslationResource{}
)

// translationsPath is the JSON Pointer of the locale map in the project
// document. Each member maps a locale to the URL of its stored translations
// document. The API renders the map as {} when no locale has custom
// translations, so an "add" of a single locale also works on a fresh project.
const translationsPath = "/services/account_experience/config/translations"

// localeRegex is the pattern the API enforces on a locale. The API rejects any
// other value with HTTP 400. The pattern also keeps "/" and "~" out of the JSON
// Pointer that addresses the locale.
var localeRegex = regexp.MustCompile(`(?i)^[a-z]{2,3}(-[a-z0-9]{2,8})*$`)

// fetchStoredDocument downloads a stored translations document. It is a
// package-level variable so unit tests can replace it. client.FetchSafeHTTPS
// refuses non-HTTPS URLs and private hosts, and caps the body at 1 MiB.
var fetchStoredDocument = func(ctx context.Context, rawURL string) ([]byte, error) {
	return client.FetchSafeHTTPS(ctx, "account experience translations", rawURL)
}

func NewResource() resource.Resource {
	return &AccountExperienceTranslationResource{}
}

type AccountExperienceTranslationResource struct {
	client *client.OryClient
}

type AccountExperienceTranslationResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Locale    types.String `tfsdk:"locale"`
	Messages  types.Map    `tfsdk:"messages"`
}

func (r *AccountExperienceTranslationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_experience_translation"
}

func (r *AccountExperienceTranslationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the custom text of the Ory Account Experience, the hosted login UI, for one locale.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource ID. The same value as locale.",
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
			"locale": schema.StringAttribute{
				Description: "Language tag of the locale the text is for, for example en or de. The hosted login uses the text only for a locale it serves: one in account_experience_enabled_locales, or account_experience_default_locale, on ory_project_config. Changing it forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(localeRegex, "must be a language tag such as en, de, or pt-BR"),
				},
			},
			"messages": schema.MapAttribute{
				Description: "Map of message keys, for example login.title, to the text the hosted login shows for this locale. A key that is not in the map keeps the built-in text. Must contain at least one entry.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *AccountExperienceTranslationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// localePath is the JSON Pointer of one locale in the translations map.
func localePath(locale string) string {
	return translationsPath + "/" + locale
}

// encodeMessages renders messages as the document the API stores. json.Marshal
// sorts map keys, so equal maps always give the same bytes, and the API names
// the stored file after the SHA-512 of exactly these bytes.
func encodeMessages(messages map[string]string) []byte {
	// Marshal cannot fail for a map of strings.
	doc, _ := json.Marshal(messages)
	return doc
}

// write uploads the plan's messages as the translations document of its
// locale. An "add" on an existing member replaces it, as RFC 6902 specifies, so
// the same patch creates the locale, updates it, and takes over a locale that
// was set in the Console.
func (r *AccountExperienceTranslationResource) write(ctx context.Context, plan *AccountExperienceTranslationResourceModel, diags *diag.Diagnostics) {
	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), diags)
	var messages map[string]string
	diags.Append(plan.Messages.ElementsAs(ctx, &messages, false)...)
	if diags.HasError() {
		return
	}

	// The API accepts only padded standard base64 here.
	patch := []ory.JsonPatch{{
		Op:    "add",
		Path:  localePath(plan.Locale.ValueString()),
		Value: "base64://" + base64.StdEncoding.EncodeToString(encodeMessages(messages)),
	}}
	if _, err := r.client.PatchProject(ctx, projectID, patch); err != nil {
		diags.AddError("Error Writing Account Experience Translation", err.Error())
		return
	}

	plan.ID = plan.Locale
	plan.ProjectID = types.StringValue(projectID)
}

func (r *AccountExperienceTranslationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AccountExperienceTranslationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *AccountExperienceTranslationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AccountExperienceTranslationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	var current map[string]string
	if !state.Messages.IsNull() {
		resp.Diagnostics.Append(state.Messages.ElementsAs(ctx, &current, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Prefer the project the last PatchProject returned, as ory_email_template
	// does: a GET straight after a write can still report the previous
	// revision. The cache lives only as long as one Terraform command, so the
	// next plan reads the project again and sees changes made in the Console.
	project := r.client.GetCachedProject(projectID)
	if project == nil {
		var err error
		if project, err = r.client.GetProject(ctx, projectID); err != nil {
			resp.Diagnostics.AddError("Error Reading Account Experience Translation", err.Error())
			return
		}
	}

	storedURL := storedDocumentURL(project, state.Locale.ValueString())
	if storedURL == "" {
		// Removed in the Console, or by another configuration.
		resp.State.RemoveResource(ctx)
		return
	}
	messages, err := resolveMessages(ctx, storedURL, current)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Account Experience Translation",
			fmt.Sprintf("Failed to read the %q translations of project %s: %v", state.Locale.ValueString(), projectID, err))
		return
	}

	var diags diag.Diagnostics
	state.Messages, diags = types.MapValueFrom(ctx, types.StringType, messages)
	resp.Diagnostics.Append(diags...)
	state.ID = state.Locale
	state.ProjectID = types.StringValue(projectID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// storedDocumentURL returns the URL of the document the project stores for
// locale, or "" when the locale has no custom translations.
func storedDocumentURL(project *ory.Project, locale string) string {
	ax := project.Services.AccountExperience
	if ax == nil {
		return ""
	}
	translations, _ := ax.Config["translations"].(map[string]interface{})
	storedURL, _ := translations[locale].(string)
	return storedURL
}

// resolveMessages returns the messages of the document stored at storedURL.
// The API names the stored file after the SHA-512 of the document, so when the
// name matches the encoding of current, current is returned without a
// download. Otherwise the document changed outside Terraform, or the resource
// is new to state after an import, and the document is downloaded and parsed.
func resolveMessages(ctx context.Context, storedURL string, current map[string]string) (map[string]string, error) {
	if current != nil && helpers.URLHashMatchesContent(storedURL, encodeMessages(current)) {
		return current, nil
	}
	doc, err := fetchStoredDocument(ctx, storedURL)
	if err != nil {
		return nil, err
	}
	var messages map[string]string
	if err := json.Unmarshal(doc, &messages); err != nil {
		return nil, fmt.Errorf("parsing the translations document at %s: %w", storedURL, err)
	}
	return messages, nil
}

func (r *AccountExperienceTranslationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AccountExperienceTranslationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *AccountExperienceTranslationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccountExperienceTranslationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	patch := []ory.JsonPatch{{Op: "remove", Path: localePath(state.Locale.ValueString())}}
	if _, err := r.client.PatchProject(ctx, projectID, patch); err != nil && !isAlreadyRemoved(err) {
		resp.Diagnostics.AddError("Error Deleting Account Experience Translation", err.Error())
	}
}

// isAlreadyRemoved reports whether a remove failed only because the locale is
// already gone, for example after a removal in the Console. The API answers
// HTTP 400 with "unable to remove nonexistent key" in the body, and
// PatchProject puts the body into the error.
func isAlreadyRemoved(err error) bool {
	return strings.Contains(err.Error(), "nonexistent key")
}

// ImportState accepts <project-id>/<locale>, or <locale> alone to use the
// provider's project_id.
func (r *AccountExperienceTranslationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, locale, found := strings.Cut(req.ID, "/")
	if !found {
		projectID, locale = "", req.ID
	}
	if locale == "" {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Expected <project-id>/<locale> or <locale>, got %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), locale)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("locale"), locale)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}
