package projectconfig

import (
	"context"
	"fmt"
	"regexp"

	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	ory "github.com/ory/client-go"

	"github.com/ory/terraform-provider-ory/internal/client"
	"github.com/ory/terraform-provider-ory/internal/helpers"
)

var (
	_ resource.Resource                = &ProjectConfigResource{}
	_ resource.ResourceWithConfigure   = &ProjectConfigResource{}
	_ resource.ResourceWithImportState = &ProjectConfigResource{}
)

func NewResource() resource.Resource {
	return &ProjectConfigResource{}
}

type ProjectConfigResource struct {
	client *client.OryClient
}

type ProjectConfigResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`

	// Keto/Permissions Namespaces
	KetoNamespaces types.List `tfsdk:"keto_namespaces"`

	// CORS (Public)
	CorsEnabled types.Bool `tfsdk:"cors_enabled"`
	CorsOrigins types.List `tfsdk:"cors_origins"`

	// CORS (Admin)
	CorsAdminEnabled types.Bool `tfsdk:"cors_admin_enabled"`
	CorsAdminOrigins types.List `tfsdk:"cors_admin_origins"`

	// Session
	SessionLifespan         types.String `tfsdk:"session_lifespan"`
	SessionCookieSameSite   types.String `tfsdk:"session_cookie_same_site"`
	SessionCookiePersistent types.Bool   `tfsdk:"session_cookie_persistent"`

	// OAuth2/Hydra
	OAuth2AccessTokenLifespan             types.String `tfsdk:"oauth2_access_token_lifespan"`
	OAuth2RefreshTokenLifespan            types.String `tfsdk:"oauth2_refresh_token_lifespan"`
	OAuth2AuthCodeLifespan                types.String `tfsdk:"oauth2_auth_code_lifespan"`
	OAuth2IDTokenLifespan                 types.String `tfsdk:"oauth2_id_token_lifespan"`
	OAuth2LoginConsentRequestLifespan     types.String `tfsdk:"oauth2_login_consent_request_lifespan"`
	OAuth2AllowedTopLevelClaims           types.List   `tfsdk:"oauth2_allowed_top_level_claims"`
	OAuth2MirrorTopLevelClaims            types.Bool   `tfsdk:"oauth2_mirror_top_level_claims"`
	OAuth2PKCEEnforced                    types.Bool   `tfsdk:"oauth2_pkce_enforced"`
	OAuth2PKCEEnforcedForPublicClients    types.Bool   `tfsdk:"oauth2_pkce_enforced_for_public_clients"`
	OAuth2AccessTokenStrategy             types.String `tfsdk:"oauth2_access_token_strategy"`
	OAuth2JWTScopeClaim                   types.String `tfsdk:"oauth2_jwt_scope_claim"`
	OAuth2ScopeStrategy                   types.String `tfsdk:"oauth2_scope_strategy"`
	OAuth2ConsentURL                      types.String `tfsdk:"oauth2_consent_url"`
	OAuth2LoginURL                        types.String `tfsdk:"oauth2_login_url"`
	OAuth2LogoutURL                       types.String `tfsdk:"oauth2_logout_url"`
	OAuth2ErrorURL                        types.String `tfsdk:"oauth2_error_url"`
	OAuth2IssuerURL                       types.String `tfsdk:"oauth2_issuer_url"`
	OAuth2CookiesSameSiteMode             types.String `tfsdk:"oauth2_cookies_same_site_mode"`
	OAuth2CookiesSameSiteLegacyWorkaround types.Bool   `tfsdk:"oauth2_cookies_same_site_legacy_workaround"`

	// URLs
	DefaultReturnURL  types.String `tfsdk:"default_return_url"`
	AllowedReturnURLs types.List   `tfsdk:"allowed_return_urls"`
	LoginUIURL        types.String `tfsdk:"login_ui_url"`
	RegistrationUIURL types.String `tfsdk:"registration_ui_url"`
	RecoveryUIURL     types.String `tfsdk:"recovery_ui_url"`
	VerificationUIURL types.String `tfsdk:"verification_ui_url"`
	SettingsUIURL     types.String `tfsdk:"settings_ui_url"`
	ErrorUIURL        types.String `tfsdk:"error_ui_url"`

	// Auth methods
	EnablePassword           types.Bool `tfsdk:"enable_password"`
	EnableCode               types.Bool `tfsdk:"enable_code"`
	CodeMFAEnabled           types.Bool `tfsdk:"code_mfa_enabled"`
	EnableOIDC               types.Bool `tfsdk:"enable_oidc"`
	EnableOIDCAutoLinkPolicy types.Bool `tfsdk:"enable_oidc_auto_link_policy"`
	EnableTOTP               types.Bool `tfsdk:"enable_totp"`
	EnableWebAuthn           types.Bool `tfsdk:"enable_webauthn"`
	EnablePasskey            types.Bool `tfsdk:"enable_passkey"`
	EnableLookupSecret       types.Bool `tfsdk:"enable_lookup_secret"`

	// Password policy
	PasswordMinLength            types.Int64 `tfsdk:"password_min_length"`
	PasswordCheckHaveIBeenPwned  types.Bool  `tfsdk:"password_check_haveibeenpwned"`
	PasswordMaxBreaches          types.Int64 `tfsdk:"password_max_breaches"`
	PasswordIdentifierSimilarity types.Bool  `tfsdk:"password_identifier_similarity"`

	// Profile method
	EnableProfile types.Bool `tfsdk:"enable_profile"`

	// Code method config
	CodeLifespan                         types.String `tfsdk:"code_lifespan"`
	CodeMissingCredentialFallbackEnabled types.Bool   `tfsdk:"code_missing_credential_fallback_enabled"`

	// Flow settings
	EnableRecovery     types.Bool   `tfsdk:"enable_recovery"`
	EnableVerification types.Bool   `tfsdk:"enable_verification"`
	EnableRegistration types.Bool   `tfsdk:"enable_registration"`
	LoginStyle         types.String `tfsdk:"login_style"`

	// Settings flow
	SettingsLifespan                types.String `tfsdk:"settings_lifespan"`
	SettingsPrivilegedSessionMaxAge types.String `tfsdk:"settings_privileged_session_max_age"`

	// Verification flow
	VerificationUse                     types.String `tfsdk:"verification_use"`
	VerificationLifespan                types.String `tfsdk:"verification_lifespan"`
	VerificationNotifyUnknownRecipients types.Bool   `tfsdk:"verification_notify_unknown_recipients"`

	// SMTP Configuration
	SMTPConnectionURI types.String `tfsdk:"smtp_connection_uri"`
	SMTPFromAddress   types.String `tfsdk:"smtp_from_address"`
	SMTPFromName      types.String `tfsdk:"smtp_from_name"`
	SMTPHeaders       types.Map    `tfsdk:"smtp_headers"`

	// MFA Policy
	MFAEnforcement           types.String `tfsdk:"mfa_enforcement"`
	TOTPIssuer               types.String `tfsdk:"totp_issuer"`
	WebAuthnRPDisplayName    types.String `tfsdk:"webauthn_rp_display_name"`
	WebAuthnRPID             types.String `tfsdk:"webauthn_rp_id"`
	WebAuthnRPOrigins        types.List   `tfsdk:"webauthn_rp_origins"`
	WebAuthnPasswordless     types.Bool   `tfsdk:"webauthn_passwordless"`
	RequiredAAL              types.String `tfsdk:"required_aal"`
	SessionWhoamiRequiredAAL types.String `tfsdk:"session_whoami_required_aal"`

	// Account Experience (Branding)
	AccountExperienceLocale types.String `tfsdk:"account_experience_default_locale"`

	// Session Tokenizer Templates
	SessionTokenizerTemplates types.Map `tfsdk:"session_tokenizer_templates"`

	// Courier HTTP Delivery
	CourierDeliveryStrategy          types.String `tfsdk:"courier_delivery_strategy"`
	CourierHTTPRequestConfig         types.Object `tfsdk:"courier_http_request_config"`
	CourierHTTPRequestConfigAuthType types.String `tfsdk:"courier_http_request_config_auth_type"`
	CourierHTTPRequestConfigMethod   types.String `tfsdk:"courier_http_request_config_method"`
	CourierChannels                  types.List   `tfsdk:"courier_channels"`

	// --- Phase 2: Auto-discovered from OpenAPI spec ---

	// OAuth2/Hydra additional
	OAuth2ClientCredentialsDefaultGrantAllowedScope   types.Bool   `tfsdk:"oauth2_client_credentials_default_grant_allowed_scope"`
	OAuth2ExcludeNotBeforeClaim                       types.Bool   `tfsdk:"oauth2_exclude_not_before_claim"`
	OAuth2GrantJWTIATOptional                         types.Bool   `tfsdk:"oauth2_grant_jwt_iat_optional"`
	OAuth2GrantJWTJTIOptional                         types.Bool   `tfsdk:"oauth2_grant_jwt_jti_optional"`
	OAuth2GrantJWTMaxTTL                              types.String `tfsdk:"oauth2_grant_jwt_max_ttl"`
	OAuth2GrantRefreshTokenRotationGracePeriod        types.String `tfsdk:"oauth2_grant_refresh_token_rotation_grace_period"`
	OAuth2GrantRefreshTokenRotationGraceReuseCount    types.Int64  `tfsdk:"oauth2_grant_refresh_token_rotation_grace_reuse_count"`
	OAuth2RefreshTokenHook                            types.String `tfsdk:"oauth2_refresh_token_hook"`
	OAuth2TokenHook                                   types.String `tfsdk:"oauth2_token_hook"`
	OAuth2TokenPrefix                                 types.String `tfsdk:"oauth2_token_prefix"`
	OIDCDynamicClientRegistrationEnabled              types.Bool   `tfsdk:"oidc_dynamic_client_registration_enabled"`
	OIDCSubjectIdentifiersPairwiseSalt                types.String `tfsdk:"oidc_subject_identifiers_pairwise_salt"`
	OAuth2UrlsPostLogoutRedirect                      types.String `tfsdk:"oauth2_urls_post_logout_redirect"`
	OAuth2UrlsRegistration                            types.String `tfsdk:"oauth2_urls_registration"`
	OAuth2WebfingerOIDCDiscoveryAuthURL               types.String `tfsdk:"oauth2_webfinger_oidc_discovery_auth_url"`
	OAuth2WebfingerOIDCDiscoveryClientRegistrationURL types.String `tfsdk:"oauth2_webfinger_oidc_discovery_client_registration_url"`
	OAuth2WebfingerOIDCDiscoveryJwksURL               types.String `tfsdk:"oauth2_webfinger_oidc_discovery_jwks_url"`
	OAuth2WebfingerOIDCDiscoveryTokenURL              types.String `tfsdk:"oauth2_webfinger_oidc_discovery_token_url"`
	OAuth2WebfingerOIDCDiscoveryUserinfoURL           types.String `tfsdk:"oauth2_webfinger_oidc_discovery_userinfo_url"`

	// Identity cookies
	CookiesSameSite types.String `tfsdk:"cookies_same_site"`

	// Courier individual fields (flat access to HTTP config)
	CourierHTTPRequestConfigAuthAPIKeyIn          types.String `tfsdk:"courier_http_request_config_auth_api_key_in"`
	CourierHTTPRequestConfigAuthAPIKeyName        types.String `tfsdk:"courier_http_request_config_auth_api_key_name"`
	CourierHTTPRequestConfigAuthAPIKeyValue       types.String `tfsdk:"courier_http_request_config_auth_api_key_value"`
	CourierHTTPRequestConfigAuthBasicAuthPassword types.String `tfsdk:"courier_http_request_config_auth_basic_auth_password"`
	CourierHTTPRequestConfigAuthBasicAuthUser     types.String `tfsdk:"courier_http_request_config_auth_basic_auth_user"`
	CourierHTTPRequestConfigBody                  types.String `tfsdk:"courier_http_request_config_body"`
	CourierHTTPRequestConfigURL                   types.String `tfsdk:"courier_http_request_config_url"`
	// courier_smtp_connection_uri is handled by SMTPConnectionURI above (sensitive, write-only)
	CourierSMTPLocalName types.String `tfsdk:"courier_smtp_local_name"`

	// Courier email/SMS templates: intentionally NOT exposed here.
	// Manage them with the dedicated `ory_email_template` resource, which
	// handles the required base64:// encoding and storage-URL drift detection.

	// Feature flags
	FeatureFlagsCacheableSessions                types.Bool   `tfsdk:"feature_flags_cacheable_sessions"`
	FeatureFlagsCacheableSessionsMaxAge          types.String `tfsdk:"feature_flags_cacheable_sessions_max_age"`
	FeatureFlagsChooseRecoveryAddress            types.Bool   `tfsdk:"feature_flags_choose_recovery_address"`
	FeatureFlagsFasterSessionExtend              types.Bool   `tfsdk:"feature_flags_faster_session_extend"`
	FeatureFlagsLegacyContinueWithVerificationUI types.Bool   `tfsdk:"feature_flags_legacy_continue_with_verification_ui"`
	FeatureFlagsLegacyOIDCRegistrationNodeGroup  types.Bool   `tfsdk:"feature_flags_legacy_oidc_registration_node_group"`
	FeatureFlagsLegacyRequireVerifiedLoginError  types.Bool   `tfsdk:"feature_flags_legacy_require_verified_login_error"`
	FeatureFlagsUseContinueWithTransitions       types.Bool   `tfsdk:"feature_flags_use_continue_with_transitions"`

	// OAuth2 provider URL
	OAuth2ProviderURL types.String `tfsdk:"oauth2_provider_url"`

	// Selfservice flow return URLs and additional settings
	SelfserviceDefaultBrowserReturnURL                               types.String `tfsdk:"selfservice_default_browser_return_url"`
	SelfserviceFlowsLoginAfterCodeDefaultBrowserReturnURL            types.String `tfsdk:"selfservice_flows_login_after_code_default_browser_return_url"`
	SelfserviceFlowsLoginAfterDefaultBrowserReturnURL                types.String `tfsdk:"selfservice_flows_login_after_default_browser_return_url"`
	SelfserviceFlowsLoginAfterLookupSecretDefaultBrowserReturnURL    types.String `tfsdk:"selfservice_flows_login_after_lookup_secret_default_browser_return_url"`
	SelfserviceFlowsLoginAfterOIDCDefaultBrowserReturnURL            types.String `tfsdk:"selfservice_flows_login_after_oidc_default_browser_return_url"`
	SelfserviceFlowsLoginAfterPasskeyDefaultBrowserReturnURL         types.String `tfsdk:"selfservice_flows_login_after_passkey_default_browser_return_url"`
	SelfserviceFlowsLoginAfterPasswordDefaultBrowserReturnURL        types.String `tfsdk:"selfservice_flows_login_after_password_default_browser_return_url"`
	SelfserviceFlowsLoginAfterTOTPDefaultBrowserReturnURL            types.String `tfsdk:"selfservice_flows_login_after_totp_default_browser_return_url"`
	SelfserviceFlowsLoginAfterWebAuthnDefaultBrowserReturnURL        types.String `tfsdk:"selfservice_flows_login_after_webauthn_default_browser_return_url"`
	SelfserviceFlowsLoginLifespan                                    types.String `tfsdk:"selfservice_flows_login_lifespan"`
	SelfserviceFlowsLogoutAfterDefaultBrowserReturnURL               types.String `tfsdk:"selfservice_flows_logout_after_default_browser_return_url"`
	SelfserviceFlowsRecoveryAfterDefaultBrowserReturnURL             types.String `tfsdk:"selfservice_flows_recovery_after_default_browser_return_url"`
	SelfserviceFlowsRecoveryLifespan                                 types.String `tfsdk:"selfservice_flows_recovery_lifespan"`
	SelfserviceFlowsRecoveryNotifyUnknownRecipients                  types.Bool   `tfsdk:"selfservice_flows_recovery_notify_unknown_recipients"`
	SelfserviceFlowsRecoveryUse                                      types.String `tfsdk:"selfservice_flows_recovery_use"`
	SelfserviceFlowsRegistrationAfterCodeDefaultBrowserReturnURL     types.String `tfsdk:"selfservice_flows_registration_after_code_default_browser_return_url"`
	SelfserviceFlowsRegistrationAfterDefaultBrowserReturnURL         types.String `tfsdk:"selfservice_flows_registration_after_default_browser_return_url"`
	SelfserviceFlowsRegistrationAfterOIDCDefaultBrowserReturnURL     types.String `tfsdk:"selfservice_flows_registration_after_oidc_default_browser_return_url"`
	SelfserviceFlowsRegistrationAfterPasskeyDefaultBrowserReturnURL  types.String `tfsdk:"selfservice_flows_registration_after_passkey_default_browser_return_url"`
	SelfserviceFlowsRegistrationAfterPasswordDefaultBrowserReturnURL types.String `tfsdk:"selfservice_flows_registration_after_password_default_browser_return_url"`
	SelfserviceFlowsRegistrationAfterWebAuthnDefaultBrowserReturnURL types.String `tfsdk:"selfservice_flows_registration_after_webauthn_default_browser_return_url"`
	SelfserviceFlowsRegistrationEnableLegacyOneStep                  types.Bool   `tfsdk:"selfservice_flows_registration_enable_legacy_one_step"`
	SelfserviceFlowsRegistrationLifespan                             types.String `tfsdk:"selfservice_flows_registration_lifespan"`
	SelfserviceFlowsRegistrationLoginHints                           types.Bool   `tfsdk:"selfservice_flows_registration_login_hints"`
	SelfserviceFlowsSettingsAfterDefaultBrowserReturnURL             types.String `tfsdk:"selfservice_flows_settings_after_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterLookupSecretDefaultBrowserReturnURL types.String `tfsdk:"selfservice_flows_settings_after_lookup_secret_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterOIDCDefaultBrowserReturnURL         types.String `tfsdk:"selfservice_flows_settings_after_oidc_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterPasskeyDefaultBrowserReturnURL      types.String `tfsdk:"selfservice_flows_settings_after_passkey_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterPasswordDefaultBrowserReturnURL     types.String `tfsdk:"selfservice_flows_settings_after_password_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterProfileDefaultBrowserReturnURL      types.String `tfsdk:"selfservice_flows_settings_after_profile_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterTOTPDefaultBrowserReturnURL         types.String `tfsdk:"selfservice_flows_settings_after_totp_default_browser_return_url"`
	SelfserviceFlowsSettingsAfterWebAuthnDefaultBrowserReturnURL     types.String `tfsdk:"selfservice_flows_settings_after_webauthn_default_browser_return_url"`
	SelfserviceFlowsVerificationAfterDefaultBrowserReturnURL         types.String `tfsdk:"selfservice_flows_verification_after_default_browser_return_url"`
	SelfserviceMethodsCodePasswordlessEnabled                        types.Bool   `tfsdk:"selfservice_methods_code_passwordless_enabled"`
	SelfserviceMethodsCodePasswordlessLoginFallbackEnabled           types.Bool   `tfsdk:"selfservice_methods_code_passwordless_login_fallback_enabled"`
	SelfserviceMethodsLinkConfigBaseURL                              types.String `tfsdk:"selfservice_methods_link_config_base_url"`
	SelfserviceMethodsLinkConfigLifespan                             types.String `tfsdk:"selfservice_methods_link_config_lifespan"`
	SelfserviceMethodsLinkEnabled                                    types.Bool   `tfsdk:"selfservice_methods_link_enabled"`
	SelfserviceMethodsOIDCConfigBaseRedirectURI                      types.String `tfsdk:"selfservice_methods_oidc_config_base_redirect_uri"`
	SelfserviceMethodsPasskeyConfigRPDisplayName                     types.String `tfsdk:"selfservice_methods_passkey_config_rp_display_name"`
	SelfserviceMethodsPasskeyConfigRPID                              types.String `tfsdk:"selfservice_methods_passkey_config_rp_id"`
	SelfserviceMethodsPasswordConfigIgnoreNetworkErrors              types.Bool   `tfsdk:"selfservice_methods_password_config_ignore_network_errors"`
	SelfserviceMethodsSAMLEnabled                                    types.Bool   `tfsdk:"selfservice_methods_saml_enabled"`
	SelfserviceMethodsWebAuthnConfigRPIcon                           types.String `tfsdk:"selfservice_methods_webauthn_config_rp_icon"`

	// Account Experience v2
	AccountExperienceEnabledLocales       types.List   `tfsdk:"account_experience_enabled_locales"`
	AccountExperienceFaviconDark          types.String `tfsdk:"account_experience_favicon_dark"`
	AccountExperienceFaviconLight         types.String `tfsdk:"account_experience_favicon_light"`
	AccountExperienceLocaleBehavior       types.String `tfsdk:"account_experience_locale_behavior"`
	AccountExperienceLogoDark             types.String `tfsdk:"account_experience_logo_dark"`
	AccountExperienceLogoLight            types.String `tfsdk:"account_experience_logo_light"`
	AccountExperienceThemeVariablesDark   types.String `tfsdk:"account_experience_theme_variables_dark"`
	AccountExperienceThemeVariablesLight  types.String `tfsdk:"account_experience_theme_variables_light"`
	DisableAccountExperienceWelcomeScreen types.Bool   `tfsdk:"disable_account_experience_welcome_screen"`
	EnableAXV2                            types.Bool   `tfsdk:"enable_ax_v2"`

	// CAPTCHA
	SelfserviceMethodsCaptchaConfigAllowedDomains     types.List   `tfsdk:"selfservice_methods_captcha_config_allowed_domains"`
	SelfserviceMethodsCaptchaConfigBYO                types.Bool   `tfsdk:"selfservice_methods_captcha_config_byo"`
	SelfserviceMethodsCaptchaConfigCFTurnstileSecret  types.String `tfsdk:"selfservice_methods_captcha_config_cf_turnstile_secret"`
	SelfserviceMethodsCaptchaConfigCFTurnstileSitekey types.String `tfsdk:"selfservice_methods_captcha_config_cf_turnstile_sitekey"`
	SelfserviceMethodsCaptchaConfigLegacyInjectNode   types.Bool   `tfsdk:"selfservice_methods_captcha_config_legacy_inject_node"`
	SelfserviceMethodsCaptchaEnabled                  types.Bool   `tfsdk:"selfservice_methods_captcha_enabled"`

	// Code max submissions
	SelfserviceMethodsCodeConfigMaxSubmissions types.Int64 `tfsdk:"selfservice_methods_code_config_max_submissions"`

	// Passkey origins
	SelfserviceMethodsPasskeyConfigRPOrigins types.List `tfsdk:"selfservice_methods_passkey_config_rp_origins"`

	// OAuth2 provider
	OAuth2ProviderHeaders          types.Map  `tfsdk:"oauth2_provider_headers"`
	OAuth2ProviderOverrideReturnTo types.Bool `tfsdk:"oauth2_provider_override_return_to"`

	// Identity secrets
	IdentitySecretsCipher     types.List `tfsdk:"identity_secrets_cipher"`
	IdentitySecretsCookie     types.List `tfsdk:"identity_secrets_cookie"`
	IdentitySecretsDefault    types.List `tfsdk:"identity_secrets_default"`
	IdentitySecretsPagination types.List `tfsdk:"identity_secrets_pagination"`

	// OAuth2 secrets
	OAuth2SecretsCookie     types.List `tfsdk:"oauth2_secrets_cookie"`
	OAuth2SecretsPagination types.List `tfsdk:"oauth2_secrets_pagination"`
	OAuth2SecretsSystem     types.List `tfsdk:"oauth2_secrets_system"`

	// OAuth2 OIDC
	OIDCDynamicClientRegistrationDefaultScope types.List `tfsdk:"oidc_dynamic_client_registration_default_scope"`
	OIDCSubjectIdentifiersSupportedTypes      types.List `tfsdk:"oidc_subject_identifiers_supported_types"`

	// OAuth2 webfinger
	OAuth2WebfingerJWKSBroadcastKeys            types.List `tfsdk:"oauth2_webfinger_jwks_broadcast_keys"`
	OAuth2WebfingerOIDCDiscoverySupportedClaims types.List `tfsdk:"oauth2_webfinger_oidc_discovery_supported_claims"`
	OAuth2WebfingerOIDCDiscoverySupportedScope  types.List `tfsdk:"oauth2_webfinger_oidc_discovery_supported_scope"`

	// Keto
	KetoNamespaceConfiguration types.String `tfsdk:"keto_namespace_configuration"`
	KetoSecretsPagination      types.List   `tfsdk:"keto_secrets_pagination"`

	// Security
	SecurityAccountEnumerationMitigate types.Bool `tfsdk:"security_account_enumeration_mitigate"`

	// Preview
	PreviewDefaultReadConsistencyLevel types.String `tfsdk:"preview_default_read_consistency_level"`

	// Feature flags
	FeatureFlagsPasswordProfileRegistrationNodeGroup types.Bool `tfsdk:"feature_flags_password_profile_registration_node_group"`

	// --- Spec-derived aliases for deprecated attribute names ---
	// These are the preferred names. The old names above still work but show deprecation warnings.

	// OAuth2 TTLs
	OAuth2TTLAccessToken         types.String `tfsdk:"oauth2_ttl_access_token"`
	OAuth2TTLRefreshToken        types.String `tfsdk:"oauth2_ttl_refresh_token"`
	OAuth2TTLAuthCode            types.String `tfsdk:"oauth2_ttl_auth_code"`
	OAuth2TTLIDToken             types.String `tfsdk:"oauth2_ttl_id_token"`
	OAuth2TTLLoginConsentRequest types.String `tfsdk:"oauth2_ttl_login_consent_request"`

	// OAuth2 Strategies
	OAuth2StrategiesAccessToken   types.String `tfsdk:"oauth2_strategies_access_token"`
	OAuth2StrategiesJWTScopeClaim types.String `tfsdk:"oauth2_strategies_jwt_scope_claim"`
	OAuth2StrategiesScope         types.String `tfsdk:"oauth2_strategies_scope"`

	// OAuth2 URLs
	OAuth2URLsConsent    types.String `tfsdk:"oauth2_urls_consent"`
	OAuth2URLsLogin      types.String `tfsdk:"oauth2_urls_login"`
	OAuth2URLsLogout     types.String `tfsdk:"oauth2_urls_logout"`
	OAuth2URLsError      types.String `tfsdk:"oauth2_urls_error"`
	OAuth2URLsSelfIssuer types.String `tfsdk:"oauth2_urls_self_issuer"`

	// OAuth2 Cookies
	OAuth2ServeCookiesSameSiteMode             types.String `tfsdk:"oauth2_serve_cookies_same_site_mode"`
	OAuth2ServeCookiesSameSiteLegacyWorkaround types.Bool   `tfsdk:"oauth2_serve_cookies_same_site_legacy_workaround"`

	// UI URLs
	SelfserviceFlowsLoginUIURL        types.String `tfsdk:"selfservice_flows_login_ui_url"`
	SelfserviceFlowsRegistrationUIURL types.String `tfsdk:"selfservice_flows_registration_ui_url"`
	SelfserviceFlowsRecoveryUIURL     types.String `tfsdk:"selfservice_flows_recovery_ui_url"`
	SelfserviceFlowsVerificationUIURL types.String `tfsdk:"selfservice_flows_verification_ui_url"`
	SelfserviceFlowsSettingsUIURL     types.String `tfsdk:"selfservice_flows_settings_ui_url"`
	SelfserviceFlowsErrorUIURL        types.String `tfsdk:"selfservice_flows_error_ui_url"`

	// Auth method enables
	SelfserviceMethodsPasswordEnabled          types.Bool `tfsdk:"selfservice_methods_password_enabled"`
	SelfserviceMethodsCodeEnabled              types.Bool `tfsdk:"selfservice_methods_code_enabled"`
	SelfserviceMethodsCodeMFAEnabled           types.Bool `tfsdk:"selfservice_methods_code_mfa_enabled"`
	SelfserviceMethodsOIDCEnabled              types.Bool `tfsdk:"selfservice_methods_oidc_enabled"`
	SelfserviceMethodsOIDCEnableAutoLinkPolicy types.Bool `tfsdk:"selfservice_methods_oidc_enable_auto_link_policy"`
	SelfserviceMethodsTOTPEnabled              types.Bool `tfsdk:"selfservice_methods_totp_enabled"`
	SelfserviceMethodsWebAuthnEnabled          types.Bool `tfsdk:"selfservice_methods_webauthn_enabled"`
	SelfserviceMethodsPasskeyEnabled           types.Bool `tfsdk:"selfservice_methods_passkey_enabled"`
	SelfserviceMethodsLookupSecretEnabled      types.Bool `tfsdk:"selfservice_methods_lookup_secret_enabled"`
	SelfserviceMethodsProfileEnabled           types.Bool `tfsdk:"selfservice_methods_profile_enabled"`

	// Code config
	SelfserviceMethodsCodeConfigLifespan                         types.String `tfsdk:"selfservice_methods_code_config_lifespan"`
	SelfserviceMethodsCodeConfigMissingCredentialFallbackEnabled types.Bool   `tfsdk:"selfservice_methods_code_config_missing_credential_fallback_enabled"`

	// Password config
	SelfserviceMethodsPasswordConfigMinPasswordLength                types.Int64 `tfsdk:"selfservice_methods_password_config_min_password_length"`
	SelfserviceMethodsPasswordConfigHaveIBeenPwnedEnabled            types.Bool  `tfsdk:"selfservice_methods_password_config_haveibeenpwned_enabled"`
	SelfserviceMethodsPasswordConfigMaxBreaches                      types.Int64 `tfsdk:"selfservice_methods_password_config_max_breaches"`
	SelfserviceMethodsPasswordConfigIdentifierSimilarityCheckEnabled types.Bool  `tfsdk:"selfservice_methods_password_config_identifier_similarity_check_enabled"`

	// Flow enables/settings
	SelfserviceFlowsRecoveryEnabled                     types.Bool   `tfsdk:"selfservice_flows_recovery_enabled"`
	SelfserviceFlowsVerificationEnabled                 types.Bool   `tfsdk:"selfservice_flows_verification_enabled"`
	SelfserviceFlowsRegistrationEnabled                 types.Bool   `tfsdk:"selfservice_flows_registration_enabled"`
	SelfserviceFlowsLoginStyle                          types.String `tfsdk:"selfservice_flows_login_style"`
	SelfserviceFlowsSettingsLifespan                    types.String `tfsdk:"selfservice_flows_settings_lifespan"`
	SelfserviceFlowsSettingsPrivilegedSessionMaxAge     types.String `tfsdk:"selfservice_flows_settings_privileged_session_max_age"`
	SelfserviceFlowsSettingsRequiredAAL                 types.String `tfsdk:"selfservice_flows_settings_required_aal"`
	SelfserviceFlowsVerificationUse                     types.String `tfsdk:"selfservice_flows_verification_use"`
	SelfserviceFlowsVerificationLifespan                types.String `tfsdk:"selfservice_flows_verification_lifespan"`
	SelfserviceFlowsVerificationNotifyUnknownRecipients types.Bool   `tfsdk:"selfservice_flows_verification_notify_unknown_recipients"`

	// SMTP
	CourierSMTPFromAddress types.String `tfsdk:"courier_smtp_from_address"`
	CourierSMTPFromName    types.String `tfsdk:"courier_smtp_from_name"`

	// MFA/WebAuthn/TOTP
	SelfserviceMethodsTOTPConfigIssuer            types.String `tfsdk:"selfservice_methods_totp_config_issuer"`
	SelfserviceMethodsWebAuthnConfigRPDisplayName types.String `tfsdk:"selfservice_methods_webauthn_config_rp_display_name"`
	SelfserviceMethodsWebAuthnConfigRPID          types.String `tfsdk:"selfservice_methods_webauthn_config_rp_id"`
	SelfserviceMethodsWebAuthnConfigPasswordless  types.Bool   `tfsdk:"selfservice_methods_webauthn_config_passwordless"`

	// Auto-discovered (review naming before release)
	OAuth2PreserveExtClaims types.Bool `tfsdk:"oauth2_preserve_ext_claims"`

	// Auto-discovered (review naming before release)
	AccountExperienceHideOryBranding      types.Bool `tfsdk:"account_experience_hide_ory_branding"`
	AccountExperienceHideRegistrationLink types.Bool `tfsdk:"account_experience_hide_registration_link"`
}

// --- Nested model types for session tokenizer templates and courier HTTP ---

type SessionTokenizerTemplateModel struct {
	TTL             types.String `tfsdk:"ttl"`
	JWKSURL         types.String `tfsdk:"jwks_url"`
	ClaimsMapperURL types.String `tfsdk:"claims_mapper_url"`
	SubjectSource   types.String `tfsdk:"subject_source"`
}

type CourierHTTPAuthModel struct {
	Type     types.String `tfsdk:"type"`
	User     types.String `tfsdk:"user"`
	Password types.String `tfsdk:"password"`
	Name     types.String `tfsdk:"name"`
	Value    types.String `tfsdk:"value"`
	In       types.String `tfsdk:"in"`
}

type CourierHTTPRequestConfigModel struct {
	URL     types.String `tfsdk:"url"`
	Method  types.String `tfsdk:"method"`
	Headers types.Map    `tfsdk:"headers"`
	Body    types.String `tfsdk:"body"`
	Auth    types.Object `tfsdk:"auth"`
}

type CourierChannelModel struct {
	ID            types.String `tfsdk:"id"`
	RequestConfig types.Object `tfsdk:"request_config"`
}

// Shared attr.Type maps for constructing types.Object / types.Map / types.List values.
var (
	tokenizerTemplateAttrTypes = map[string]attr.Type{
		"ttl":               types.StringType,
		"jwks_url":          types.StringType,
		"claims_mapper_url": types.StringType,
		"subject_source":    types.StringType,
	}

	courierHTTPAuthAttrTypes = map[string]attr.Type{
		"type":     types.StringType,
		"user":     types.StringType,
		"password": types.StringType,
		"name":     types.StringType,
		"value":    types.StringType,
		"in":       types.StringType,
	}

	courierHTTPRequestConfigAttrTypes = map[string]attr.Type{
		"url":     types.StringType,
		"method":  types.StringType,
		"headers": types.MapType{ElemType: types.StringType},
		"body":    types.StringType,
		"auth":    types.ObjectType{AttrTypes: courierHTTPAuthAttrTypes},
	}

	courierChannelAttrTypes = map[string]attr.Type{
		"id":             types.StringType,
		"request_config": types.ObjectType{AttrTypes: courierHTTPRequestConfigAttrTypes},
	}
)

func (r *ProjectConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_config"
}

const projectConfigMarkdownDescription = `
Configures an Ory Network project's settings.

