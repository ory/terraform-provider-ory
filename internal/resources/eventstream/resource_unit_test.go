package eventstream

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The masked endpoint is the live API response for the configured endpoint.
const (
	configuredEndpoint = "https://user:pa55@example.com/hook?b=2&a=secret&a=two" //nolint:gosec // G101 false positive: test fixture, not a real credential
	reportedEndpoint   = "https://xxxxx@example.com/hook?a=xxxxx&a=xxxxx&b=xxxxx"
)

func TestMaskEndpoint(t *testing.T) {
	assert.Equal(t, reportedEndpoint, maskEndpoint(configuredEndpoint))
	assert.Equal(t, "https://example.com/hook", maskEndpoint("https://example.com/hook"))
	assert.Equal(t, "https://xxxxx@example.com/", maskEndpoint("https://user@example.com/"))
}

func TestReadEndpoint(t *testing.T) {
	configured := types.StringValue(configuredEndpoint)
	reported := reportedEndpoint
	assert.Equal(t, configured, readEndpoint(configured, &reported), "a matching masked endpoint keeps the configured one")

	changed := "https://xxxxx@example.org/hook?a=xxxxx&a=xxxxx&b=xxxxx"
	assert.Equal(t, types.StringValue(changed), readEndpoint(configured, &changed), "a changed host surfaces as drift")

	assert.Equal(t, types.StringValue(reported), readEndpoint(types.StringNull(), &reported), "an import takes the masked endpoint")
	assert.True(t, readEndpoint(configured, nil).IsNull(), "an SNS stream has no endpoint")
}

func TestReadStream(t *testing.T) {
	t.Run("sns", func(t *testing.T) {
		m := EventStreamResourceModel{HTTPSAuthSecret: types.StringValue("stale")}
		readStream(&ory.EventStream{
			Type:     ory.PtrString("sns"),
			TopicArn: ory.PtrString("arn:aws:sns:us-east-1:123456789012:ory-events"),
			RoleArn:  ory.PtrString("arn:aws:iam::123456789012:role/ory-events"),
			Status:   ory.PtrString("active"),
		}, &m)

		assert.Equal(t, "arn:aws:sns:us-east-1:123456789012:ory-events", m.TopicArn.ValueString())
		assert.Equal(t, "none", m.HTTPSAuthType.ValueString(), "an SNS stream reports no scheme")
		assert.True(t, m.HTTPSEndpoint.IsNull())
		assert.True(t, m.HTTPSAuthHeaderName.IsNull())
		assert.True(t, m.HTTPSAuthSecret.IsNull(), "a stream without a stored secret clears the state secret")
		assert.Equal(t, "active", m.Status.ValueString())
	})

	t.Run("https", func(t *testing.T) {
		m := EventStreamResourceModel{
			HTTPSEndpoint:   types.StringValue(configuredEndpoint),
			HTTPSAuthSecret: types.StringValue("s3cret"),
		}
		readStream(&ory.EventStream{
			Type:                ory.PtrString("https"),
			HttpsEndpoint:       *ory.NewNullableString(ory.PtrString(reportedEndpoint)),
			HttpsAuthType:       ory.PtrString("api_key"),
			HttpsAuthHeaderName: ory.PtrString("X-Api-Key"),
			HttpsAuthSecret:     ory.PtrString(redactionMask),
			Status:              ory.PtrString("paused"),
		}, &m)

		assert.True(t, m.TopicArn.IsNull())
		assert.True(t, m.RoleArn.IsNull())
		assert.Equal(t, configuredEndpoint, m.HTTPSEndpoint.ValueString())
		assert.Equal(t, "api_key", m.HTTPSAuthType.ValueString())
		assert.Equal(t, "X-Api-Key", m.HTTPSAuthHeaderName.ValueString())
		assert.Equal(t, "s3cret", m.HTTPSAuthSecret.ValueString(), "the masked secret never replaces the configured one")
		assert.Equal(t, "paused", m.Status.ValueString())
	})
}

