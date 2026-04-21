package samlprovider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func tfStringValue(v types.String) tftypes.Value {
	if v.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}
	return tftypes.NewValue(tftypes.String, v.ValueString())
}

func buildTestConfig(t *testing.T, model SAMLProviderResourceModel) resource.ValidateConfigRequest {
	t.Helper()

	r := &SAMLProviderResource{}
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	vals := map[string]tftypes.Value{
		"id":                           tftypes.NewValue(tftypes.String, nil),
		"project_id":                   tftypes.NewValue(tftypes.String, nil),
		"provider_id":                  tfStringValue(model.ProviderID),
		"label":                        tfStringValue(model.Label),
		"mapper_url":                   tfStringValue(model.MapperURL),
		"raw_idp_metadata_xml":         tfStringValue(model.RawIDPMetadataXML),
		"organization_id":              tfStringValue(model.OrganizationID),
		"audience_override_base_url":   tfStringValue(model.AudienceOverrideBaseURL),
		"proxy_saml_audience_override": tfStringValue(model.ProxySAMLAudienceOverride),
	}

	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":                           tftypes.String,
			"project_id":                   tftypes.String,
			"provider_id":                  tftypes.String,
			"label":                        tftypes.String,
			"mapper_url":                   tftypes.String,
			"raw_idp_metadata_xml":         tftypes.String,
			"organization_id":              tftypes.String,
			"audience_override_base_url":   tftypes.String,
			"proxy_saml_audience_override": tftypes.String,
		},
	}

	return resource.ValidateConfigRequest{
		Config: tfsdk.Config{
			Raw:    tftypes.NewValue(objType, vals),
			Schema: schemaResp.Schema,
		},
	}
}

func TestValidateConfig_EmptyProviderID(t *testing.T) {
	model := SAMLProviderResourceModel{
		ProviderID:        types.StringValue(""),
		RawIDPMetadataXML: types.StringValue("base64://abc"),
	}
	req := buildTestConfig(t, model)

	r := &SAMLProviderResource{}
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(context.Background(), req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected validation error for empty provider_id, got none")
	}
}

func TestValidateConfig_EmptyMetadata(t *testing.T) {
	model := SAMLProviderResourceModel{
		ProviderID:        types.StringValue("saml"),
		RawIDPMetadataXML: types.StringValue(""),
	}
	req := buildTestConfig(t, model)

	r := &SAMLProviderResource{}
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(context.Background(), req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected validation error for empty raw_idp_metadata_xml, got none")
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	model := SAMLProviderResourceModel{
		ProviderID:        types.StringValue("saml"),
		RawIDPMetadataXML: types.StringValue("base64://abc"),
	}
	req := buildTestConfig(t, model)

	r := &SAMLProviderResource{}
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected validation error: %v", resp.Diagnostics)
	}
}
