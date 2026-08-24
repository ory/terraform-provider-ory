package socialprovider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseProviderPlan returns the minimum plan every buildProviderConfig test needs.
func baseProviderPlan() *SocialProviderResourceModel {
	return &SocialProviderResourceModel{
		ProviderID:   types.StringValue("netid"),
		ProviderType: types.StringValue("netid"),
		ClientID:     types.StringValue("test-client-id"),
	}
}

// Create and Update replace the whole provider object, so every attribute the
// user configured must appear in the payload. A missing net_id_token_origin_header
// blanks the field server-side on each apply and breaks NetID FedCM sign-in.
// Regression test for https://github.com/ory/terraform-provider-ory/issues/329
func TestBuildProviderConfig_NetIDTokenOriginHeader(t *testing.T) {
	tests := []struct {
		name        string
		originValue types.String
		wantPresent bool
		wantValue   string
	}{
		{
			name:        "set",
			originValue: types.StringValue("https://www.example.com"),
			wantPresent: true,
			wantValue:   "https://www.example.com",
		},
		{
			name:        "null",
			originValue: types.StringNull(),
			wantPresent: false,
		},
		{
			name:        "unknown",
			originValue: types.StringUnknown(),
			wantPresent: false,
		},
		{
			name:        "empty string",
			originValue: types.StringValue(""),
			wantPresent: false,
		},
	}

	r := &SocialProviderResource{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := baseProviderPlan()
			plan.NetIDTokenOriginHeader = tt.originValue

			config := r.buildProviderConfig(ctx, plan,
				plan.ClientID, types.StringValue("test-client-secret"), types.StringNull())

			got, ok := config["net_id_token_origin_header"]
			if !tt.wantPresent {
				assert.False(t, ok, "net_id_token_origin_header must be absent from the payload")
				return
			}
			require.True(t, ok, "net_id_token_origin_header must be sent to the API")
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

// organization_id links an OIDC provider to a B2B SSO organization. Create and
// Update replace the whole provider object, so the attribute must survive in the
// payload when set, and must be absent when the user removes it. The API clears
// the link whenever the key is missing from the replacement object.
// Regression test for https://github.com/ory/terraform-provider-ory/issues/339
func TestBuildProviderConfig_OrganizationID(t *testing.T) {
	tests := []struct {
		name        string
		orgID       types.String
		wantPresent bool
		wantValue   string
	}{
		{
			name:        "set",
			orgID:       types.StringValue("64f12157-10fa-4a9b-899d-997cbc99fce3"),
			wantPresent: true,
			wantValue:   "64f12157-10fa-4a9b-899d-997cbc99fce3",
		},
		{
			name:        "null",
			orgID:       types.StringNull(),
			wantPresent: false,
		},
		{
			name:        "unknown",
			orgID:       types.StringUnknown(),
			wantPresent: false,
		},
		{
			name:        "empty string",
			orgID:       types.StringValue(""),
			wantPresent: false,
		},
	}

	r := &SocialProviderResource{}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := baseProviderPlan()
			plan.OrganizationID = tt.orgID

			config := r.buildProviderConfig(ctx, plan,
				plan.ClientID, types.StringValue("test-client-secret"), types.StringNull())

			got, ok := config["organization_id"]
			if !tt.wantPresent {
				assert.False(t, ok, "organization_id must be absent from the payload")
				return
			}
			require.True(t, ok, "organization_id must be sent to the API")
			assert.Equal(t, tt.wantValue, got)
		})
	}
}

// A NetID provider carries both the FedCM config URL and the origin header, so
// the payload must keep them side by side.
func TestBuildProviderConfig_NetIDFedcmPair(t *testing.T) {
	r := &SocialProviderResource{}

	plan := baseProviderPlan()
	plan.FedcmConfigURL = types.StringValue("https://broker.netid.de/fedcm.json")
	plan.NetIDTokenOriginHeader = types.StringValue("https://www.example.com")

	config := r.buildProviderConfig(context.Background(), plan,
		plan.ClientID, types.StringValue("test-client-secret"), types.StringNull())

	assert.Equal(t, "https://broker.netid.de/fedcm.json", config["fedcm_config_url"])
	assert.Equal(t, "https://www.example.com", config["net_id_token_origin_header"])
}
