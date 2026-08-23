package provider

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The inventory script in the import-existing-project skill walks a project
// revision with jq. Every jq expression that indexes a nested key aborts the
// whole run when the key holds an unexpected type, and because the script sets
// `set -euo pipefail`, an abort truncates imports.tf after a partial write. The
// result looks like a complete inventory and is not one.
//
// These tests feed the script's own jq programs the revision shapes that used to
// crash them. They read the programs out of the script so a future edit cannot
// pass the test while breaking the script.

const skillScriptRelPath = "../../.claude/skills/import-existing-project/scripts/generate-imports.sh"

func requireJQ(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not installed; skipping inventory script jq tests")
	}
}

func readSkillScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(skillScriptRelPath))
	require.NoError(t, err, "the import skill inventory script must ship with the provider")
	return string(b)
}

// extractSingleQuoted returns the body of a single-quoted shell assignment,
// for example ACTIONS_JQ='...'. jq programs are stored that way so the shell
// leaves their $variables alone.
func extractSingleQuoted(t *testing.T, script, varName string) string {
	t.Helper()
	marker := varName + "='"
	start := strings.Index(script, marker)
	require.GreaterOrEqual(t, start, 0, "%s is not assigned in the inventory script", varName)
	rest := script[start+len(marker):]
	end := strings.Index(rest, "'")
	require.GreaterOrEqual(t, end, 0, "%s is not terminated", varName)
	program := rest[:end]
	require.NotEmpty(t, strings.TrimSpace(program), "%s is empty", varName)
	return program
}

// runJQ pipes input through a jq program and returns its compact output. A jq
// failure is returned as an error so a test can assert the program no longer
// aborts on a given shape.
func runJQ(t *testing.T, program, input string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "jq", "-c", program)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// obj is a shorthand for a decoded JSON object.
type obj = map[string]any

// identityRevision wraps an identity service config the way GET /projects/{id}
// returns it, and marshals it. Building the JSON instead of writing it by hand
// keeps a miscounted brace from masquerading as a jq failure.
func identityRevision(t *testing.T, config obj) string {
	t.Helper()
	b, err := json.Marshal(obj{"services": obj{"identity": obj{"config": config}}})
	require.NoError(t, err)
	return string(b)
}

// flowsConfig nests a flows map at the path the provider reads it from.
func flowsConfig(flows obj) obj {
	return obj{"selfservice": obj{"flows": flows}}
}

// hookURL is the webhook target every actions fixture uses. The jq program
// treats it as an opaque string, so one value covers every case.
const hookURL = "https://a.test"

func webHook() obj {
	return obj{"hook": "web_hook", "config": obj{"url": hookURL, "method": "POST"}}
}