This resource manages the configuration of an Ory Network project, including authentication methods,
password policies, session settings, CORS, and more.

## Example Usage

` + "```hcl" + `
resource "ory_project_config" "main" {
  cors_enabled        = true
  cors_origins        = ["https://app.example.com"]
  password_min_length = 10
  session_lifespan    = "720h"  # 30 days
}
` + "```" + `

## Import

Import using the project ID:

` + "```shell" + `
terraform import ory_project_config.main <project-id>
` + "```" + `

### Avoiding "Forces Replacement" After Import

After importing, if Terraform shows ` + "`project_id forces replacement`" + `, ensure your configuration matches:

**Option 1: Explicit project_id**
` + "```hcl" + `
resource "ory_project_config" "main" {
  project_id = "the-exact-project-id-you-imported"
  # ... other settings
}
` + "```" + `

**Option 2: Use provider default** (recommended)
` + "```hcl" + `
provider "ory" {
  project_id = "the-exact-project-id-you-imported"
}

resource "ory_project_config" "main" {
  # project_id inherits from provider
  # ... other settings
}
` + "```" + `

## Notes

- Project config cannot be deleted - it always exists for a project
- Deleting this resource from Terraform state does not reset the project configuration
- The ` + "`project_id`" + ` attribute forces replacement if changed (you cannot move config to a different project)
`

func (r *ProjectConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Start with generated simple attributes
	attrs := simpleSchemaAttributes()

	// Add custom/complex attributes that can't be generated
	attrs["id"] = schema.StringAttribute{
		Description: "Resource ID (same as project_id).",
		Computed:    true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
	attrs["project_id"] = schema.StringAttribute{
		Description: "Project ID to configure. If not set, uses provider's project_id.",
		Optional:    true,
		Computed:    true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		},
	}
	// --- Hand-written attributes (not in mappings.yaml) ---
	// These require custom logic that the codegen can't express:
	//   CORS (4):                   project-level /cors_* paths (not /services/*/config/)
	//   keto_namespaces:            transforms name list to [{name,id}] objects
	//   default_return_url:         remove-on-empty (empty string sends "remove" patch)
	//   allowed_return_urls:        remove-on-empty (empty list sends "remove" patch)
	//   mfa_enforcement:            maps string to AAL enum
	//   session_tokenizer_templates: nested object schema
	//   courier_channels:           nested objects with sub-config
	//   courier_http_request_config: nested object (url/method/headers/body/auth)

	attrs["keto_namespaces"] = schema.ListAttribute{
		Description: "List of Keto namespace names to configure for Ory Permissions. " +
			"Namespaces define the types of resources in your permission model (e.g., 'documents', 'folders'). " +
			"Each namespace name must be unique.",
		Optional:    true,
		ElementType: types.StringType,
	}
	attrs["cors_enabled"] = schema.BoolAttribute{
		Description: "Enable CORS for the public API.",
		Optional:    true,
		Computed:    true,
		Default:     booldefault.StaticBool(false),
	}
	attrs["cors_origins"] = schema.ListAttribute{
		Description: "Allowed CORS origins.",
		Optional:    true,
		ElementType: types.StringType,
	}
	attrs["cors_admin_enabled"] = schema.BoolAttribute{
		Description: "Enable CORS for the admin API.",
		Optional:    true,
	}
	attrs["cors_admin_origins"] = schema.ListAttribute{
		Description: "Allowed CORS origins for the admin API.",
		Optional:    true,
		ElementType: types.StringType,
	}
	attrs["default_return_url"] = schema.StringAttribute{
		Description: "Default URL to redirect after flows.",
		Optional:    true,
	}
	attrs["allowed_return_urls"] = schema.ListAttribute{
		Description: "List of allowed return URLs.",
		Optional:    true,
		ElementType: types.StringType,
	}
	attrs["mfa_enforcement"] = schema.StringAttribute{
		Description: "MFA enforcement level: 'none', 'optional', or 'required'.",
		Optional:    true,
	}

	// Session Tokenizer Templates
	attrs["session_tokenizer_templates"] = schema.MapNestedAttribute{
		Description: "JWT tokenizer templates for the /sessions/whoami endpoint. " +
			"Each key is a template name, and the value configures how JWTs are generated.",
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"ttl": schema.StringAttribute{
					Description: "Token time-to-live duration (e.g., '1h', '30m'). Default: '1m'.",
					Optional:    true,
					Validators: []validator.String{
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^[0-9]+(ns|us|ms|s|m|h)$`),
							"must be a valid Go duration (e.g., '1h', '30m', '10s')",
						),
					},
				},
				"jwks_url": schema.StringAttribute{
					Description: "JWKS URL for signing tokens. Must use base64:// scheme (e.g., 'base64://eyJrZXlzIjpbXX0=').",
					Required:    true,
					Sensitive:   true,
					Validators: []validator.String{
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^base64://`),
							"must start with 'base64://'",
						),
					},
				},
				"claims_mapper_url": schema.StringAttribute{
					Description: "Jsonnet claims mapper URL. Supports base64:// and https:// schemes.",
					Optional:    true,
				},
				"subject_source": schema.StringAttribute{
					Description: "Subject source for the JWT: 'id' (default) or 'external_id'.",
					Optional:    true,
					Validators: []validator.String{
						stringvalidator.OneOf("id", "external_id"),
					},
				},
			},
		},
	}

	// Courier HTTP Delivery
	attrs["courier_http_request_config"] = courierHTTPRequestConfigSchemaAttr(
		"HTTP request configuration for courier message delivery (used when courier_delivery_strategy is 'http').",
	)
	attrs["courier_channels"] = schema.ListNestedAttribute{
		Description: "Per-channel courier delivery configurations (e.g., SMS via Twilio). " +
			"Each channel overrides the default delivery for a specific message channel.",
		Optional: true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Description: "Channel identifier (e.g., 'sms').",
					Required:    true,
				},
				"request_config": courierHTTPRequestConfigSchemaAttr(
					"HTTP request configuration for this channel.",
				),
			},
		},
	}

	resp.Schema = schema.Schema{
		Description:         "Configures an Ory Network project's settings.",
		MarkdownDescription: projectConfigMarkdownDescription,
		Attributes:          attrs,
	}
}

func courierHTTPAuthSchemaAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"type": schema.StringAttribute{
			Description: "Authentication type: 'basic_auth' or 'api_key'.",
			Required:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("basic_auth", "api_key"),
			},
		},
		"user": schema.StringAttribute{
			Description: "Username for basic_auth.",
			Optional:    true,
		},
		"password": schema.StringAttribute{
			Description: "Password for basic_auth.",
			Optional:    true,
			Sensitive:   true,
		},
		"name": schema.StringAttribute{
			Description: "Header/cookie/query parameter name for api_key auth.",
			Optional:    true,
		},
		"value": schema.StringAttribute{
			Description: "API key value for api_key auth.",
			Optional:    true,
			Sensitive:   true,
		},
		"in": schema.StringAttribute{
			Description: "Where to send the API key: 'header', 'cookie', or 'query'.",
			Optional:    true,
			Validators: []validator.String{
				stringvalidator.OneOf("header", "cookie", "query"),
			},
		},
	}
}

func courierHTTPRequestConfigSchemaAttr(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: description,
		Optional:    true,
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "Target URL for the HTTP request.",
				Required:    true,
			},
			"method": schema.StringAttribute{
				Description: "HTTP method (e.g., 'POST', 'PUT').",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("POST", "PUT", "PATCH", "GET"),
				},
			},
			"headers": schema.MapAttribute{
				Description: "Additional HTTP headers to include.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"body": schema.StringAttribute{
				Description: "Request body template. Supports base64:// scheme for Jsonnet templates.",
				Optional:    true,
			},
			"auth": schema.SingleNestedAttribute{
				Description: "Authentication configuration for the HTTP request.",
				Optional:    true,
				Attributes:  courierHTTPAuthSchemaAttrs(),
			},
		},
	}
}

