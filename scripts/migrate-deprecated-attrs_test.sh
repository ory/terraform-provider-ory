#!/usr/bin/env bash
# migrate-deprecated-attrs_test.sh
#
# Integration tests for migrate-deprecated-attrs.sh. Exercises the script
# against temporary fixture directories and verifies it renames the
# attributes users actually hit in real project config files.
#
# Run:
#   ./scripts/migrate-deprecated-attrs_test.sh
# or:
#   make test-migrate-script
#
# Exits non-zero on any failing assertion.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATE_SCRIPT="${SCRIPT_DIR}/migrate-deprecated-attrs.sh"

if [ ! -x "$MIGRATE_SCRIPT" ]; then
  echo "FAIL: migration script not executable at $MIGRATE_SCRIPT" >&2
  exit 1
fi

PASS=0
FAIL=0
TMPROOT="$(mktemp -d)"
trap 'rm -rf "$TMPROOT"' EXIT

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1" >&2; FAIL=$((FAIL + 1)); }

assert_contains() {
  local file="$1" needle="$2" desc="$3"
  if grep -qF "$needle" "$file"; then
    pass "$desc"
  else
    fail "$desc — expected to find '$needle' in $file"
    echo "--- $file ---" >&2
    cat "$file" >&2
  fi
}

assert_not_contains() {
  local file="$1" needle="$2" desc="$3"
  if grep -qF "$needle" "$file"; then
    fail "$desc — did not expect to find '$needle' in $file"
    echo "--- $file ---" >&2
    cat "$file" >&2
  else
    pass "$desc"
  fi
}

# ------------------------------------------------------------------------------
# Test 1: renames deprecated attrs inside ory_project_config
# ------------------------------------------------------------------------------
echo "Test 1: renames deprecated attrs inside ory_project_config"
t1="$TMPROOT/test1"
mkdir -p "$t1"
cat > "$t1/project.tf" <<'EOF'
resource "ory_project_config" "main" {
  oauth2_login_url    = "https://example.com/login"
  oauth2_consent_url  = "https://example.com/consent"
  oauth2_issuer_url   = "https://example.com/"
  login_ui_url        = "https://example.com/ui/login"
  registration_ui_url = "https://example.com/ui/register"
  error_ui_url        = "https://example.com/ui/error"
  settings_ui_url     = "https://example.com/ui/settings"
  enable_password     = true
  enable_oidc         = true
}
EOF
"$MIGRATE_SCRIPT" "$t1" >"$t1/out.log" 2>&1 || { fail "script exited non-zero"; cat "$t1/out.log" >&2; }
assert_contains "$t1/project.tf"   "oauth2_urls_login"                          "renames oauth2_login_url"
assert_contains "$t1/project.tf"   "oauth2_urls_consent"                        "renames oauth2_consent_url"
assert_contains "$t1/project.tf"   "oauth2_urls_self_issuer"                    "renames oauth2_issuer_url"
assert_contains "$t1/project.tf"   "selfservice_flows_login_ui_url"             "renames login_ui_url"
assert_contains "$t1/project.tf"   "selfservice_flows_registration_ui_url"      "renames registration_ui_url"
assert_contains "$t1/project.tf"   "selfservice_flows_error_ui_url"             "renames error_ui_url"
assert_contains "$t1/project.tf"   "selfservice_flows_settings_ui_url"          "renames settings_ui_url"
assert_contains "$t1/project.tf"   "selfservice_methods_password_enabled"       "renames enable_password"
assert_contains "$t1/project.tf"   "selfservice_methods_oidc_enabled"           "renames enable_oidc"
assert_contains "$t1/out.log"      "Migration complete. 1 file(s) updated."    "reports 1 file updated"
assert_contains "$t1/project.tf.bak" "oauth2_login_url"                         "backup preserves original attribute name"

# ------------------------------------------------------------------------------
# Test 2: does not rename deprecated attrs outside ory_project_config blocks
# ------------------------------------------------------------------------------
echo "Test 2: does not rename deprecated attrs outside ory_project_config blocks"
t2="$TMPROOT/test2"
mkdir -p "$t2"
cat > "$t2/other.tf" <<'EOF'
resource "some_other_resource" "main" {
  login_ui_url = "https://example.com/ui/login"
}
EOF
"$MIGRATE_SCRIPT" "$t2" >"$t2/out.log" 2>&1 || true
assert_contains     "$t2/other.tf" "login_ui_url"                       "leaves attr outside ory_project_config untouched"
assert_not_contains "$t2/other.tf" "selfservice_flows_login_ui_url"     "does not inject new name outside ory_project_config"

