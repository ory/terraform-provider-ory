#!/usr/bin/env bash
#
# generate-imports.sh — inventory an existing Ory Network project and emit
# Terraform import blocks for every importable resource it contains.
#
# Usage:
#   export ORY_WORKSPACE_API_KEY=ory_wak_...   # required
#   export ORY_PROJECT_ID=<project-uuid>       # required
#   export ORY_PROJECT_API_KEY=ory_pat_...     # optional: OAuth2 clients, JWKS,
#                                              #   trusted issuers, identity count
#   export ORY_JWKS_SETS=my-set,other-set      # optional: JWKS set IDs to import
#   ./generate-imports.sh > imports.tf
#
# Endpoint overrides (self-explanatory defaults for Ory Network production):
#   ORY_CONSOLE_API_URL   default https://api.console.ory.sh
#   ORY_PROJECT_API_URL   printf template, default https://%s.projects.oryapis.com
#
# Import blocks are written to stdout, progress and warnings to stderr.
# Resources that hold data rather than configuration (identities, relationship
# tuples) and resources whose secrets cannot be recovered (project API keys)
# are emitted as comments with an explanation instead of import blocks.

set -euo pipefail

CONSOLE_URL="${ORY_CONSOLE_API_URL:-https://api.console.ory.sh}"
PROJECT_URL_TEMPLATE="${ORY_PROJECT_API_URL:-https://%s.projects.oryapis.com}"

log() { echo "$*" >&2; }
die() {
  log "ERROR: $*"
  exit 1
}

command -v curl >/dev/null || die "curl is required"
command -v jq >/dev/null || die "jq is required"
[ -n "${ORY_WORKSPACE_API_KEY:-}" ] || die "ORY_WORKSPACE_API_KEY is required"
[ -n "${ORY_PROJECT_ID:-}" ] || die "ORY_PROJECT_ID is required"

# Per-run scratch state. This lives in files rather than shell variables because
# nearly every caller reads a value through $( ) or from inside a `... | while
# read` pipeline, both of which run in a subshell: a variable assigned in there
# never reaches the caller. With variables, pagination stops after the first page
# and the label dedupe never sees anything as taken.
RUN_DIR=$(mktemp -d)
HEADERS_FILE="$RUN_DIR/headers"
ERROR_FILE="$RUN_DIR/error"
LABELS_FILE="$RUN_DIR/labels"
: >"$HEADERS_FILE"
: >"$ERROR_FILE"
: >"$LABELS_FILE"
cleanup() { rm -rf "$RUN_DIR"; }
trap cleanup EXIT

http_error() { cat "$ERROR_FILE"; }

# GET with the given bearer token. Prints the body on 2xx and returns 1 on any
# other status, so callers decide whether the failure is fatal. curl exits 0 on
# a 4xx or 5xx, so the status has to be captured explicitly.
http_get() {
  local url="$1" token="$2" body status reason
  : >"$ERROR_FILE"
  body=$(curl -sS -D "$HEADERS_FILE" -w '\n%{http_code}' \
    -H "Authorization: Bearer $token" "$url" 2>/dev/null) || {
    echo "network error" >"$ERROR_FILE"
    return 1
  }
  status="${body##*$'\n'}"
  body="${body%$'\n'*}"
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    reason=$(echo "$body" | jq -r '.error.message? // .error? // empty' 2>/dev/null || true)
    if [ -n "$reason" ]; then
      echo "HTTP $status: $reason" >"$ERROR_FILE"
    else
      echo "HTTP $status" >"$ERROR_FILE"
    fi
    return 1
  fi
  echo "$body"
}

# GET from the Ory Console API (workspace API key). Dies on any failure. Use
# only for requests the whole inventory depends on.
console_get() {
  local path="$1"
  http_get "$CONSOLE_URL$path" "$ORY_WORKSPACE_API_KEY" ||
    die "GET $path failed ($(http_error))"
}

# GET from the Ory Console API, tolerating failure. Plan-gated features answer
# with 403 feature_not_available and environment-gated ones with 400 or 403, so
# a hard failure here would abort the run on a project that simply does not have
# custom domains, event streams, or organizations.
console_get_optional() {
  local path="$1" body
  if body=$(http_get "$CONSOLE_URL$path" "$ORY_WORKSPACE_API_KEY"); then
    echo "$body"
    return 0
  fi
  log "WARN: GET $path unavailable ($(http_error)); skipping this section"
  return 1
}