func (r *ProjectConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// =============================================================================
// buildPatches — uses generated tables for simple attrs + custom logic
// =============================================================================

func (r *ProjectConfigResource) buildPatches(ctx context.Context, plan *ProjectConfigResourceModel) []ory.JsonPatch {
	var patches []ory.JsonPatch

	// --- Generated simple attribute patches ---
	for _, e := range simpleStringPatchEntries(plan) {
		field := e.Field
		if (field.IsNull() || field.IsUnknown()) && e.Deprecated != nil {
			field = e.Deprecated
		}
		if !field.IsNull() && !field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  e.Path,
				Value: field.ValueString(),
			})
		}
	}
	for _, e := range simpleBoolPatchEntries(plan) {
		field := e.Field
		if (field.IsNull() || field.IsUnknown()) && e.Deprecated != nil {
			field = e.Deprecated
		}
		if !field.IsNull() && !field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  e.Path,
				Value: field.ValueBool(),
			})
		}
	}
	for _, e := range simpleInt64PatchEntries(plan) {
		field := e.Field
		if (field.IsNull() || field.IsUnknown()) && e.Deprecated != nil {
			field = e.Deprecated
		}
		if !field.IsNull() && !field.IsUnknown() {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  e.Path,
				Value: field.ValueInt64(),
			})
		}
	}
	for _, e := range simpleListStringPatchEntries(plan) {
		field := e.Field
		if (field.IsNull() || field.IsUnknown()) && e.Deprecated != nil {
			field = e.Deprecated
		}
		if !field.IsNull() && !field.IsUnknown() {
			var vals []string
			field.ElementsAs(ctx, &vals, false)
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  e.Path,
				Value: vals,
			})
		}
	}
	for _, e := range simpleMapStringPatchEntries(plan) {
		field := e.Field
		if (field.IsNull() || field.IsUnknown()) && e.Deprecated != nil {
			field = e.Deprecated
		}
		if !field.IsNull() && !field.IsUnknown() {
			var vals map[string]string
			field.ElementsAs(ctx, &vals, false)
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  e.Path,
				Value: vals,
			})
		}
	}

	// --- Custom patches for complex types ---

	// Keto/Permissions Namespaces
	if !plan.KetoNamespaces.IsNull() && !plan.KetoNamespaces.IsUnknown() {
		var namespaceNames []string
		plan.KetoNamespaces.ElementsAs(ctx, &namespaceNames, false)
		namespaces := make([]map[string]interface{}, len(namespaceNames))
		for i, name := range namespaceNames {
			namespaces[i] = map[string]interface{}{
				"name": name,
				"id":   i + 1,
			}
		}
		patches = append(patches, ory.JsonPatch{
			Op:    "add",
			Path:  "/services/permission/config/namespaces",
			Value: namespaces,
		})
	}

	// CORS
	if !plan.CorsEnabled.IsNull() && !plan.CorsEnabled.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_public/enabled",
			Value: plan.CorsEnabled.ValueBool(),
		})
	}
	if !plan.CorsOrigins.IsNull() && !plan.CorsOrigins.IsUnknown() {
		var origins []string
		plan.CorsOrigins.ElementsAs(ctx, &origins, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_public/origins",
			Value: origins,
		})
	}
	if !plan.CorsAdminEnabled.IsNull() && !plan.CorsAdminEnabled.IsUnknown() {
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_admin/enabled",
			Value: plan.CorsAdminEnabled.ValueBool(),
		})
	}
	if !plan.CorsAdminOrigins.IsNull() && !plan.CorsAdminOrigins.IsUnknown() {
		var origins []string
		plan.CorsAdminOrigins.ElementsAs(ctx, &origins, false)
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/cors_admin/origins",
			Value: origins,
		})
	}

	// URLs with remove-on-empty semantics
	if !plan.DefaultReturnURL.IsNull() && !plan.DefaultReturnURL.IsUnknown() {
		if plan.DefaultReturnURL.ValueString() == "" {
			patches = append(patches, ory.JsonPatch{
				Op:   "remove",
				Path: "/services/identity/config/selfservice/default_browser_return_url",
			})
		} else {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  "/services/identity/config/selfservice/default_browser_return_url",
				Value: plan.DefaultReturnURL.ValueString(),
			})
		}
	}
	if !plan.AllowedReturnURLs.IsNull() && !plan.AllowedReturnURLs.IsUnknown() {
		var urls []string
		plan.AllowedReturnURLs.ElementsAs(ctx, &urls, false)
		if len(urls) == 0 {
			patches = append(patches, ory.JsonPatch{
				Op:   "remove",
				Path: "/services/identity/config/selfservice/allowed_return_urls",
			})
		} else {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  "/services/identity/config/selfservice/allowed_return_urls",
				Value: urls,
			})
		}
	}

	// MFA enforcement (special mapping)
	if !plan.MFAEnforcement.IsNull() && !plan.MFAEnforcement.IsUnknown() {
		enforcement := plan.MFAEnforcement.ValueString()
		if enforcement == "required" {
			patches = append(patches, ory.JsonPatch{
				Op:    "replace",
				Path:  "/services/identity/config/selfservice/flows/settings/required_aal",
				Value: "aal2",
			})
		}
	}

	// Session Tokenizer Templates
	if !plan.SessionTokenizerTemplates.IsNull() && !plan.SessionTokenizerTemplates.IsUnknown() {
		var templates map[string]SessionTokenizerTemplateModel
		plan.SessionTokenizerTemplates.ElementsAs(ctx, &templates, false)
		templatesMap := make(map[string]interface{}, len(templates))
		for name, tmpl := range templates {
			entry := map[string]interface{}{}
			if !tmpl.TTL.IsNull() && !tmpl.TTL.IsUnknown() {
				entry["ttl"] = tmpl.TTL.ValueString()
			}
			if !tmpl.JWKSURL.IsNull() && !tmpl.JWKSURL.IsUnknown() {
				entry["jwks_url"] = tmpl.JWKSURL.ValueString()
			}
			if !tmpl.ClaimsMapperURL.IsNull() && !tmpl.ClaimsMapperURL.IsUnknown() {
				entry["claims_mapper_url"] = tmpl.ClaimsMapperURL.ValueString()
			}
			if !tmpl.SubjectSource.IsNull() && !tmpl.SubjectSource.IsUnknown() {
				entry["subject_source"] = tmpl.SubjectSource.ValueString()
			}
			templatesMap[name] = entry
		}
		patches = append(patches, ory.JsonPatch{
			Op:    "add",
			Path:  "/services/identity/config/session/whoami/tokenizer/templates",
			Value: templatesMap,
		})
	}

	// Courier HTTP Request Config
	if !plan.CourierHTTPRequestConfig.IsNull() && !plan.CourierHTTPRequestConfig.IsUnknown() {
		var reqConfig CourierHTTPRequestConfigModel
		plan.CourierHTTPRequestConfig.As(ctx, &reqConfig, basetypes.ObjectAsOptions{})
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/http/request_config",
			Value: buildHTTPRequestConfigMap(ctx, &reqConfig),
		})
	}

	// Courier Channels
	if !plan.CourierChannels.IsNull() && !plan.CourierChannels.IsUnknown() {
		var channels []CourierChannelModel
		plan.CourierChannels.ElementsAs(ctx, &channels, false)
		channelsList := make([]map[string]interface{}, 0, len(channels))
		for _, ch := range channels {
			chMap := map[string]interface{}{
				"id": ch.ID.ValueString(),
			}
			if !ch.RequestConfig.IsNull() && !ch.RequestConfig.IsUnknown() {
				var reqConfig CourierHTTPRequestConfigModel
				ch.RequestConfig.As(ctx, &reqConfig, basetypes.ObjectAsOptions{})
				chMap["request_config"] = buildHTTPRequestConfigMap(ctx, &reqConfig)
			}
			channelsList = append(channelsList, chMap)
		}
		patches = append(patches, ory.JsonPatch{
			Op:    "replace",
			Path:  "/services/identity/config/courier/channels",
			Value: channelsList,
		})
	}

	return patches
}

