package projectconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sessionWhoamiRulesPath = "/services/identity/config/session/whoami/rules"

// sessionWhoamiRules returns a known list. With no rules it is the empty
// list, not null: ListValueFrom turns a nil slice into a null list.
func sessionWhoamiRules(t *testing.T, rules ...SessionWhoamiRuleModel) types.List {
	t.Helper()
	if rules == nil {
		rules = []SessionWhoamiRuleModel{}
	}
	list, diags := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: sessionWhoamiRuleAttrTypes}, rules)
	require.False(t, diags.HasError(), diags)
	return list
}

// readSessionWhoamiRules runs the read path over a project whose
// session.whoami holds the given value. A nil value leaves the rules key out,
// as the API does after an empty list was applied.
func readSessionWhoamiRules(state *ProjectConfigResourceModel, stored interface{}) {
	whoami := map[string]interface{}{"required_aal": "highest_available"}
	if stored != nil {
		whoami["rules"] = stored
	}
	project := identityProject(map[string]interface{}{"session": map[string]interface{}{"whoami": whoami}})
	(&ProjectConfigResource{}).readProjectConfig(context.Background(), project, state)
}

// The patch sends the configured rules in order, and the API stores them as
// sent, so reading the stored value back must reproduce the plan exactly.
func TestSessionWhoamiRules_RoundTrip(t *testing.T) {
	ctx := context.Background()
	rules := sessionWhoamiRules(t,
		SessionWhoamiRuleModel{ID: types.StringValue("vpn"), Condition: types.StringValue("'ory:vpn' in request.ip_labels"), Action: types.StringValue("require_aal2")},
		SessionWhoamiRuleModel{ID: types.StringValue("botnet"), Condition: types.StringValue("'ory:botnet_cc' in request.ip_labels"), Action: types.StringValue("revoke")},
	)

	patch := findPatch((&ProjectConfigResource{}).buildPatches(ctx, &ProjectConfigResourceModel{SessionWhoamiRules: rules}), sessionWhoamiRulesPath)
	require.NotNil(t, patch)
	assert.Equal(t, "add", patch.Op)
	assert.Equal(t, []map[string]interface{}{
		{"id": "vpn", "condition": "'ory:vpn' in request.ip_labels", "action": "require_aal2"},
		{"id": "botnet", "condition": "'ory:botnet_cc' in request.ip_labels", "action": "revoke"},
	}, patch.Value)

	raw, err := json.Marshal(patch.Value)
	require.NoError(t, err)
	var stored interface{}
	require.NoError(t, json.Unmarshal(raw, &stored))

	state := &ProjectConfigResourceModel{SessionWhoamiRules: sessionWhoamiRules(t)}
	readSessionWhoamiRules(state, stored)
	assert.Equal(t, rules, state.SessionWhoamiRules)
}

// An empty list removes every rule. The API then omits the key, which must
// read back as the configured empty list so the next plan is empty.
func TestSessionWhoamiRules_EmptyList(t *testing.T) {
	empty := sessionWhoamiRules(t)

	patch := findPatch((&ProjectConfigResource{}).buildPatches(context.Background(), &ProjectConfigResourceModel{SessionWhoamiRules: empty}), sessionWhoamiRulesPath)
	require.NotNil(t, patch)
	assert.Equal(t, "add", patch.Op)
	assert.Equal(t, []map[string]interface{}{}, patch.Value)

	state := &ProjectConfigResourceModel{SessionWhoamiRules: empty}
	readSessionWhoamiRules(state, nil)
	assert.Equal(t, empty, state.SessionWhoamiRules)
}

// Rules removed outside Terraform read as an empty list, so plan shows them
// being added again.
func TestSessionWhoamiRules_RemovedOutOfBand(t *testing.T) {
	state := &ProjectConfigResourceModel{SessionWhoamiRules: sessionWhoamiRules(t,
		SessionWhoamiRuleModel{ID: types.StringValue("vpn"), Condition: types.StringValue("true"), Action: types.StringValue("revoke")},
	)}
	readSessionWhoamiRules(state, nil)
	assert.Equal(t, sessionWhoamiRules(t), state.SessionWhoamiRules)
}

// An unconfigured attribute sends nothing and is not refreshed, so rules
// managed in the Ory Console stay out of the plan.
func TestSessionWhoamiRules_Untracked(t *testing.T) {
	nullRules := types.ListNull(types.ObjectType{AttrTypes: sessionWhoamiRuleAttrTypes})
	plan := &ProjectConfigResourceModel{SessionWhoamiRules: nullRules}
	assert.Nil(t, findPatch((&ProjectConfigResource{}).buildPatches(context.Background(), plan), sessionWhoamiRulesPath))

	readSessionWhoamiRules(plan, []interface{}{map[string]interface{}{"id": "vpn", "condition": "true", "action": "revoke"}})
	assert.True(t, plan.SessionWhoamiRules.IsNull())
}
