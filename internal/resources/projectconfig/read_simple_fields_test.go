package projectconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	ory "github.com/ory/client-go"
)

// identityProject builds a project whose identity service config is the given map.
func identityProject(config map[string]interface{}) *ory.Project {
	return &ory.Project{
		Services: ory.ProjectServices{
			Identity: &ory.ProjectServiceIdentity{Config: config},
		},
	}
}

// oidcConfig nests value under selfservice.methods.oidc.config using key, or
// produces an OIDC config with no keys at all when key is empty.
func oidcConfig(key string, value interface{}) map[string]interface{} {
	inner := map[string]interface{}{}
	if key != "" {
		inner[key] = value
	}
	return map[string]interface{}{
		"selfservice": map[string]interface{}{
			"methods": map[string]interface{}{
				"oidc": map[string]interface{}{
					"config": inner,
				},
			},
		},
	}
}

// TestReadSimpleFields_NullsTrackedFieldWhenAPIOmitsKey covers the drift a
// refresh used to hide: a value removed server-side left the old value in
// state, so terraform plan reported no changes and the removal was never
// reconciled.
func TestReadSimpleFields_NullsTrackedFieldWhenAPIOmitsKey(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceMethodsOIDCConfigBaseRedirectURI: types.StringValue("https://iam.example.com"),
	}

	readSimpleFields(context.Background(), identityProject(oidcConfig("", nil)), state)

	if !state.SelfserviceMethodsOIDCConfigBaseRedirectURI.IsNull() {
		t.Errorf("base_redirect_uri = %q, want null so the removal surfaces as drift",
			state.SelfserviceMethodsOIDCConfigBaseRedirectURI.ValueString())
	}
}

func TestReadSimpleFields_KeepsValueReturnedByAPI(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceMethodsOIDCConfigBaseRedirectURI: types.StringValue("https://old.example.com"),
	}

	readSimpleFields(context.Background(),
		identityProject(oidcConfig("base_redirect_uri", "https://iam.example.com")), state)

	if got := state.SelfserviceMethodsOIDCConfigBaseRedirectURI.ValueString(); got != "https://iam.example.com" {
		t.Errorf("base_redirect_uri = %q, want the value returned by the API", got)
	}
}

// Untracked attributes must stay null: the generated read only touches fields
// already present in state, so an unmanaged config key never enters state.
func TestReadSimpleFields_LeavesUntrackedFieldNull(t *testing.T) {
	state := &ProjectConfigResourceModel{}

	readSimpleFields(context.Background(),
		identityProject(oidcConfig("base_redirect_uri", "https://iam.example.com")), state)

	if !state.SelfserviceMethodsOIDCConfigBaseRedirectURI.IsNull() {
		t.Errorf("base_redirect_uri = %q, want null for an unmanaged attribute",
			state.SelfserviceMethodsOIDCConfigBaseRedirectURI.ValueString())
	}
}

// Secrets the API returns masked or not at all must keep their state value;
// nulling them on absence would trade missed drift for a perpetual diff.
func TestReadSimpleFields_PreservesSensitiveFieldWhenAPIOmitsKey(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceMethodsCaptchaConfigCFTurnstileSecret: types.StringValue("secret-value"),
	}

	readSimpleFields(context.Background(), identityProject(map[string]interface{}{}), state)

	if got := state.SelfserviceMethodsCaptchaConfigCFTurnstileSecret.ValueString(); got != "secret-value" {
		t.Errorf("cf_turnstile_secret = %q, want the state value preserved", got)
	}
}

// The list(string) branch reaches the preserve check through getNestedValue and
// a []interface{} cast rather than getNestedString, so a secret list needs its
// own proof that absence keeps the state value.
func TestReadSimpleFields_PreservesSensitiveListWhenAPIOmitsKey(t *testing.T) {
	secrets, diags := types.ListValueFrom(context.Background(), types.StringType,
		[]string{"cipher-secret-a", "cipher-secret-b"})
	if diags.HasError() {
		t.Fatalf("building the state list: %v", diags)
	}
	state := &ProjectConfigResourceModel{IdentitySecretsCipher: secrets}

	readSimpleFields(context.Background(), identityProject(map[string]interface{}{}), state)

	if !state.IdentitySecretsCipher.Equal(secrets) {
		t.Errorf("identity_secrets_cipher = %v, want the state value preserved",
			state.IdentitySecretsCipher)
	}
}