func buildHTTPRequestConfigMap(ctx context.Context, cfg *CourierHTTPRequestConfigModel) map[string]interface{} {
	result := map[string]interface{}{
		"url":    cfg.URL.ValueString(),
		"method": cfg.Method.ValueString(),
	}
	if !cfg.Body.IsNull() && !cfg.Body.IsUnknown() {
		result["body"] = cfg.Body.ValueString()
	}
	if !cfg.Headers.IsNull() && !cfg.Headers.IsUnknown() {
		var headers map[string]string
		cfg.Headers.ElementsAs(ctx, &headers, false)
		result["headers"] = headers
	}
	if !cfg.Auth.IsNull() && !cfg.Auth.IsUnknown() {
		var auth CourierHTTPAuthModel
		cfg.Auth.As(ctx, &auth, basetypes.ObjectAsOptions{})
		result["auth"] = buildAuthConfigMap(&auth)
	}
	return result
}

func buildAuthConfigMap(auth *CourierHTTPAuthModel) map[string]interface{} {
	authType := auth.Type.ValueString()
	config := map[string]interface{}{}
	switch authType {
	case "basic_auth":
		if !auth.User.IsNull() && !auth.User.IsUnknown() {
			config["user"] = auth.User.ValueString()
		}
		if !auth.Password.IsNull() && !auth.Password.IsUnknown() {
			config["password"] = auth.Password.ValueString()
		}
	case "api_key":
		if !auth.Name.IsNull() && !auth.Name.IsUnknown() {
			config["name"] = auth.Name.ValueString()
		}
		if !auth.Value.IsNull() && !auth.Value.IsUnknown() {
			config["value"] = auth.Value.ValueString()
		}
		if !auth.In.IsNull() && !auth.In.IsUnknown() {
			config["in"] = auth.In.ValueString()
		}
	}
	return map[string]interface{}{
		"type":   authType,
		"config": config,
	}
}

