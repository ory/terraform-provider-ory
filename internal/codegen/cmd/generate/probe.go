// Live-API probe for mapped attributes.
//
// The OpenAPI spec cannot be trusted alone: governs descriptions sometimes
// name config keys the API never reads (enable_ax_v2,
// disable_account_experience_welcome_screen), and declared types sometimes
// differ from the wire format (feature_flags_password_profile_registration_
// node_group is spec'd boolean but stores "password"/"default"). Both classes
// return HTTP 200 on write and only show up as perpetual plan diffs much
// later (issue #321). This probe writes a sentinel value and the type's empty
// value to each attribute's patch path on a throwaway project and classifies
// what the API reports back, so a wrong path or type is caught the moment an
// attribute is auto-discovered, not after a release.
//
// The probe is report-only: plan-gated (HTTP 403 feature_not_available),
// environment-constrained, and validation-rejected attributes are expected
// and appear as their own classes for a human to review.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

// sentinelFor returns a per-attribute sentinel value. A unique sentinel keeps
// the reported-elsewhere search from matching leftovers of another
// attribute's probe on the same project.
func sentinelFor(a Attribute) string {
	return "tf-probe-" + a.Name
}

var probeHTTPClient = &http.Client{Timeout: 30 * time.Second}

type probeEnv struct {
	ConsoleAPIURL   string
	WorkspaceAPIKey string
	WorkspaceID     string
	ProjectID       string // reuse an existing project instead of creating one
}

func probeEnvFromOS() (probeEnv, error) {
	env := probeEnv{
		ConsoleAPIURL:   strings.TrimRight(os.Getenv("ORY_CONSOLE_API_URL"), "/"),
		WorkspaceAPIKey: os.Getenv("ORY_WORKSPACE_API_KEY"),
		WorkspaceID:     os.Getenv("ORY_WORKSPACE_ID"),
		ProjectID:       os.Getenv("ORY_PROBE_PROJECT_ID"),
	}
	if env.ConsoleAPIURL == "" {
		env.ConsoleAPIURL = "https://api.console.ory.sh"
	}
	if env.WorkspaceAPIKey == "" {
		return env, fmt.Errorf("ORY_WORKSPACE_API_KEY is required for probing")
	}
	if env.WorkspaceID == "" && env.ProjectID == "" {
		return env, fmt.Errorf("set ORY_WORKSPACE_ID (to create a throwaway project) or ORY_PROBE_PROJECT_ID (to reuse one)")
	}
	return env, nil
}

// probeValues synthesizes the non-empty sentinel and the empty value for an
// attribute. Heuristics keep validation-rejected probes rare: one_of
// validators use an allowed value, URL-ish attributes get a resolvable https
// URL (the API resolves webhook hosts in DNS), and duration-ish attributes
// get a Go duration string.
func probeValues(a Attribute) (nonEmpty, empty interface{}) {
	switch a.Type {
	case typeBool:
		return true, false
	case typeInt64:
		return int64(42), int64(0)
	case typeListString:
		return []string{sentinelFor(a)}, []string{}
	case typeMapString:
		return map[string]string{"tf-probe-key": sentinelFor(a)}, map[string]string{}
	default: // string
		if a.Validators != nil && len(a.Validators.OneOf) > 0 {
			return a.Validators.OneOf[0], nil // "" would fail the validator server-side too
		}
		name := a.Name + " " + a.PatchPath
		if strings.Contains(name, "url") || strings.Contains(name, "uri") || strings.Contains(name, "origin") {
			return "https://tf-probe.example.com/" + a.Name, nil // "" is often a remove-marker; skip
		}
		if strings.Contains(name, "lifespan") || strings.Contains(name, "ttl") ||
			strings.Contains(name, "max_age") || strings.Contains(name, "period") ||
			strings.Contains(name, "expiry") || strings.Contains(name, "extend") {
			return "42m0s", nil
		}
		return sentinelFor(a), ""
	}
}