// Companion to the test above: without this, that one would also pass if the
// list branch never nulled anything, which would hide the drift bug for lists.
func TestReadSimpleFields_NullsNonSensitiveListWhenAPIOmitsKey(t *testing.T) {
	domains, diags := types.ListValueFrom(context.Background(), types.StringType,
		[]string{"example.com"})
	if diags.HasError() {
		t.Fatalf("building the state list: %v", diags)
	}
	state := &ProjectConfigResourceModel{SelfserviceMethodsCaptchaConfigAllowedDomains: domains}

	readSimpleFields(context.Background(), identityProject(map[string]interface{}{}), state)

	if !state.SelfserviceMethodsCaptchaConfigAllowedDomains.IsNull() {
		t.Errorf("allowed_domains = %v, want null so the removal surfaces as drift",
			state.SelfserviceMethodsCaptchaConfigAllowedDomains)
	}
}

// skip_empty_read attributes are ones the API does not report faithfully.
func TestReadSimpleFields_PreservesSkipEmptyReadFieldWhenAPIOmitsKey(t *testing.T) {
	state := &ProjectConfigResourceModel{
		AccountExperienceLocale: types.StringValue("en"),
	}

	project := &ory.Project{
		Services: ory.ProjectServices{
			AccountExperience: &ory.ProjectServiceAccountExperience{Config: map[string]interface{}{}},
		},
	}
	readSimpleFields(context.Background(), project, state)

	if got := state.AccountExperienceLocale.ValueString(); got != "en" {
		t.Errorf("default_locale = %q, want the state value preserved", got)
	}
}

// write_only attributes are omitted from the read tables entirely, so nulling
// on absence must never reach them — the API is not expected to return them.
func TestReadSimpleFields_PreservesWriteOnlyFieldWhenAPIOmitsKey(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SMTPConnectionURI: types.StringValue("smtps://user:pass@smtp.example.com:465"),
	}

	readSimpleFields(context.Background(), identityProject(map[string]interface{}{}), state)

	if got := state.SMTPConnectionURI.ValueString(); got != "smtps://user:pass@smtp.example.com:465" {
		t.Errorf("smtp_connection_uri = %q, want the state value preserved", got)
	}
}

func TestReadSimpleFields_NullsTrackedFieldsOfEveryType(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SessionLifespan:                    types.StringValue("24h"),
		SelfserviceMethodsPasswordEnabled:  types.BoolValue(true),
		SelfserviceMethodsTOTPConfigIssuer: types.StringValue("example.com"),
	}

	readSimpleFields(context.Background(), identityProject(map[string]interface{}{}), state)

	if !state.SessionLifespan.IsNull() {
		t.Errorf("session_lifespan = %q, want null", state.SessionLifespan.ValueString())
	}
	if !state.SelfserviceMethodsPasswordEnabled.IsNull() {
		t.Errorf("password_enabled = %v, want null", state.SelfserviceMethodsPasswordEnabled.ValueBool())
	}
	if !state.SelfserviceMethodsTOTPConfigIssuer.IsNull() {
		t.Errorf("totp_issuer = %q, want null", state.SelfserviceMethodsTOTPConfigIssuer.ValueString())
	}
}

// A nil service config must not be mistaken for "keys absent" — the whole
// service block is missing from the response, which says nothing about drift.
func TestReadSimpleFields_IgnoresMissingServiceBlock(t *testing.T) {
	state := &ProjectConfigResourceModel{
		SelfserviceMethodsOIDCConfigBaseRedirectURI: types.StringValue("https://iam.example.com"),
	}

	readSimpleFields(context.Background(), &ory.Project{}, state)

	if got := state.SelfserviceMethodsOIDCConfigBaseRedirectURI.ValueString(); got != "https://iam.example.com" {
		t.Errorf("base_redirect_uri = %q, want the state value preserved when the identity service block is absent", got)
	}
}