// =============================================================================
// CRUD operations
// =============================================================================

func (r *ProjectConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ProjectConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	patches := r.buildPatches(ctx, &plan)

	tflog.Debug(ctx, "Building project config patches", map[string]interface{}{
		"project_id":  projectID,
		"patch_count": len(patches),
	})

	if len(patches) > 0 {
		_, err := r.client.PatchProject(ctx, projectID, patches)
		if err != nil {
			resp.Diagnostics.AddError("Error Applying Project Config", err.Error())
			return
		}
		tflog.Debug(ctx, "Successfully applied project config patches", map[string]interface{}{
			"project_id":  projectID,
			"patch_count": len(patches),
		})
	}

	plan.ID = types.StringValue(projectID)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ProjectConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(state.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var project *ory.Project
	if cached := r.client.GetCachedProject(projectID); cached != nil {
		project = cached
	} else {
		var err error
		project, err = r.client.GetProject(ctx, projectID)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Project Config",
				"Could not read project "+projectID+": "+err.Error())
			return
		}
	}

	r.readProjectConfig(ctx, project, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// =============================================================================
// readProjectConfig — uses generated tables + custom read logic
// =============================================================================

func (r *ProjectConfigResource) readProjectConfig(ctx context.Context, project *ory.Project, state *ProjectConfigResourceModel) {
	// --- Generated simple attribute reads ---
	readSimpleFields(ctx, project, state)

	// --- Custom reads for complex types ---

	// CORS (Public) — uses project struct, not config map
	if project.CorsPublic != nil {
		if !state.CorsEnabled.IsNull() {
			if project.CorsPublic.Enabled != nil {
				state.CorsEnabled = types.BoolValue(*project.CorsPublic.Enabled)
			}
		}
		if !state.CorsOrigins.IsNull() {
			if len(project.CorsPublic.Origins) > 0 {
				originsList, diags := types.ListValueFrom(ctx, types.StringType, project.CorsPublic.Origins)
				if !diags.HasError() {
					state.CorsOrigins = originsList
				}
			}
		}
	}

	// CORS (Admin)
	if project.CorsAdmin != nil {
		if !state.CorsAdminEnabled.IsNull() {
			if project.CorsAdmin.Enabled != nil {
				state.CorsAdminEnabled = types.BoolValue(*project.CorsAdmin.Enabled)
			}
		}
		if !state.CorsAdminOrigins.IsNull() {
			if len(project.CorsAdmin.Origins) > 0 {
				originsList, diags := types.ListValueFrom(ctx, types.StringType, project.CorsAdmin.Origins)
				if !diags.HasError() {
					state.CorsAdminOrigins = originsList
				}
			}
		}
	}

	// Identity service custom reads
	if project.Services.Identity != nil {
		identityConfig := project.Services.Identity.Config

		// URLs with special behavior
		if !state.DefaultReturnURL.IsNull() {
			if state.DefaultReturnURL.ValueString() != "" {
				if v, ok := getNestedString(identityConfig, "selfservice", "default_browser_return_url"); ok {
					state.DefaultReturnURL = types.StringValue(v)
				}
			} else if v, ok := getNestedString(identityConfig, "selfservice", "default_browser_return_url"); ok && v != "" {
				tflog.Warn(ctx, "API returned non-empty default_browser_return_url after remove patch; preserving empty state value", map[string]interface{}{
					"api_value": v,
				})
			}
		}
		if !state.AllowedReturnURLs.IsNull() {
			if len(state.AllowedReturnURLs.Elements()) == 0 {
				if v := getNestedValue(identityConfig, "selfservice", "allowed_return_urls"); v != nil {
					if apiURLs, ok := v.([]interface{}); ok && len(apiURLs) > 0 {
						tflog.Warn(ctx, "API returned non-empty allowed_return_urls after remove patch; preserving empty state value", map[string]interface{}{
							"api_url_count": len(apiURLs),
						})
					}
				}
			} else if v := getNestedValue(identityConfig, "selfservice", "allowed_return_urls"); v != nil {
				if apiURLs, ok := v.([]interface{}); ok && len(apiURLs) > 0 {
					apiSet := make(map[string]struct{}, len(apiURLs))
					for _, u := range apiURLs {
						if s, ok := u.(string); ok {
							apiSet[s] = struct{}{}
						}
					}
					var stateURLs []string
					state.AllowedReturnURLs.ElementsAs(ctx, &stateURLs, false)
					filtered := make([]string, 0, len(stateURLs))
					for _, u := range stateURLs {
						if _, exists := apiSet[u]; exists {
							filtered = append(filtered, u)
						}
					}
					urlsList, diags := types.ListValueFrom(ctx, types.StringType, filtered)
					if !diags.HasError() {
						state.AllowedReturnURLs = urlsList
					}
				}
			}
		}

		// Session Tokenizer Templates
		if !state.SessionTokenizerTemplates.IsNull() {
			v := getNestedValue(identityConfig, "session", "whoami", "tokenizer", "templates")
			templatesRaw, rawOK := v.(map[string]interface{})
			if v == nil || !rawOK {
				state.SessionTokenizerTemplates = types.MapNull(types.ObjectType{AttrTypes: tokenizerTemplateAttrTypes})
			} else if len(templatesRaw) == 0 {
				emptyMap, diags := types.MapValue(types.ObjectType{AttrTypes: tokenizerTemplateAttrTypes}, map[string]attr.Value{})
				if !diags.HasError() {
					state.SessionTokenizerTemplates = emptyMap
				}
			} else {
				templateObjects := make(map[string]attr.Value, len(templatesRaw))
				for name, tmplRaw := range templatesRaw {
					tmplMap, ok := tmplRaw.(map[string]interface{})
					if !ok {
						continue
					}
					attrs := map[string]attr.Value{
						"ttl":               types.StringNull(),
						"jwks_url":          types.StringNull(),
						"claims_mapper_url": types.StringNull(),
						"subject_source":    types.StringNull(),
					}
					if s, ok := tmplMap["ttl"].(string); ok && s != "" {
						attrs["ttl"] = preserveTokenizerField(state, name, "ttl", s)
					}
					if _, ok := tmplMap["jwks_url"].(string); ok {
						attrs["jwks_url"] = preserveTokenizerField(state, name, "jwks_url", "")
					}
					if s, ok := tmplMap["claims_mapper_url"].(string); ok && s != "" {
						if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
							attrs["claims_mapper_url"] = preserveTokenizerField(state, name, "claims_mapper_url", s)
						} else {
							attrs["claims_mapper_url"] = types.StringValue(s)
						}
					}
					if s, ok := tmplMap["subject_source"].(string); ok && s != "" {
						attrs["subject_source"] = types.StringValue(s)
					}
					objVal, diags := types.ObjectValue(tokenizerTemplateAttrTypes, attrs)
					if !diags.HasError() {
						templateObjects[name] = objVal
					}
				}
				mapVal, diags := types.MapValue(types.ObjectType{AttrTypes: tokenizerTemplateAttrTypes}, templateObjects)
				if !diags.HasError() {
					state.SessionTokenizerTemplates = mapVal
				}
			}
		}

		// Courier HTTP Request Config
		if !state.CourierHTTPRequestConfig.IsNull() {
			if v := getNestedValue(identityConfig, "courier", "http", "request_config"); v != nil {
				if reqCfgRaw, ok := v.(map[string]interface{}); ok {
					objVal := readHTTPRequestConfigObject(ctx, reqCfgRaw, state.CourierHTTPRequestConfig)
					if !objVal.IsNull() {
						state.CourierHTTPRequestConfig = objVal
					}
				}
			}
		}

		// Courier Channels
		if !state.CourierChannels.IsNull() {
			if v := getNestedValue(identityConfig, "courier", "channels"); v != nil {
				if channelsRaw, ok := v.([]interface{}); ok && len(channelsRaw) > 0 {
					channelObjects := make([]attr.Value, 0, len(channelsRaw))
					for _, chRaw := range channelsRaw {
						chMap, ok := chRaw.(map[string]interface{})
						if !ok {
							continue
						}
						attrs := map[string]attr.Value{
							"id":             types.StringNull(),
							"request_config": types.ObjectNull(courierHTTPRequestConfigAttrTypes),
						}
						if id, ok := chMap["id"].(string); ok {
							attrs["id"] = types.StringValue(id)
						}
						if rc, ok := chMap["request_config"].(map[string]interface{}); ok {
							stateRC := findChannelRequestConfig(state.CourierChannels, attrs["id"])
							attrs["request_config"] = readHTTPRequestConfigObject(ctx, rc, stateRC)
						}
						objVal, diags := types.ObjectValue(courierChannelAttrTypes, attrs)
						if !diags.HasError() {
							channelObjects = append(channelObjects, objVal)
						}
					}
					listVal, diags := types.ListValue(types.ObjectType{AttrTypes: courierChannelAttrTypes}, channelObjects)
					if !diags.HasError() {
						state.CourierChannels = listVal
					}
				}
			}
		}
	}

	// Permission/Keto service config
	if project.Services.Permission != nil && !state.KetoNamespaces.IsNull() {
		permConfig := project.Services.Permission.Config
		if v := getNestedValue(permConfig, "namespaces"); v != nil {
			if nsList, ok := v.([]interface{}); ok && len(nsList) > 0 {
				names := make([]string, 0, len(nsList))
				for _, ns := range nsList {
					if nsMap, ok := ns.(map[string]interface{}); ok {
						if name, ok := nsMap["name"].(string); ok {
							names = append(names, name)
						}
					}
				}
				if len(names) > 0 {
					namesList, diags := types.ListValueFrom(ctx, types.StringType, names)
					if !diags.HasError() {
						state.KetoNamespaces = namesList
					}
				}
			}
		}
	}
}

