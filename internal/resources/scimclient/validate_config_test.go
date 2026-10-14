package scimclient

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validateConfig(t *testing.T, model SCIMClientResourceModel) resource.ValidateConfigResponse {
	t.Helper()
	ctx := context.Background()

	r := &SCIMClientResource{}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	// tfsdk.Config has no Set, so the value is encoded through a State with
	// the same schema.
	state := tfsdk.State{Schema: schemaResp.Schema}
	require.False(t, state.Set(ctx, model).HasError())
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: state.Raw}

	var resp resource.ValidateConfigResponse
	r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: config}, &resp)
	return resp
}

func validModel() SCIMClientResourceModel {
	return SCIMClientResourceModel{
		ID:                                 types.StringNull(),
		ProjectID:                          types.StringNull(),
		OrganizationID:                     types.StringValue("6a3c1e8a-3e6b-4f3c-9a4c-2b8b1d5f7e10"),
		ClientID:                           types.StringValue("okta"),
		Label:                              types.StringValue("Okta"),
		MapperURL:                          types.StringValue("base64://abc"),
		AuthorizationHeaderSecret:          types.StringValue("s3cret"),
		AuthorizationHeaderSecretWO:        types.StringNull(),
		AuthorizationHeaderSecretWOVersion: types.StringNull(),
		State:                              types.StringNull(),
	}
}

// The SCIM server rejects every request for a client without a secret, so
// the configuration must supply one from either attribute.
func TestValidateConfig_RequiresOneSecret(t *testing.T) {
	model := validModel()
	model.AuthorizationHeaderSecret = types.StringNull()

	resp := validateConfig(t, model)
	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics.Errors()[0].Detail(), "authorization_header_secret_wo")
}

func TestValidateConfig_AcceptsEitherSecret(t *testing.T) {
	stateful := validModel()
	assert.False(t, validateConfig(t, stateful).Diagnostics.HasError())

	writeOnly := validModel()
	writeOnly.AuthorizationHeaderSecret = types.StringNull()
	writeOnly.AuthorizationHeaderSecretWO = types.StringValue("s3cret")
	writeOnly.AuthorizationHeaderSecretWOVersion = types.StringValue("1")
	assert.False(t, validateConfig(t, writeOnly).Diagnostics.HasError())
}

// A secret sourced from a data source or an ephemeral resource is unknown
// at validation time and must not fail the plan.
func TestValidateConfig_DefersUnknownSecret(t *testing.T) {
	model := validModel()
	model.AuthorizationHeaderSecret = types.StringNull()
	model.AuthorizationHeaderSecretWO = types.StringUnknown()

	assert.False(t, validateConfig(t, model).Diagnostics.HasError())
}