// probeOutcome classifies what the API reported back for a written value.
type probeOutcome struct {
	Class  string
	Detail string
}

// classifyProbe compares the value reported at the attribute's read path with
// the value that was written. When the key is absent it searches the whole
// config tree for the sentinel to catch the reported-under-a-different-key
// class.
func classifyProbe(config map[string]interface{}, keys []string, written interface{}) probeOutcome {
	got, present := lookupNested(config, keys)
	if present {
		if probeValueEqual(written, got) {
			return probeOutcome{Class: "OK", Detail: "reported at the read path"}
		}
		if reflect.TypeOf(got) != reflect.TypeOf(normalizeWritten(written)) {
			return probeOutcome{Class: "TYPE MISMATCH", Detail: fmt.Sprintf("wrote %v, key holds %T %v — bool_enum or a type fix may be needed", written, got, got)}
		}
		return probeOutcome{Class: "VALUE CHANGED", Detail: fmt.Sprintf("wrote %v, key holds %v — the API normalized or substituted the value", written, got)}
	}
	if path, found := findSentinel(config, written, nil); found {
		return probeOutcome{Class: "REPORTED ELSEWHERE", Detail: fmt.Sprintf("value found at %q — add read_path", path)}
	}
	return probeOutcome{Class: "NOT REPORTED", Detail: "accepted with HTTP 200 but absent from the response — wrong path (patch_path_override / revision_property), plan-gated, environment-constrained, or preserve_on_missing territory"}
}

// classifyEmptyProbe classifies the empty-value pass, which runs after the
// sentinel pass so it also exercises clearing a set value. Pruning is the
// normal, generically-handled behavior; a substituted default deserves a docs
// note; the previous sentinel still being reported means the API silently
// refuses to clear the key (verified live: neither `replace` with the empty
// value nor `remove` clears such keys), which no provider code can settle.
func classifyEmptyProbe(config map[string]interface{}, keys []string, written, previous interface{}) probeOutcome {
	got, present := lookupNested(config, keys)
	if !present {
		return probeOutcome{Class: "EMPTY PRUNED", Detail: "empty value pruned from the stored config (handled generically by the read)"}
	}
	if probeValueEqual(written, got) {
		return probeOutcome{Class: "EMPTY OK", Detail: "empty value round-trips"}
	}
	if previous != nil && probeValueEqual(previous, got) {
		return probeOutcome{Class: "CANNOT CLEAR", Detail: "the empty write leaves the previous value in place — the key cannot be cleared through the PATCH API once set; note it in the description"}
	}
	return probeOutcome{Class: "DEFAULT SUBSTITUTED", Detail: fmt.Sprintf("empty value replaced by %v — note it in the description (plan reports the substitution as a change)", got)}
}

// normalizeWritten maps written Go values onto their JSON-decoded types so
// type comparison against the response is meaningful.
func normalizeWritten(written interface{}) interface{} {
	switch v := written.(type) {
	case int64:
		return float64(v)
	case []string:
		return []interface{}{}
	case map[string]string:
		return map[string]interface{}{}
	default:
		return v
	}
}

func probeValueEqual(written, got interface{}) bool {
	switch w := written.(type) {
	case string:
		s, ok := got.(string)
		return ok && s == w
	case bool:
		b, ok := got.(bool)
		return ok && b == w
	case int64:
		f, ok := got.(float64)
		return ok && int64(f) == w
	case []string:
		arr, ok := got.([]interface{})
		if !ok || len(arr) != len(w) {
			return false
		}
		for i, item := range arr {
			s, ok := item.(string)
			if !ok || s != w[i] {
				return false
			}
		}
		return true
	case map[string]string:
		m, ok := got.(map[string]interface{})
		if !ok || len(m) != len(w) {
			return false
		}
		for k, want := range w {
			s, ok := m[k].(string)
			if !ok || s != want {
				return false
			}
		}
		return true
	}
	return false
}

func lookupNested(config map[string]interface{}, keys []string) (interface{}, bool) {
	var current interface{} = config
	for _, k := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[k]
		if !ok {
			return nil, false
		}
	}
	if current == nil {
		return nil, false
	}
	return current, true
}