// =============================================================================
// Helper functions for complex type reading
// =============================================================================

func preserveTokenizerField(state *ProjectConfigResourceModel, templateName, fieldName, apiValue string) basetypes.StringValue {
	if state.SessionTokenizerTemplates.IsNull() || state.SessionTokenizerTemplates.IsUnknown() {
		if apiValue != "" {
			return types.StringValue(apiValue)
		}
		return types.StringNull()
	}
	elems := state.SessionTokenizerTemplates.Elements()
	if tmplVal, ok := elems[templateName]; ok {
		if objVal, ok := tmplVal.(types.Object); ok && !objVal.IsNull() {
			attrs := objVal.Attributes()
			if v, ok := attrs[fieldName]; ok {
				if s, ok := v.(types.String); ok && !s.IsNull() {
					return s
				}
			}
		}
	}
	if apiValue != "" {
		return types.StringValue(apiValue)
	}
	return types.StringNull()
}

func readHTTPRequestConfigObject(_ context.Context, raw map[string]interface{}, stateObj basetypes.ObjectValue) basetypes.ObjectValue {
	attrs := map[string]attr.Value{
		"url":     types.StringNull(),
		"method":  types.StringNull(),
		"headers": types.MapNull(types.StringType),
		"body":    types.StringNull(),
		"auth":    types.ObjectNull(courierHTTPAuthAttrTypes),
	}

	if s, ok := raw["url"].(string); ok {
		attrs["url"] = types.StringValue(s)
	}
	if s, ok := raw["method"].(string); ok {
		attrs["method"] = types.StringValue(strings.ToUpper(s))
	}
	if s, ok := raw["body"].(string); ok && s != "" {
		if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
			attrs["body"] = getStateRequestConfigField(stateObj, "body")
		} else {
			attrs["body"] = types.StringValue(s)
		}
	}
	if hdrs, ok := raw["headers"].(map[string]interface{}); ok && len(hdrs) > 0 {
		strHdrs := make(map[string]attr.Value, len(hdrs))
		for k, v := range hdrs {
			if s, ok := v.(string); ok {
				strHdrs[k] = types.StringValue(s)
			}
		}
		mapVal, diags := types.MapValue(types.StringType, strHdrs)
		if !diags.HasError() {
			attrs["headers"] = mapVal
		}
	}
	if authRaw, ok := raw["auth"].(map[string]interface{}); ok {
		attrs["auth"] = readAuthObject(authRaw, stateObj)
	}

	objVal, diags := types.ObjectValue(courierHTTPRequestConfigAttrTypes, attrs)
	if diags.HasError() {
		return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
	}
	return objVal
}

