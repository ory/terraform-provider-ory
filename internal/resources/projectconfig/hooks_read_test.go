package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
	"github.com/stretchr/testify/assert"
)

// readStateFromHooks runs the resource read path over a project whose
// settings.after.profile and login.after.password hook arrays hold the given
// entries. The OIDC login flow mirrors the password flow, so a caller that
// only cares about the password flow still exercises both.
func readStateFromHooks(state *ProjectConfigResourceModel, loginPasswordHooks, settingsProfileHooks []interface{}) {
	project := &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{
				Config: map[string]interface{}{
					"selfservice": map[string]interface{}{
						"flows": map[string]interface{}{
							"login": map[string]interface{}{
								"after": map[string]interface{}{
									"password": map[string]interface{}{"hooks": loginPasswordHooks},
									"oidc":     map[string]interface{}{"hooks": loginPasswordHooks},
								},
							},
							"settings": map[string]interface{}{
								"after": map[string]interface{}{
									"profile": map[string]interface{}{"hooks": settingsProfileHooks},
								},
							},
						},
					},
				},
			},
		},
	}
	(&ProjectConfigResource{}).readProjectConfig(context.Background(), project, state)
}

func TestReadProjectConfig_EmailVerificationHooksPresent(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress:    types.BoolValue(false),
		SelfserviceFlowsLoginAfterOIDCHookRequireVerifiedAddress:        types.BoolValue(false),
		SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress:        types.BoolValue(false),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses: types.BoolValue(false),
	}

	readStateFromHooks(state,
		[]interface{}{map[string]interface{}{"hook": "require_verified_address"}},
		[]interface{}{
			map[string]interface{}{"hook": "verify_new_address"},
			map[string]interface{}{"hook": "notify_previous_addresses"},
		},
	)

	assert.True(t, state.SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress.ValueBool())
	assert.True(t, state.SelfserviceFlowsLoginAfterOIDCHookRequireVerifiedAddress.ValueBool())
	assert.True(t, state.SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress.ValueBool())
	assert.True(t, state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses.ValueBool())
}

// A hook removed out of band shows up as false rather than staying true.
func TestReadProjectConfig_EmailVerificationHooksAbsent(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress:    types.BoolValue(true),
		SelfserviceFlowsLoginAfterOIDCHookRequireVerifiedAddress:        types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress:        types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses: types.BoolValue(true),
	}

	readStateFromHooks(state, []interface{}{}, []interface{}{map[string]interface{}{"hook": "organization"}})

	assert.False(t, state.SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress.ValueBool())
	assert.False(t, state.SelfserviceFlowsLoginAfterOIDCHookRequireVerifiedAddress.ValueBool())
	assert.False(t, state.SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress.ValueBool())
	assert.False(t, state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses.ValueBool())
}

// An attribute the practitioner never set stays null, so an untracked hook
// does not start showing up as drift.
func TestReadProjectConfig_UntrackedEmailVerificationHooksStayNull(t *testing.T) {
	state := &ProjectConfigResourceModel{}

	readStateFromHooks(state,
		[]interface{}{map[string]interface{}{"hook": "require_verified_address"}},
		[]interface{}{
			map[string]interface{}{"hook": "verify_new_address"},
			map[string]interface{}{
				"hook":   "notify_previous_addresses",
				"config": map[string]interface{}{"recipients": "all"},
			},
		},
	)

	assert.True(t, state.SelfserviceFlowsLoginAfterPasswordHookRequireVerifiedAddress.IsNull())
	assert.True(t, state.SelfserviceFlowsLoginAfterOIDCHookRequireVerifiedAddress.IsNull())
	assert.True(t, state.SelfserviceFlowsSettingsAfterProfileHookVerifyNewAddress.IsNull())
	assert.True(t, state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses.IsNull())
	assert.True(t, state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients.IsNull())
}

func TestReadProjectConfig_NotifyPreviousAddressesRecipientsRefreshed(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses:           types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients: types.StringValue("removed"),
	}

	readStateFromHooks(state, nil, []interface{}{
		map[string]interface{}{
			"hook":   "notify_previous_addresses",
			"config": map[string]interface{}{"recipients": "all_verified"},
		},
	})

	assert.Equal(t, "all_verified", state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients.ValueString())
}

// Ory drops the config key when the hook runs with its default scope, which
// must read back as that default instead of an empty string.
func TestReadProjectConfig_NotifyPreviousAddressesMissingConfigReadsAsDefault(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses:           types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients: types.StringValue("all"),
	}

	readStateFromHooks(state, nil, []interface{}{
		map[string]interface{}{"hook": "notify_previous_addresses"},
	})

	assert.Equal(t, notifyPreviousAddressesDefaultRecipients,
		state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients.ValueString())
}

// With the hook gone there is no scope to read, so the tracked scope is left
// as configured and the boolean alone reports the divergence.
func TestReadProjectConfig_NotifyPreviousAddressesRecipientsKeptWhenHookAbsent(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses:           types.BoolValue(true),
		SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients: types.StringValue("all"),
	}

	readStateFromHooks(state, nil, []interface{}{})

	assert.False(t, state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddresses.ValueBool())
	assert.Equal(t, "all", state.SelfserviceFlowsSettingsAfterProfileHookNotifyPreviousAddressesRecipients.ValueString())
}