func validateConfig(t *testing.T, model EventStreamResourceModel) resource.ValidateConfigResponse {
	t.Helper()
	ctx := context.Background()

	r := &EventStreamResource{}
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

func TestValidateConfig(t *testing.T) {
	sns := func(m *EventStreamResourceModel) {
		m.Type = types.StringValue("sns")
		m.TopicArn = types.StringValue("arn:aws:sns:us-east-1:123456789012:ory-events")
		m.RoleArn = types.StringValue("arn:aws:iam::123456789012:role/ory-events")
	}
	https := func(m *EventStreamResourceModel) {
		m.Type = types.StringValue("https")
		m.HTTPSEndpoint = types.StringValue("https://events.example.com/ory")
	}
	bearer := func(m *EventStreamResourceModel) {
		https(m)
		m.HTTPSAuthType = types.StringValue("bearer")
		m.HTTPSAuthSecret = types.StringValue("s3cret")
	}
	apiKey := func(m *EventStreamResourceModel) {
		https(m)
		m.HTTPSAuthType = types.StringValue("api_key")
		m.HTTPSAuthHeaderName = types.StringValue("X-Api-Key")
		m.HTTPSAuthSecret = types.StringValue("s3cret")
	}

	tests := []struct {
		name    string
		build   func(m *EventStreamResourceModel)
		wantErr string // a substring of the only error, empty when the config is valid
	}{
		{"sns", sns, ""},
		{"sns without role_arn", func(m *EventStreamResourceModel) { sns(m); m.RoleArn = types.StringNull() }, "role_arn must be set"},
		{"sns with endpoint", func(m *EventStreamResourceModel) {
			sns(m)
			m.HTTPSEndpoint = types.StringValue("https://events.example.com/ory")
		}, "https_endpoint cannot be set"},
		{"sns with bearer", func(m *EventStreamResourceModel) {
			sns(m)
			m.HTTPSAuthType = types.StringValue("bearer")
		}, `https_auth_type must be "none"`},
		{"https without auth", https, ""},
		{"https without endpoint", func(m *EventStreamResourceModel) { https(m); m.HTTPSEndpoint = types.StringNull() }, "https_endpoint must be set"},
		{"https with topic_arn", func(m *EventStreamResourceModel) {
			https(m)
			m.TopicArn = types.StringValue("arn:aws:sns:us-east-1:123456789012:ory-events")
		}, "topic_arn cannot be set"},
		{"https with a secret but no scheme", func(m *EventStreamResourceModel) {
			https(m)
			m.HTTPSAuthSecret = types.StringValue("s3cret")
		}, "https_auth_secret cannot be set"},
		{"bearer", bearer, ""},
		{"bearer with a header", func(m *EventStreamResourceModel) {
			bearer(m)
			m.HTTPSAuthHeaderName = types.StringValue("X-Api-Key")
		}, "https_auth_header_name cannot be set"},
		{"bearer without a secret", func(m *EventStreamResourceModel) {
			bearer(m)
			m.HTTPSAuthSecret = types.StringNull()
		}, "https_auth_secret or https_auth_secret_wo must be set"},
		{"bearer with a write-only secret", func(m *EventStreamResourceModel) {
			bearer(m)
			m.HTTPSAuthSecret = types.StringNull()
			m.HTTPSAuthSecretWO = types.StringValue("s3cret")
		}, ""},
		{"api_key", apiKey, ""},
		{"api_key without a header", func(m *EventStreamResourceModel) {
			apiKey(m)
			m.HTTPSAuthHeaderName = types.StringNull()
		}, "https_auth_header_name must be set"},
		// A value sourced from another resource is unknown at validation
		// time and must not fail the plan.
		{"unknown values", func(m *EventStreamResourceModel) {
			apiKey(m)
			m.HTTPSEndpoint = types.StringUnknown()
			m.HTTPSAuthSecret = types.StringUnknown()
		}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var m EventStreamResourceModel
			tc.build(&m)
			errs := validateConfig(t, m).Diagnostics.Errors()
			if tc.wantErr == "" {
				assert.Empty(t, errs)
				return
			}
			require.Len(t, errs, 1)
			assert.Contains(t, errs[0].Detail(), tc.wantErr)
		})
	}
}