# GET from the project admin API (project API key). Prints nothing on error so
# optional sections can degrade gracefully.
admin_get() {
  local path="$1" body
  if body=$(http_get "$ADMIN_URL$path" "$ORY_PROJECT_API_KEY"); then
    echo "$body"
    return 0
  fi
  log "WARN: GET $path: $(http_error)"
  return 1
}

# Extract the page_token of the "next" relation from the last response's Link
# header. Ory paginates its admin collections with an opaque cursor, the same way
# parseLinkNextPageToken does in internal/client/client.go.
next_page_token() {
  tr -d '\r' <"$HEADERS_FILE" | grep -i '^link:' |
    tr ',' '\n' | grep 'rel="next"' |
    sed -n 's/.*[?&]page_token=\([^&>]*\).*/\1/p' | head -1
}

# GET every page of an admin collection and print the concatenated JSON array.
# Bounded so a server that keeps returning a cursor cannot loop forever.
admin_get_all() {
  local path="$1" sep="?" page all token
  case "$path" in *\?*) sep="&" ;; esac
  all="[]"
  token=""
  local i
  for i in $(seq 1 50); do
    local url="$path"
    [ -n "$token" ] && url="$path${sep}page_token=$token"
    page=$(admin_get "$url") || return 1
    echo "$page" | jq -e 'type == "array"' >/dev/null 2>&1 || {
      log "WARN: GET $path returned $(echo "$page" | jq -r 'type'), expected an array"
      return 1
    }
    all=$(jq -c -n --argjson a "$all" --argjson b "$page" '$a + $b')
    [ "$(echo "$page" | jq 'length')" -eq 0 ] && break
    token=$(next_page_token)
    [ -z "$token" ] && break
    if [ "$i" -eq 50 ]; then
      log "WARN: GET $path stopped after 50 pages; results may be truncated"
    fi
  done
  echo "$all"
}

# Turn an arbitrary string into a valid Terraform resource label fragment.
sanitize() {
  echo "$1" | tr '[:upper:]' '[:lower:]' | sed -e 's/[^a-z0-9]\{1,\}/_/g' -e 's/^_//' -e 's/_$//' |
    sed -e 's/^$/x/' -e 's/^\([0-9]\)/x\1/'
}

short_id() { echo "$1" | cut -c1-8; }

# Terraform addresses must be unique, but sanitize collapses every run of
# non-alphanumerics to "_", so "google-1" and "google_1" both become "google_1".
# Record what has been emitted and suffix a counter on a repeat, otherwise
# Terraform rejects the whole file for duplicate import blocks. The record has to
# be a file: callers invoke this from inside $( ), so a variable would reset on
# every call and never report a label as taken.
unique_label() {
  local base="$1" candidate="$1" n=2
  while grep -qxF "$candidate" "$LABELS_FILE"; do
    candidate="${base}_${n}"
    n=$((n + 1))
  done
  printf '%s\n' "$candidate" >>"$LABELS_FILE"
  echo "$candidate"
}