// findSentinel walks the config tree looking for the written sentinel value,
// returning the dotted path where it was found. Only sentinel-bearing types
// are searched; bools and ints are too ambiguous to locate uniquely.
func findSentinel(node interface{}, written interface{}, path []string) (string, bool) {
	var sentinel string
	switch w := written.(type) {
	case string:
		sentinel = w
	case []string:
		if len(w) == 1 {
			sentinel = w[0]
		}
	case map[string]string:
		for _, v := range w {
			sentinel = v
		}
	}
	if sentinel == "" || sentinel == "42m0s" {
		return "", false // ambiguous values would produce false positives
	}
	switch n := node.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(n))
		for k := range n {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if p, found := findSentinel(n[k], written, append(path, k)); found {
				return p, true
			}
		}
	case []interface{}:
		for _, item := range n {
			if p, found := findSentinel(item, written, path); found {
				return p, true
			}
		}
	case string:
		if n == sentinel {
			return strings.Join(path, "."), true
		}
	}
	return "", false
}

// probeAttributes probes the named attributes against the live console API
// and writes a markdown report. It is report-only: any classification other
// than OK is information for the human reviewing the auto-discovered entries,
// not a failure.
func probeAttributes(m Mappings, names []string, reportPath string) error {
	env, err := probeEnvFromOS()
	if err != nil {
		return err
	}

	byName := make(map[string]Attribute, len(m.Attributes))
	for _, a := range m.Attributes {
		byName[a.Name] = a
	}

	projectID := env.ProjectID
	created := false
	if projectID == "" {
		projectID, err = createProbeProject(env)
		if err != nil {
			return fmt.Errorf("creating throwaway probe project: %w", err)
		}
		created = true
		fmt.Printf("Created throwaway probe project %s\n", projectID)
	}
	defer func() {
		if created {
			if err := deleteProbeProject(env, projectID); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: could not delete probe project %s: %v\n", projectID, err)
			} else {
				fmt.Printf("Deleted throwaway probe project %s\n", projectID)
			}
		}
	}()

	var report strings.Builder
	report.WriteString("### Live API probe of auto-discovered attributes\n\n")
	report.WriteString("Every attribute was written to a throwaway project at its derived patch path and read back. ")
	report.WriteString("`OK` plus `EMPTY PRUNED`/`EMPTY OK` needs no action. Anything else needs a human: ")
	report.WriteString("`TYPE MISMATCH` -> `bool_enum` or a type fix, `REPORTED ELSEWHERE` -> `read_path`, ")
	report.WriteString("`NOT REPORTED` -> verify the path per CONTRIBUTING (wrong governs path, plan-gated, or `preserve_on_missing`), ")
	report.WriteString("`DEFAULT SUBSTITUTED` -> mention the server default in the description, `WRITE REJECTED` -> probe manually with a valid value.\n\n")
	report.WriteString("| Attribute | Sentinel write | Empty write |\n|---|---|---|\n")

	for _, name := range names {
		a, ok := byName[name]
		if !ok {
			fmt.Fprintf(&report, "| `%s` | UNKNOWN ATTRIBUTE — not in mappings.yaml | |\n", name)
			continue
		}
		if a.RevisionProperty {
			fmt.Fprintf(&report, "| `%s` | handled via the normalized revision API (revision_property) — document-path probe not applicable | |\n", name)
			continue
		}
		nonEmptyOutcome, emptyOutcome := probeOneAttribute(env, projectID, a)
		fmt.Fprintf(&report, "| `%s` | %s | %s |\n", name, renderOutcome(nonEmptyOutcome), renderOutcome(emptyOutcome))
	}

	if reportPath == "" || reportPath == "-" {
		fmt.Println(report.String())
		return nil
	}
	// #nosec G306 -- markdown report, standard permissions are fine
	return os.WriteFile(reportPath, []byte(report.String()), 0644)
}

