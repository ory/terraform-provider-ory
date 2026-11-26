package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tokenizerTemplates(t *testing.T, templates map[string]SessionTokenizerTemplateModel) types.Map {
	t.Helper()
	m, diags := types.MapValueFrom(context.Background(), types.ObjectType{AttrTypes: tokenizerTemplateAttrTypes}, templates)
	require.False(t, diags.HasError(), diags)
	return m
}

// The plan carries jwks_url_wo as null, as Terraform sends it, so the key
// only reaches the patch from the configuration.
func TestAddWriteOnlyTokenizerKeys(t *testing.T) {
	ctx := context.Background()
	wo := SessionTokenizerTemplateModel{TTL: types.StringValue("5m"), JWKSURLWOVersion: types.StringValue("1")}
	plain := SessionTokenizerTemplateModel{TTL: types.StringValue("1m"), JWKSURL: types.StringValue("base64://cGxhaW4=")}
	plan := &ProjectConfigResourceModel{SessionTokenizerTemplates: tokenizerTemplates(t, map[string]SessionTokenizerTemplateModel{"wo": wo, "plain": plain})}

	schemaResp := &resource.SchemaResponse{}
	(&ProjectConfigResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)
	state := projectConfigState(t, schemaResp.Schema)
	wo.JWKSURLWO = types.StringValue("base64://c2VjcmV0")
	configured := tokenizerTemplates(t, map[string]SessionTokenizerTemplateModel{"wo": wo, "plain": plain})
	require.False(t, state.SetAttribute(ctx, path.Root("session_tokenizer_templates"), configured).HasError())
	config := tfsdk.Config(state)

	patches := (&ProjectConfigResource{}).buildPatches(ctx, plan)
	var diags diag.Diagnostics
	addWriteOnlyTokenizerKeys(ctx, config, patches, &diags)
	require.False(t, diags.HasError(), diags)

	patch := findPatch(patches, tokenizerTemplatesPath)
	require.NotNil(t, patch)
	assert.Equal(t, map[string]interface{}{
		"wo":    map[string]interface{}{"ttl": "5m", "jwks_url": "base64://c2VjcmV0"},
		"plain": map[string]interface{}{"ttl": "1m", "jwks_url": "base64://cGxhaW4="},
	}, patch.Value)
}

// The API returns the stored jwks_url in full. A template that uses
// jwks_url_wo must keep both key attributes null in state, and keep the
// version trigger.
func TestReadProjectConfig_TokenizerWriteOnlyKeyStaysOutOfState(t *testing.T) {
	state := &ProjectConfigResourceModel{SessionTokenizerTemplates: tokenizerTemplates(t, map[string]SessionTokenizerTemplateModel{
		"wo":    {TTL: types.StringValue("5m"), JWKSURLWOVersion: types.StringValue("1")},
		"plain": {TTL: types.StringValue("1m"), JWKSURL: types.StringValue("base64://cGxhaW4=")},
	})}
	project := &ory.Project{Services: ory.ProjectServices{Identity: &ory.ProjectServiceIdentity{Config: map[string]interface{}{
		"session": map[string]interface{}{"whoami": map[string]interface{}{"tokenizer": map[string]interface{}{"templates": map[string]interface{}{
			"wo":    map[string]interface{}{"ttl": "5m0s", "jwks_url": "base64://c2VjcmV0"},
			"plain": map[string]interface{}{"ttl": "1m0s", "jwks_url": "base64://cGxhaW4="},
		}}}},
	}}}}

	(&ProjectConfigResource{}).readProjectConfig(context.Background(), project, state)

	var got map[string]SessionTokenizerTemplateModel
	require.False(t, state.SessionTokenizerTemplates.ElementsAs(context.Background(), &got, false).HasError())
	assert.Equal(t, map[string]SessionTokenizerTemplateModel{
		"wo": {
			TTL: types.StringValue("5m"), JWKSURL: types.StringNull(), JWKSURLWO: types.StringNull(),
			JWKSURLWOVersion: types.StringValue("1"), ClaimsMapperURL: types.StringNull(), SubjectSource: types.StringNull(),
		},
		"plain": {
			TTL: types.StringValue("1m"), JWKSURL: types.StringValue("base64://cGxhaW4="), JWKSURLWO: types.StringNull(),
			JWKSURLWOVersion: types.StringNull(), ClaimsMapperURL: types.StringNull(), SubjectSource: types.StringNull(),
		},
	}, got)
}

// A key without the base64:// scheme must not be echoed into terminal or CI
// output.
func TestBase64JWKSValidator_DoesNotEchoTheValue(t *testing.T) {
	for value, wantError := range map[string]bool{"base64://e30=": false, `{"keys":[{"d":"secret-scalar"}]}`: true} {
		resp := &validator.StringResponse{}
		base64JWKSValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path:        path.Root("session_tokenizer_templates").AtMapKey("api").AtName("jwks_url_wo"),
			ConfigValue: types.StringValue(value),
		}, resp)
		require.Equal(t, wantError, resp.Diagnostics.HasError(), value)
		for _, d := range resp.Diagnostics {
			assert.Contains(t, d.Detail(), "must start with 'base64://'")
			assert.NotContains(t, d.Detail(), "secret-scalar")
		}
	}
}
