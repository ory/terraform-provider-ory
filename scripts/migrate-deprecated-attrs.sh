#!/usr/bin/env bash
# migrate-deprecated-attrs.sh
#
# Migrates Terraform configuration files from deprecated ory_project_config
# attribute names to the new spec-derived names.
#
# Usage:
#   ./scripts/migrate-deprecated-attrs.sh [directory]
#
# If no directory is specified, searches the current directory recursively.
# Creates a backup of each modified file as *.tf.bak.
#
# After running, review the changes and run:
#   terraform plan
# to verify no changes are detected.

set -euo pipefail

DIR="${1:-.}"

# Mapping of old attribute names to new names (compatible with Bash 3.2+)
# Format: "old_name=new_name"
RENAMES=(
  # OAuth2 TTLs
  "oauth2_access_token_lifespan=oauth2_ttl_access_token"
  "oauth2_refresh_token_lifespan=oauth2_ttl_refresh_token"
  "oauth2_auth_code_lifespan=oauth2_ttl_auth_code"
  "oauth2_id_token_lifespan=oauth2_ttl_id_token"
  "oauth2_login_consent_request_lifespan=oauth2_ttl_login_consent_request"

  # OAuth2 strategies
  "oauth2_access_token_strategy=oauth2_strategies_access_token"
  "oauth2_jwt_scope_claim=oauth2_strategies_jwt_scope_claim"
  "oauth2_scope_strategy=oauth2_strategies_scope"

  # OAuth2 URLs
  "oauth2_consent_url=oauth2_urls_consent"
  "oauth2_login_url=oauth2_urls_login"
  "oauth2_logout_url=oauth2_urls_logout"
  "oauth2_error_url=oauth2_urls_error"
  "oauth2_issuer_url=oauth2_urls_self_issuer"

  # OAuth2 cookies
  "oauth2_cookies_same_site_mode=oauth2_serve_cookies_same_site_mode"
  "oauth2_cookies_same_site_legacy_workaround=oauth2_serve_cookies_same_site_legacy_workaround"

  # UI URLs
  "login_ui_url=selfservice_flows_login_ui_url"
  "registration_ui_url=selfservice_flows_registration_ui_url"
  "recovery_ui_url=selfservice_flows_recovery_ui_url"
  "verification_ui_url=selfservice_flows_verification_ui_url"
  "settings_ui_url=selfservice_flows_settings_ui_url"
  "error_ui_url=selfservice_flows_error_ui_url"

  # Auth method enables
  "enable_password=selfservice_methods_password_enabled"
  "enable_code=selfservice_methods_code_enabled"
  "code_mfa_enabled=selfservice_methods_code_mfa_enabled"
  "enable_oidc=selfservice_methods_oidc_enabled"
  "enable_oidc_auto_link_policy=selfservice_methods_oidc_enable_auto_link_policy"
  "enable_totp=selfservice_methods_totp_enabled"
  "enable_webauthn=selfservice_methods_webauthn_enabled"
  "enable_passkey=selfservice_methods_passkey_enabled"
  "enable_lookup_secret=selfservice_methods_lookup_secret_enabled"
  "enable_profile=selfservice_methods_profile_enabled"

  # Code config
  "code_lifespan=selfservice_methods_code_config_lifespan"
  "code_missing_credential_fallback_enabled=selfservice_methods_code_config_missing_credential_fallback_enabled"

  # Password config
  "password_min_length=selfservice_methods_password_config_min_password_length"
  "password_check_haveibeenpwned=selfservice_methods_password_config_haveibeenpwned_enabled"
  "password_max_breaches=selfservice_methods_password_config_max_breaches"
  "password_identifier_similarity=selfservice_methods_password_config_identifier_similarity_check_enabled"

  # Flow enables/settings
  "enable_recovery=selfservice_flows_recovery_enabled"
  "enable_verification=selfservice_flows_verification_enabled"
  "enable_registration=selfservice_flows_registration_enabled"
  "login_style=selfservice_flows_login_style"
  "settings_lifespan=selfservice_flows_settings_lifespan"
  "settings_privileged_session_max_age=selfservice_flows_settings_privileged_session_max_age"
  "required_aal=selfservice_flows_settings_required_aal"
  "verification_use=selfservice_flows_verification_use"
  "verification_lifespan=selfservice_flows_verification_lifespan"
  "verification_notify_unknown_recipients=selfservice_flows_verification_notify_unknown_recipients"

  # SMTP
  "smtp_from_address=courier_smtp_from_address"
  "smtp_from_name=courier_smtp_from_name"

  # MFA / WebAuthn / TOTP
  "totp_issuer=selfservice_methods_totp_config_issuer"
  "webauthn_rp_display_name=selfservice_methods_webauthn_config_rp_display_name"
  "webauthn_rp_id=selfservice_methods_webauthn_config_rp_id"
  "webauthn_passwordless=selfservice_methods_webauthn_config_passwordless"
)

CHANGED=0

while IFS= read -r -d '' file; do
  MODIFIED=false
  for entry in "${RENAMES[@]}"; do
    old="${entry%%=*}"
    new="${entry#*=}"
    # Only match HCL attribute assignments: optional whitespace + key + optional whitespace + =
    # Use grep -F (fixed string) for the key, then verify assignment context with perl
    if grep -qF "${old}" "$file" 2>/dev/null && grep -qE "^[[:space:]]*${old}[[:space:]]*=" "$file" 2>/dev/null; then
      if [ "$MODIFIED" = false ]; then
        if [ -e "${file}.bak" ]; then
          echo "Error: Backup file '${file}.bak' already exists; refusing to overwrite." >&2
          echo "Remove existing .bak files and re-run." >&2
          exit 1
        fi
        cp "$file" "${file}.bak"
        MODIFIED=true
      fi
      # Replace attribute name only inside resource "ory_project_config" blocks.
      # Tracks brace depth; handles opening brace on same or next line.
      perl -pi -e '
        BEGIN { $in_block = 0; $depth = 0; $seen_open = 0; }
        if (!$in_block && /resource\s+"ory_project_config"/) { $in_block = 1; $depth = 0; $seen_open = 0; }
        if ($in_block) {
          my $opens = () = /\{/g;
          my $closes = () = /\}/g;
          $depth += $opens - $closes;
          $seen_open = 1 if $opens > 0;
          if ($seen_open && $depth <= 0) { $in_block = 0; $depth = 0; $seen_open = 0; }
        }
        if ($in_block) { s/^(\s*)\Q'"${old}"'\E(\s*=)/${1}'"${new}"'${2}/; }
      ' "$file"
      echo "  $file: $old -> $new"
    fi
  done
  if [ "$MODIFIED" = true ]; then
    CHANGED=$((CHANGED + 1))
  fi
done < <(find "$DIR" -type f -name '*.tf' -not -path '*/.terraform/*' -print0)

echo ""
echo "Migration complete. $CHANGED file(s) updated."
echo "Review the changes and run 'terraform plan' to verify."
echo "Backups saved as *.tf.bak"