func TestImportSkillActionsJQToleratesUnexpectedShapes(t *testing.T) {
	requireJQ(t)
	program := extractSingleQuoted(t, readSkillScript(t), "ACTIONS_JQ")

	tests := []struct {
		name   string
		config obj
		want   string
	}{
		{
			// default_browser_return_url is a real project_config attribute and a
			// scalar sibling of the auth-method objects under "after". Indexing it
			// used to abort with `Cannot index string with string ("hooks")`.
			name: "scalar sibling under after",
			config: flowsConfig(obj{"login": obj{"after": obj{
				"default_browser_return_url": "https://x.test",
				"password":                   obj{"hooks": []any{webHook()}},
			}}}),
			want: `[{"label":"login_after_password","id":"login:after:password:POST:https://a.test"}]`,
		},
		{
			// login scopes its after hooks by auth method, so a flat array there is
			// unreachable for ory_action and must not be emitted at all.
			name: "flat after hooks on login are dropped",
			config: flowsConfig(obj{"login": obj{"after": obj{
				"hooks": []any{webHook()},
			}}}),
			want: `[]`,
		},
		{
			// verification does not scope by auth method, so the "_" placeholder is
			// correct there.
			name: "flat after hooks on verification keep the placeholder",
			config: flowsConfig(obj{"verification": obj{"after": obj{
				"hooks": []any{webHook()},
			}}}),
			want: `[{"label":"verification_after","id":"verification:after:_:POST:https://a.test"}]`,
		},
		{
			name: "before hooks",
			config: flowsConfig(obj{"login": obj{"before": obj{
				"hooks": []any{webHook()},
			}}}),
			want: `[{"label":"login_before","id":"login:before:POST:https://a.test"}]`,
		},
		{
			name:   "flows missing entirely",
			config: obj{},
			want:   `[]`,
		},
		{
			name:   "flows is not an object",
			config: obj{"selfservice": obj{"flows": "nope"}},
			want:   `[]`,
		},
		{
			name:   "after is not an object",
			config: flowsConfig(obj{"login": obj{"after": "nope"}}),
			want:   `[]`,
		},
		{
			name: "hook entry is not an object",
			config: flowsConfig(obj{"login": obj{"after": obj{
				"password": obj{"hooks": []any{"nope"}},
			}}}),
			want: `[]`,
		},
		{
			// A hook with no config.url would build an import ID ending in "null".
			name: "hook without a url is skipped",
			config: flowsConfig(obj{"login": obj{"after": obj{
				"password": obj{"hooks": []any{obj{"hook": "web_hook", "config": obj{"method": "POST"}}}},
			}}}),
			want: `[]`,
		},
		{
			name: "non-webhook hooks are skipped",
			config: flowsConfig(obj{"settings": obj{"after": obj{
				"profile": obj{"hooks": []any{obj{"hook": "verify_new_address"}}},
			}}}),
			want: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runJQ(t, program, identityRevision(t, tt.config))
			require.NoError(t, err, "the actions program must not abort on this shape: %s", got)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The remaining sections build their jq inline, so these tests assert the
// property that matters: the script survives a revision holding each shape and
// still writes a complete file. Running the whole script needs network access,
// so the check is on the inline programs the script actually contains.
func TestImportSkillInlineJQToleratesUnexpectedShapes(t *testing.T) {
	requireJQ(t)
	script := readSkillScript(t)

	permissionRevision := func(config obj) string {
		b, err := json.Marshal(obj{"services": obj{"permission": obj{"config": config}}})
		require.NoError(t, err)
		return string(b)
	}
	methodProviders := func(method string, providers []any) obj {
		return obj{"selfservice": obj{"methods": obj{method: obj{"config": obj{"providers": providers}}}}}
	}

	tests := []struct {
		name string
		// needle is a distinctive fragment of the inline jq, used to prove the
		// script still contains the guarded form rather than a regressed one.
		needle string
		input  string
	}{
		{
			// namespaces is either an array of {name, id} or an object holding a
			// "location" URL. Indexing the object shape as an array aborted jq.
			name:   "keto namespaces as a location object",
			needle: `if type == "array" then [.[] | select(type == "object") | .name | select(type == "string")]`,
			input:  permissionRevision(obj{"namespaces": obj{"location": "base64://abc"}}),
		},
		{
			// A schema entry without a string id aborted startswith().
			name:   "identity schema entry without a string id",
			needle: `select(startswith("preset://") | not)`,
			input: identityRevision(t, obj{"identity": obj{"schemas": []any{
				obj{"url": "https://x"},
			}}}),
		},
		{
			// A scalar under courier.templates.<base> aborted to_entries.
			name:   "courier template group as a scalar",
			needle: `select(.key == "valid" or .key == "invalid")`,
			input:  identityRevision(t, obj{"courier": obj{"templates": obj{"recovery_code": "nope"}}}),
		},
		{
			name:   "oidc providers array holding a scalar",
			needle: `.services.identity.config.selfservice.methods.oidc.config.providers[]?`,
			input:  identityRevision(t, methodProviders("oidc", []any{"nope"})),
		},
		{
			name:   "saml providers array holding a scalar",
			needle: `.services.identity.config.selfservice.methods.saml.config.providers[]?`,
			input:  identityRevision(t, methodProviders("saml", []any{"nope"})),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, script, tt.needle,
				"the inventory script no longer contains the guarded form this test covers")
			program := inlineJQContaining(t, script, tt.needle)
			got, err := runJQ(t, program, tt.input)
			require.NoError(t, err, "the program must not abort on this shape: %s", got)
		})
	}
}

// inlineJQContaining reconstructs the single-quoted jq program surrounding a
// needle. The script writes these as jq -c '...' or $(... | jq -c '...'), so the
// program is delimited by the single quotes on either side of the needle.
func inlineJQContaining(t *testing.T, script, needle string) string {
	t.Helper()
	idx := strings.Index(script, needle)
	require.GreaterOrEqual(t, idx, 0)
	start := strings.LastIndex(script[:idx], "'")
	require.GreaterOrEqual(t, start, 0, "no opening quote before the needle")
	rest := script[start+1:]
	end := strings.Index(rest, "'")
	require.GreaterOrEqual(t, end, 0, "no closing quote after the needle")
	return rest[:end]
}

// The script emits Terraform addresses built from user-controlled strings.
// sanitize collapses every run of non-alphanumerics to "_", so two different
// provider IDs can land on the same label and Terraform then rejects the file
// for duplicate import blocks. unique_label has to dedupe across subshells,
// because every call site invokes it inside a command substitution.
func TestImportSkillScriptDedupesLabels(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed; skipping inventory script helper tests")
	}
	scriptPath, err := filepath.Abs(filepath.Clean(skillScriptRelPath))
	require.NoError(t, err)

	// Source only the helper block, then exercise it the way the script does.
	harness := `
set -euo pipefail
RUN_DIR=$(mktemp -d); LABELS_FILE="$RUN_DIR/labels"; : >"$LABELS_FILE"
eval "$(sed -n '/^# Turn an arbitrary string into a valid Terraform/,/^emit() {/p' "$1" | sed '$d')"
printf '%s %s %s %s\n' \
  "$(unique_label "$(sanitize 'google-1')")" \
  "$(unique_label "$(sanitize 'google_1')")" \
  "$(unique_label "$(sanitize 'google 1')")" \
  "$(unique_label "$(sanitize 'github')")"
printf '%s %s\n' "$(hcl_escape 'a"b')" "$(hcl_escape 'a\b')"
rm -rf "$RUN_DIR"
`
	cmd := exec.CommandContext(t.Context(), "bash", "-c", harness, "harness", scriptPath)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "helper harness failed: %s", out)

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.Len(t, lines, 2, "unexpected harness output: %s", out)

	assert.Equal(t, "google_1 google_1_2 google_1_3 github", lines[0],
		"colliding labels must be suffixed so Terraform addresses stay unique")
	assert.Equal(t, `a\"b a\\b`, lines[1],
		"a quote or backslash in an import ID must be escaped for HCL")
}