func renderOutcome(o *probeOutcome) string {
	if o == nil {
		return "skipped"
	}
	return fmt.Sprintf("**%s** — %s", o.Class, o.Detail)
}

func probeOneAttribute(env probeEnv, projectID string, a Attribute) (*probeOutcome, *probeOutcome) {
	_, keys := parseService(a.PatchPath)
	service := strings.Split(strings.TrimPrefix(a.PatchPath, "/"), "/")[1]
	readPath := a.ReadKeysPath()
	_, readKeys := parseService(readPath)

	nonEmpty, empty := probeValues(a)

	// Classification uses the PATCH response body: the endpoint returns the
	// updated project, which is the authoritative read-after-write. A separate
	// GET can race revision propagation and report the previous write.
	writeAndRead := func(value interface{}) (map[string]interface{}, *probeOutcome) {
		respBody, status, err := patchProbeProject(env, projectID, a.PatchPath, value)
		if err != nil {
			return nil, &probeOutcome{Class: "WRITE FAILED", Detail: err.Error()}
		}
		if status != http.StatusOK {
			return nil, &probeOutcome{Class: "WRITE REJECTED", Detail: fmt.Sprintf("HTTP %d: %s", status, truncate(respBody, 160))}
		}
		config, err := serviceConfigFromPatchResponse(respBody, service)
		if err != nil {
			return nil, &probeOutcome{Class: "READ FAILED", Detail: err.Error()}
		}
		return config, nil
	}

	config, failure := writeAndRead(nonEmpty)
	if failure != nil {
		// An unreadable write makes the empty pass meaningless noise; report
		// the first finding alone. The write path is exercised either way.
		return failure, nil
	}
	outcome := classifyProbe(config, readKeys, nonEmpty)
	nonEmptyOutcome := &outcome

	if empty == nil {
		return nonEmptyOutcome, nil
	}
	config, failure = writeAndRead(empty)
	if failure != nil {
		return nonEmptyOutcome, failure
	}
	emptyResult := classifyEmptyProbe(config, readKeys, empty, nonEmpty)
	_ = keys
	return nonEmptyOutcome, &emptyResult
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// --- Raw console API helpers -------------------------------------------------

func probeRequest(env probeEnv, method, path string, body interface{}) (string, int, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return "", 0, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, env.ConsoleAPIURL+path, reader)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+env.WorkspaceAPIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := probeHTTPClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(respBody), resp.StatusCode, nil
}

func createProbeProject(env probeEnv) (string, error) {
	body, status, err := probeRequest(env, http.MethodPost, "/projects", map[string]string{
		"name":         "tf-codegen-probe",
		"environment":  "prod",
		"workspace_id": env.WorkspaceID,
		"home_region":  "eu-central",
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", status, truncate(body, 200))
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil || parsed.ID == "" {
		return "", fmt.Errorf("could not parse project id from create response")
	}
	return parsed.ID, nil
}

func deleteProbeProject(env probeEnv, projectID string) error {
	body, status, err := probeRequest(env, http.MethodDelete, "/projects/"+projectID, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", status, truncate(body, 160))
	}
	return nil
}

func patchProbeProject(env probeEnv, projectID, path string, value interface{}) (string, int, error) {
	return probeRequest(env, http.MethodPatch, "/projects/"+projectID, []map[string]interface{}{
		{"op": "replace", "path": path, "value": value},
	})
}

// serviceConfigFromPatchResponse extracts a service's config map from the
// PATCH /projects/{id} response body ({"project": {...}, "warnings": [...]}).
func serviceConfigFromPatchResponse(body, service string) (map[string]interface{}, error) {
	var parsed struct {
		Project struct {
			Services map[string]struct {
				Config map[string]interface{} `json:"config"`
			} `json:"services"`
		} `json:"project"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, err
	}
	svc, ok := parsed.Project.Services[service]
	if !ok {
		return nil, fmt.Errorf("service %q missing from patch response", service)
	}
	return svc.Config, nil
}