# HCL escaping for an import ID. Action IDs embed arbitrary webhook URLs, so a
# quote or backslash in one would otherwise produce a broken file.
hcl_escape() {
  printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

emit() {
  printf 'import {\n  to = %s\n  id = "%s"\n}\n\n' "$1" "$(hcl_escape "$2")"
}

log "Inventorying project $ORY_PROJECT_ID ..."
PROJECT_JSON=$(console_get "/projects/$ORY_PROJECT_ID")
PROJECT_NAME=$(echo "$PROJECT_JSON" | jq -r '.name // empty')
PROJECT_SLUG=$(echo "$PROJECT_JSON" | jq -r '.slug // empty')
PROJECT_STATE=$(echo "$PROJECT_JSON" | jq -r '.state // empty')
WORKSPACE_ID=$(echo "$PROJECT_JSON" | jq -r '.workspace_id // empty')
[ -n "$PROJECT_SLUG" ] ||
  die "project $ORY_PROJECT_ID has no slug; check ORY_PROJECT_ID and ORY_WORKSPACE_API_KEY"

# Projects are soft deleted: GET returns HTTP 200 with state "deleted". Every
# import block would then fail with "Cannot import non-existent remote object"
# even though each request succeeded, so stop before emitting any.
if [ "$PROJECT_STATE" = "deleted" ]; then
  die "project $ORY_PROJECT_ID is soft deleted (state: deleted); nothing can be imported from it"
fi

# shellcheck disable=SC2059 # PROJECT_URL_TEMPLATE is intentionally a printf template
ADMIN_URL=$(printf "$PROJECT_URL_TEMPLATE" "$PROJECT_SLUG")

cat <<EOF
# Terraform import blocks for Ory Network project:
#   name: ${PROJECT_NAME:-<unnamed>}
#   id:   $ORY_PROJECT_ID
#   slug: $PROJECT_SLUG
#
# Generated by generate-imports.sh. Review before use: remove blocks for
# resources you do not want Terraform to manage, then run
#   terraform plan -generate-config-out=generated.tf
# to generate matching resource configuration.

EOF

# --- Project and project config (always present) -----------------------------
emit "ory_project.main" "$ORY_PROJECT_ID"
emit "ory_project_config.main" "$ORY_PROJECT_ID"

if [ -n "$WORKSPACE_ID" ]; then
  cat <<EOF
# The project belongs to workspace $WORKSPACE_ID. Uncomment to also manage the
# workspace itself (name only; deleting the resource deletes the workspace).
# import {
#   to = ory_workspace.main
#   id = "$WORKSPACE_ID"
# }

EOF
fi

# --- Custom domains -----------------------------------------------------------
DOMAINS="[]"
if RESP=$(console_get_optional "/projects/$ORY_PROJECT_ID/cname"); then
  DOMAINS=$(echo "$RESP" | jq -c 'if type == "array" then . else [] end')
fi
echo "$DOMAINS" | jq -c '.[]' | while read -r row; do
  id=$(echo "$row" | jq -r '.id // empty')
  hostname=$(echo "$row" | jq -r '.hostname // empty')
  [ -n "$id" ] || continue
  emit "ory_custom_domain.$(unique_label "$(sanitize "${hostname:-domain}")")" "$ORY_PROJECT_ID/$id"
done
log "custom domains: $(echo "$DOMAINS" | jq 'length')"

# --- Event streams ------------------------------------------------------------
STREAMS="[]"
if RESP=$(console_get_optional "/projects/$ORY_PROJECT_ID/eventstreams"); then
  STREAMS=$(echo "$RESP" | jq -c '.event_streams // [] | if type == "array" then . else [] end')
fi
echo "$STREAMS" | jq -c '.[]' | while read -r row; do
  id=$(echo "$row" | jq -r '.id // empty')
  type=$(echo "$row" | jq -r '.type // "stream"')
  [ -n "$id" ] || continue
  emit "ory_event_stream.$(unique_label "$(sanitize "$type")_$(short_id "$id")")" "$ORY_PROJECT_ID/$id"
done
log "event streams: $(echo "$STREAMS" | jq 'length')"

# --- Organizations (B2B) --------------------------------------------------------
# Paginated with an explicit next_page_token rather than a Link header.
ORGS="[]"
ORG_PAGE_TOKEN=""
for _ in $(seq 1 50); do
  ORG_PATH="/projects/$ORY_PROJECT_ID/organizations"
  [ -n "$ORG_PAGE_TOKEN" ] && ORG_PATH="$ORG_PATH?page_token=$ORG_PAGE_TOKEN"
  ORGS_RESP=$(console_get_optional "$ORG_PATH") || break
  ORGS=$(jq -c -n --argjson a "$ORGS" \
    --argjson b "$(echo "$ORGS_RESP" | jq -c '.organizations // [] | if type == "array" then . else [] end')" \
    '$a + $b')
  [ "$(echo "$ORGS_RESP" | jq -r '.has_next_page // false')" = "true" ] || break
  ORG_PAGE_TOKEN=$(echo "$ORGS_RESP" | jq -r '.next_page_token // empty')
  [ -n "$ORG_PAGE_TOKEN" ] || break
done
echo "$ORGS" | jq -c '.[]' | while read -r row; do
  id=$(echo "$row" | jq -r '.id // empty')
  label=$(echo "$row" | jq -r '.label // empty')
  [ -n "$id" ] || continue
  emit "ory_organization.$(unique_label "$(sanitize "${label:-org}")_$(short_id "$id")")" "$ORY_PROJECT_ID/$id"
done
log "organizations: $(echo "$ORGS" | jq 'length')"

# --- Project API keys (comment only: key values are unrecoverable) -------------
KEYS="[]"
if RESP=$(console_get_optional "/projects/$ORY_PROJECT_ID/tokens"); then
  KEYS=$(echo "$RESP" | jq -c 'if type == "array" then . else [] end')
fi
if [ "$(echo "$KEYS" | jq 'length')" -gt 0 ]; then
  echo "# Project API keys exist but are not imported by default: the secret value"
  echo "# is only returned at creation time and cannot be recovered by an import."
  echo "# Uncomment to import one anyway (state will hold a null value):"
  echo "$KEYS" | jq -c '.[]' | while read -r row; do
    id=$(echo "$row" | jq -r '.id // empty')
    name=$(echo "$row" | jq -r '.name // "key"')
    [ -n "$id" ] || continue
    printf '# import {\n#   to = ory_project_api_key.%s\n#   id = "%s"\n# }\n' \
      "$(unique_label "$(sanitize "$name")_$(short_id "$id")")" "$ORY_PROJECT_ID/$id"
  done
  echo
fi
log "project API keys: $(echo "$KEYS" | jq 'length') (commented out)"

# --- Social sign-in providers (from the project revision) ----------------------
SOCIAL=$(echo "$PROJECT_JSON" | jq -c '
  [.services.identity.config.selfservice.methods.oidc.config.providers[]?
   | select(type == "object")]')
if [ "$(echo "$SOCIAL" | jq 'length')" -gt 0 ]; then
  echo "# Social provider client secrets are never returned by the API. After"
  echo "# generating config, re-supply each client_secret (or client_secret_wo)."
  if [ "$(echo "$SOCIAL" | jq '[.[] | select((.organization_id // "") != "")] | length')" -gt 0 ]; then
    echo "# Providers with an organization_id are B2B SSO providers. Import the"
    echo "# matching ory_organization too and reference it as"
    echo "# organization_id = ory_organization.<name>.id so Terraform orders the"
    echo "# create and the destroy correctly."
  fi
fi
echo "$SOCIAL" | jq -c '.[]' | while read -r row; do
  id=$(echo "$row" | jq -r '.id // empty')
  [ -n "$id" ] || continue
  emit "ory_social_provider.$(unique_label "$(sanitize "$id")")" "$id"
done
log "social providers: $(echo "$SOCIAL" | jq 'length')"

# --- SAML providers (from the project revision) ---------------------------------
SAML=$(echo "$PROJECT_JSON" | jq -c '
  [.services.identity.config.selfservice.methods.saml.config.providers[]?
   | select(type == "object")]')
echo "$SAML" | jq -c '.[]' | while read -r row; do
  id=$(echo "$row" | jq -r '.id // empty')
  [ -n "$id" ] || continue
  emit "ory_saml_provider.$(unique_label "$(sanitize "$id")")" "$id"
done
log "saml providers: $(echo "$SAML" | jq 'length')"

# --- Actions / webhooks (from the project revision) -----------------------------
# after-timing hooks nested under an auth method: flow:after:<method>:METHOD:url
# after-timing hooks in a flat array: auth method placeholder "_". Only valid on
# a flow that does not scope its hooks by authentication method. For login,
# registration, and settings the provider resolves "_" to the default "password"
# and then reads .../after/password/hooks, so a flat hook there is unreachable.
# Mirrors authMethodsForFlow in internal/resources/action/resource.go.
# before-timing hooks: flow:before:METHOD:url
#
# Every level is type-checked. The "after" node holds scalar siblings of the
# auth-method objects, such as default_browser_return_url and required_aal, and
# indexing one of those aborts jq and truncates the whole file.
# shellcheck disable=SC2016 # jq program: $flow and $timing are jq variables
ACTIONS_JQ='
  def hooks_of($flow; $timing; $method):
    (if type == "array" then . else [] end)
    | map(select(type == "object" and .hook == "web_hook"
                 and (.config | type) == "object"
                 and (.config.url | type) == "string")
        | {label: ($flow + "_" + $timing + (if $method == "" then "" else "_" + $method end)),
           id: ($flow + ":" + $timing
                + (if $timing == "after" then ":" + (if $method == "" then "_" else $method end) else "" end)
                + ":" + (.config.method // "POST") + ":" + .config.url)});
  def flat_allowed($flow): ($flow | IN("login", "registration", "settings")) | not;
  (.services.identity.config.selfservice.flows // {})
  | (if type == "object" then . else {} end)
  | to_entries
  | map(.key as $flow
      | (if (.value | type) == "object" then .value else {} end) as $cfg
      | (
          (if ($cfg.before | type) == "object" then ($cfg.before.hooks | hooks_of($flow; "before"; "")) else [] end)
          + (
            (if ($cfg.after | type) == "object" then $cfg.after else {} end)
            | to_entries
            | map(.key as $m
                | if $m == "hooks" then
                    (if flat_allowed($flow) then (.value | hooks_of($flow; "after"; "")) else [] end)
                  elif (.value | type) == "object" then
                    (.value.hooks | hooks_of($flow; "after"; $m))
                  else
                    []
                  end)
            | add // [])
        ))
  | add // []'
ACTIONS=$(echo "$PROJECT_JSON" | jq -c "$ACTIONS_JQ")

# Flat "after" hooks on a flow that scopes hooks by auth method are unreachable
# for ory_action. Emitting an import block for one produces "Cannot import
# non-existent remote object", so report them instead.
UNREACHABLE=$(echo "$PROJECT_JSON" | jq -c '
  (.services.identity.config.selfservice.flows // {})
  | (if type == "object" then . else {} end)
  | to_entries
  | map(select(.key | IN("login", "registration", "settings"))
      | .key as $flow
      | (if (.value | type) == "object" and (.value.after | type) == "object"
         then (.value.after.hooks // []) else [] end)
      | (if type == "array" then . else [] end)
      | map(select(type == "object" and .hook == "web_hook")
          | "\($flow): \(.config.url // "?")"))
  | add // []')
if [ "$(echo "$UNREACHABLE" | jq 'length')" -gt 0 ]; then
  echo "# WARNING: these webhooks sit in a flat .../<flow>/after/hooks array on a"
  echo "# flow whose hooks the provider scopes by authentication method. ory_action"
  echo "# always reads .../<flow>/after/<auth_method>/hooks for these flows, so the"
  echo "# hooks below cannot be imported. Move each one under an auth method in the"
  echo "# Ory Console, then re-run this script."
  echo "$UNREACHABLE" | jq -r '.[] | "#   \(.)"'
  echo
  log "WARN: $(echo "$UNREACHABLE" | jq 'length') flat after-hook(s) on an auth-method flow cannot be imported"
fi

n=0
while read -r row; do
  [ -n "$row" ] || continue
  n=$((n + 1))
  label=$(echo "$row" | jq -r '.label')
  id=$(echo "$row" | jq -r '.id')
  emit "ory_action.$(unique_label "$(sanitize "$label")_$n")" "$ORY_PROJECT_ID:$id"
done < <(echo "$ACTIONS" | jq -c '.[]')
log "actions (web_hooks): $(echo "$ACTIONS" | jq 'length')"

# --- Email templates (from the project revision) --------------------------------
# Only templates with actual content (subject or body) are customized; the rest
# are empty skeletons for the defaults and cannot be imported meaningfully.
# The subject and body live one level below the validity key, under ".email".
# ory_email_template accepts a closed set of template types, so an unexpected
# group in the revision is reported rather than emitted as a doomed import.
VALID_TEMPLATE_TYPES='["recovery_code_valid","recovery_code_invalid","recovery_valid","recovery_invalid","verification_code_valid","verification_code_invalid","verification_valid","verification_invalid","login_code_valid","login_code_invalid","registration_code_valid","registration_code_invalid"]'
TEMPLATES_ALL=$(echo "$PROJECT_JSON" | jq -c '
  [(.services.identity.config.courier.templates // {})
   | (if type == "object" then . else {} end)
   | to_entries[]
   | .key as $base
   | (if (.value | type) == "object" then .value else {} end)
   | to_entries[]
   | select(.key == "valid" or .key == "invalid")
   | .key as $validity
   | (if (.value | type) == "object" then (.value.email // {}) else {} end)
   | (if type == "object" then . else {} end)
   | select(((.subject // "") | tostring) != ""
            or ((.body.html? // "") | tostring) != ""
            or ((.body.plaintext? // "") | tostring) != "")
   | "\($base)_\($validity)"]')
TEMPLATES=$(jq -c -n --argjson all "$TEMPLATES_ALL" --argjson valid "$VALID_TEMPLATE_TYPES" \
  '[$all[] | select(. as $t | $valid | index($t))]')
UNKNOWN_TEMPLATES=$(jq -c -n --argjson all "$TEMPLATES_ALL" --argjson valid "$VALID_TEMPLATE_TYPES" \
  '[$all[] | select(. as $t | $valid | index($t) | not)]')
echo "$TEMPLATES" | jq -r '.[]' | while read -r type; do
  [ -n "$type" ] || continue
  emit "ory_email_template.$(unique_label "$(sanitize "$type")")" "$type"
done
if [ "$(echo "$UNKNOWN_TEMPLATES" | jq 'length')" -gt 0 ]; then
  echo "# These courier templates hold content but are not among the template types"
  echo "# ory_email_template accepts, so no import block was emitted for them:"
  echo "$UNKNOWN_TEMPLATES" | jq -r '.[] | "#   \(.)"'
  echo
fi
log "email templates with custom content: $(echo "$TEMPLATES" | jq 'length')"

# --- Identity schemas: not importable, list them for reference -------------------
# An entry without a string id would abort startswith(), and the project revision
# lists only schemas explicitly added to the project. The workspace endpoint holds
# the rest, which is why ory_identity_schemas merges both sources.
SCHEMAS=$(echo "$PROJECT_JSON" | jq -c '
  [.services.identity.config.identity.schemas[]?
   | select(type == "object") | .id
   | select(type == "string")
   | select(startswith("preset://") | not)]')
if WS_SCHEMAS_RESP=$(console_get_optional "/identity-schemas"); then
  SCHEMAS=$(jq -c -n --argjson a "$SCHEMAS" \
    --argjson b "$(echo "$WS_SCHEMAS_RESP" | jq -c '
      [(if type == "array" then . else (.identity_schemas // []) end)[]?
       | select(type == "object") | (.id // .schema_id)
       | select(type == "string")
       | select(startswith("preset://") | not)]')" \
    '($a + $b) | unique')
fi
if [ "$(echo "$SCHEMAS" | jq 'length')" -gt 0 ]; then
  echo "# Identity schemas are immutable and intentionally NOT importable."
  echo "# Existing schemas stay as they are; manage future schemas with new"
  echo "# ory_identity_schema resources, or read these with the ory_identity_schema"
  echo "# and ory_identity_schemas data sources:"
  echo "$SCHEMAS" | jq -r '.[] | "#   schema id: \(.)"'
  echo
fi
log "identity schemas (not importable): $(echo "$SCHEMAS" | jq 'length')"

# --- Keto namespaces / relationships ---------------------------------------------
# namespaces is either an array of {name, id} objects or an object holding a
# "location" URL that points at a namespace config file. Indexing the object
# shape as an array aborts jq, so handle both.
NAMESPACES=$(echo "$PROJECT_JSON" | jq -c '
  (.services.permission.config.namespaces // [])
  | if type == "array" then [.[] | select(type == "object") | .name | select(type == "string")]
    else [] end')
NAMESPACE_LOCATION=$(echo "$PROJECT_JSON" | jq -r '
  (.services.permission.config.namespaces // {})
  | if type == "object" then (.location // empty) else empty end')
if [ "$(echo "$NAMESPACES" | jq 'length')" -gt 0 ] || [ -n "$NAMESPACE_LOCATION" ]; then
  echo "# Keto namespaces (managed inside ory_project_config, not separate resources):"
  if [ -n "$NAMESPACE_LOCATION" ]; then
    echo "#   configured from a location URL: $NAMESPACE_LOCATION"
    echo "#   set keto_namespace_configuration in ory_project_config to manage it."
  else
    echo "$NAMESPACES" | jq -r '.[] | "#   \(.)"'
  fi
  echo "# Relationship tuples are data, not configuration; import selectively with an"
  echo "# ory_relationship import block whose id is \"namespace:object#relation@subject\"."
  echo "# List tuples via GET /relation-tuples?namespace=<name> on the project API."
  echo
fi
log "keto namespaces: $(echo "$NAMESPACES" | jq 'length')${NAMESPACE_LOCATION:+ (location URL)}"

# --- Project-API-key gated resources ---------------------------------------------
if [ -n "${ORY_PROJECT_API_KEY:-}" ]; then
  # OAuth2 clients
  if CLIENTS=$(admin_get_all "/admin/clients?page_size=250"); then
    COUNT=$(echo "$CLIENTS" | jq 'length')
    if [ "$COUNT" -gt 0 ]; then
      echo "# OAuth2 client secrets are only returned at creation time. After"
      echo "# generating config, re-supply client_secret (or client_secret_wo) for"
      echo "# confidential clients."
      echo "#"
      echo "# GET /admin/clients returns dynamically registered (RFC 7591) clients"
      echo "# alongside normal ones and carries no field that distinguishes them, so"
      echo "# every client below is emitted as ory_oauth2_client. For a client you"
      echo "# registered through /oauth2/register, change the resource type on the"
      echo "# 'to' line to ory_oidc_dynamic_client; both take the same {client_id}"
      echo "# import ID, so nothing else changes."
    fi
    echo "$CLIENTS" | jq -c '.[]' | while read -r row; do
      id=$(echo "$row" | jq -r '.client_id // empty')
      name=$(echo "$row" | jq -r '.client_name // ""')
      [ -n "$id" ] || continue
      emit "ory_oauth2_client.$(unique_label "$(sanitize "${name:-client}")_$(short_id "$id")")" "$id"
    done
    log "oauth2 clients: $COUNT"
  fi

  # Trusted OAuth2 JWT grant issuers. The API defaults to 250 per page, so this
  # has to follow the cursor or a larger project loses issuers silently.
  if ISSUERS=$(admin_get_all "/admin/trust/grants/jwt-bearer/issuers?page_size=250"); then
    echo "$ISSUERS" | jq -c '.[]' | while read -r row; do
      id=$(echo "$row" | jq -r '.id // empty')
      issuer=$(echo "$row" | jq -r '.issuer // "issuer"')
      [ -n "$id" ] || continue
      emit "ory_trusted_oauth2_jwt_grant_issuer.$(unique_label "$(sanitize "$issuer")_$(short_id "$id")")" "$id"
    done
    log "trusted jwt grant issuers: $(echo "$ISSUERS" | jq 'length')"
  fi

  # JSON web key sets: there is no list-all endpoint, so sets must be named
  # explicitly. Default hydra.* sets are system-managed; do not import them.
  if [ -n "${ORY_JWKS_SETS:-}" ]; then
    echo "# JWKS imports put PRIVATE keys into the Terraform state. Handle with care."
    IFS=',' read -r -a JWKS_SETS <<<"$ORY_JWKS_SETS"
    for jwks_set in "${JWKS_SETS[@]}"; do
      jwks_set=$(echo "$jwks_set" | sed -e 's/^ *//' -e 's/ *$//')
      [ -z "$jwks_set" ] && continue
      # The set id becomes a path segment, so it has to be percent-encoded.
      jwks_set_encoded=$(jq -rn --arg s "$jwks_set" '$s | @uri')
      if admin_get "/admin/keys/$jwks_set_encoded" >/dev/null; then
        emit "ory_json_web_key_set.$(unique_label "$(sanitize "$jwks_set")")" "$ORY_PROJECT_ID/$jwks_set"
      else
        log "WARN: JWKS set '$jwks_set' not found, skipping"
      fi
    done
  else
    log "jwks: set ORY_JWKS_SETS=set1,set2 to import custom key sets (no list endpoint exists)"
  fi

  # Identities: data, not configuration. Report the count only (first page).
  if IDENTITIES=$(admin_get "/admin/identities?page_size=250"); then
    IDENTITY_COUNT=$(echo "$IDENTITIES" | jq 'if type == "array" then length else 0 end')
    SUFFIX=""
    if [ "$IDENTITY_COUNT" -gt 0 ]; then
      [ "$IDENTITY_COUNT" = "250" ] && SUFFIX="+"
      echo "# This project has $IDENTITY_COUNT$SUFFIX identities. Identities are data, not"
      echo "# configuration; import selectively (e.g. service accounts) with an"
      echo "# ory_identity import block whose id is the identity UUID. Credentials"
      echo "# are not importable."
      echo
    fi
    log "identities: $IDENTITY_COUNT$SUFFIX (not imported)"
  fi
else
  log "ORY_PROJECT_API_KEY not set: skipping OAuth2 clients, trusted JWT issuers, JWKS, identities"
fi

log "Done. Review the emitted blocks, then run: terraform plan -generate-config-out=generated.tf"