# ------------------------------------------------------------------------------
# Test 3: warns when both old and new names are present
# ------------------------------------------------------------------------------
echo "Test 3: warns when both old and new names are present"
t3="$TMPROOT/test3"
mkdir -p "$t3"
cat > "$t3/duplicate.tf" <<'EOF'
resource "ory_project_config" "main" {
  login_ui_url                   = "https://example.com/old"
  selfservice_flows_login_ui_url = "https://example.com/new"
}
EOF
"$MIGRATE_SCRIPT" "$t3" >"$t3/out.log" 2>&1 || true
assert_contains "$t3/out.log"      "both 'login_ui_url' and 'selfservice_flows_login_ui_url' exist"  "reports duplicate"
assert_contains "$t3/duplicate.tf" "login_ui_url"                       "leaves old name in place when duplicate"
assert_contains "$t3/duplicate.tf" "selfservice_flows_login_ui_url"     "leaves new name in place when duplicate"

# ------------------------------------------------------------------------------
# Test 4: warns for removed attributes but does not rename them
# ------------------------------------------------------------------------------
echo "Test 4: warns for removed attributes"
t4="$TMPROOT/test4"
mkdir -p "$t4"
cat > "$t4/removed.tf" <<'EOF'
resource "ory_project_config" "main" {
  account_experience_name        = "My App"
  account_experience_stylesheet  = "https://example.com/style.css"
  oauth2_session_encrypt_at_rest = true
}
EOF
"$MIGRATE_SCRIPT" "$t4" >"$t4/out.log" 2>&1 || true
assert_contains "$t4/out.log"   "following attributes have been removed"          "emits removed-attrs warning"
assert_contains "$t4/out.log"   "account_experience_name"                         "lists account_experience_name"
assert_contains "$t4/out.log"   "oauth2_session_encrypt_at_rest"                  "lists oauth2_session_encrypt_at_rest"
assert_contains "$t4/removed.tf" "account_experience_name"                        "removed attributes are not auto-deleted"

# ------------------------------------------------------------------------------
# Test 5: warns about base_redirect_uri on ory_social_provider
# ------------------------------------------------------------------------------
echo "Test 5: warns about cross-resource base_redirect_uri"
t5="$TMPROOT/test5"
mkdir -p "$t5"
cat > "$t5/social.tf" <<'EOF'
resource "ory_social_provider" "github" {
  provider          = "github"
  base_redirect_uri = "https://example.com"
}
EOF
"$MIGRATE_SCRIPT" "$t5" >"$t5/out.log" 2>&1 || true
assert_contains     "$t5/out.log"   "base_redirect_uri on ory_social_provider"                    "warns about cross-resource attr"
assert_contains     "$t5/out.log"   "selfservice_methods_oidc_config_base_redirect_uri"           "suggests correct new attribute"
assert_not_contains "$t5/social.tf" "selfservice_methods_oidc_config_base_redirect_uri"           "does not silently move cross-resource attr"

# ------------------------------------------------------------------------------
# Test 6: refuses to overwrite an existing .bak file
# ------------------------------------------------------------------------------
echo "Test 6: refuses to overwrite existing .bak"
t6="$TMPROOT/test6"
mkdir -p "$t6"
cat > "$t6/project.tf" <<'EOF'
resource "ory_project_config" "main" {
  login_ui_url = "https://example.com/ui/login"
}
EOF
cp "$t6/project.tf" "$t6/project.tf.bak"
set +e
"$MIGRATE_SCRIPT" "$t6" >"$t6/out.log" 2>&1
rc=$?
set -e
if [ "$rc" -ne 0 ]; then
  pass "exits non-zero when .bak already exists"
else
  fail "expected non-zero exit when .bak already exists, got $rc"
fi
assert_contains "$t6/out.log" "Backup file" "reports backup collision"

# ------------------------------------------------------------------------------
# Test 7: no deprecated attrs → no changes, 0 files updated
# ------------------------------------------------------------------------------
echo "Test 7: file without deprecated attrs is untouched"
t7="$TMPROOT/test7"
mkdir -p "$t7"
cat > "$t7/clean.tf" <<'EOF'
resource "ory_project_config" "main" {
  selfservice_flows_login_ui_url = "https://example.com/ui/login"
}
EOF
cp "$t7/clean.tf" "$t7/clean.tf.orig"
"$MIGRATE_SCRIPT" "$t7" >"$t7/out.log" 2>&1 || true
if cmp -s "$t7/clean.tf" "$t7/clean.tf.orig"; then
  pass "already-migrated file is unchanged"
else
  fail "already-migrated file was modified"
fi
assert_contains "$t7/out.log" "Migration complete. 0 file(s) updated." "reports 0 files updated"
if [ -e "$t7/clean.tf.bak" ]; then
  fail "no .bak should be created when nothing changes"
else
  pass "no .bak created when nothing changes"
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
echo ""
echo "Passed: $PASS"
echo "Failed: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