func readAuthObject(raw map[string]interface{}, parentStateObj basetypes.ObjectValue) basetypes.ObjectValue {
	attrs := map[string]attr.Value{
		"type":     types.StringNull(),
		"user":     types.StringNull(),
		"password": types.StringNull(),
		"name":     types.StringNull(),
		"value":    types.StringNull(),
		"in":       types.StringNull(),
	}

	authType, _ := raw["type"].(string)
	if authType == "" {
		return types.ObjectNull(courierHTTPAuthAttrTypes)
	}
	attrs["type"] = types.StringValue(authType)

	config, _ := raw["config"].(map[string]interface{})
	if config == nil {
		config = map[string]interface{}{}
	}

	switch authType {
	case "basic_auth":
		if s, ok := config["user"].(string); ok {
			attrs["user"] = types.StringValue(s)
		}
		attrs["password"] = getStateAuthField(parentStateObj, "password")
	case "api_key":
		if s, ok := config["name"].(string); ok {
			attrs["name"] = types.StringValue(s)
		}
		if s, ok := config["in"].(string); ok {
			attrs["in"] = types.StringValue(s)
		}
		attrs["value"] = getStateAuthField(parentStateObj, "value")
	}

	objVal, diags := types.ObjectValue(courierHTTPAuthAttrTypes, attrs)
	if diags.HasError() {
		return types.ObjectNull(courierHTTPAuthAttrTypes)
	}
	return objVal
}

func getStateRequestConfigField(stateObj basetypes.ObjectValue, field string) basetypes.StringValue {
	if stateObj.IsNull() || stateObj.IsUnknown() {
		return types.StringNull()
	}
	attrs := stateObj.Attributes()
	if v, ok := attrs[field]; ok {
		if s, ok := v.(types.String); ok && !s.IsNull() {
			return s
		}
	}
	return types.StringNull()
}

func getStateAuthField(parentStateObj basetypes.ObjectValue, field string) basetypes.StringValue {
	if parentStateObj.IsNull() || parentStateObj.IsUnknown() {
		return types.StringNull()
	}
	parentAttrs := parentStateObj.Attributes()
	authVal, ok := parentAttrs["auth"]
	if !ok {
		return types.StringNull()
	}
	authObj, ok := authVal.(types.Object)
	if !ok || authObj.IsNull() || authObj.IsUnknown() {
		return types.StringNull()
	}
	authAttrs := authObj.Attributes()
	if v, ok := authAttrs[field]; ok {
		if s, ok := v.(types.String); ok && !s.IsNull() {
			return s
		}
	}
	return types.StringNull()
}

func findChannelRequestConfig(stateChannels basetypes.ListValue, channelID attr.Value) basetypes.ObjectValue {
	if stateChannels.IsNull() || stateChannels.IsUnknown() {
		return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
	}
	idStr, ok := channelID.(types.String)
	if !ok || idStr.IsNull() {
		return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
	}
	for _, elem := range stateChannels.Elements() {
		chObj, ok := elem.(types.Object)
		if !ok || chObj.IsNull() {
			continue
		}
		chAttrs := chObj.Attributes()
		if chID, ok := chAttrs["id"].(types.String); ok && chID.ValueString() == idStr.ValueString() {
			if rc, ok := chAttrs["request_config"].(types.Object); ok {
				return rc
			}
		}
	}
	return types.ObjectNull(courierHTTPRequestConfigAttrTypes)
}

func (r *ProjectConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ProjectConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := helpers.ResolveProjectID(plan.ProjectID, r.client.ProjectID(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	patches := r.buildPatches(ctx, &plan)
	if len(patches) > 0 {
		_, err := r.client.PatchProject(ctx, projectID, patches)
		if err != nil {
			resp.Diagnostics.AddError("Error Updating Project Config", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(projectID)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ProjectConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Config cannot be deleted - it just exists. We leave the config as-is.
}

func (r *ProjectConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID := req.ID

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)

	resp.Diagnostics.AddWarning(
		"Project Config Import - Read Your Existing Config",
		"The project config has been imported with project_id: "+projectID+".\n\n"+
			"IMPORTANT: After import, you must ensure your Terraform configuration matches the imported project:\n\n"+
			"Option 1 - Set project_id explicitly:\n"+
			"  resource \"ory_project_config\" \"main\" {\n"+
			"    project_id = \""+projectID+"\"\n"+
			"    # ... your config\n"+
			"  }\n\n"+
			"Option 2 - Use provider default:\n"+
			"  provider \"ory\" {\n"+
			"    project_id = \""+projectID+"\"\n"+
			"  }\n\n"+
			"  resource \"ory_project_config\" \"main\" {\n"+
			"    # project_id inherits from provider\n"+
			"  }\n\n"+
			"If you see 'project_id forces replacement', the project_id in your config doesn't match the imported project.")
}
