package main

import (
	"strings"
	"testing"
)

func cfg(nested map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"selfservice": map[string]interface{}{"methods": nested}}
}

// classifyProbe must recognize every read-back class the live audits found:
// echoed values, string enums behind bool-typed spec properties (the
// password_profile_registration_node_group class), values reported under a
// different key (the pairwise-salt class), and writes the API accepts and
// discards (the enable_ax_v2 class).
func TestClassifyProbe(t *testing.T) {
	keys := []string{"selfservice", "methods", "flag"}

	t.Run("value reported at the read path", func(t *testing.T) {
		outcome := classifyProbe(cfg(map[string]interface{}{"flag": true}), keys, true)
		if outcome.Class != "OK" {
			t.Errorf("class = %q, want OK (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("bool written, string enum reported", func(t *testing.T) {
		outcome := classifyProbe(cfg(map[string]interface{}{"flag": "default"}), keys, true)
		if outcome.Class != "TYPE MISMATCH" {
			t.Errorf("class = %q, want TYPE MISMATCH (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("value normalized to a different value", func(t *testing.T) {
		outcome := classifyProbe(cfg(map[string]interface{}{"flag": "other"}), keys, "tf-probe-sentinel")
		if outcome.Class != "VALUE CHANGED" {
			t.Errorf("class = %q, want VALUE CHANGED (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("value reported under a different key", func(t *testing.T) {
		config := cfg(map[string]interface{}{"nested": map[string]interface{}{"elsewhere": "tf-probe-sentinel"}})
		outcome := classifyProbe(config, keys, "tf-probe-sentinel")
		if outcome.Class != "REPORTED ELSEWHERE" {
			t.Fatalf("class = %q, want REPORTED ELSEWHERE (%s)", outcome.Class, outcome.Detail)
		}
		if !strings.Contains(outcome.Detail, "selfservice.methods.nested.elsewhere") {
			t.Errorf("detail %q must name the path the value was found at", outcome.Detail)
		}
	})

	t.Run("write accepted and discarded", func(t *testing.T) {
		outcome := classifyProbe(cfg(map[string]interface{}{}), keys, "tf-probe-sentinel")
		if outcome.Class != "NOT REPORTED" {
			t.Errorf("class = %q, want NOT REPORTED (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("list sentinel found in a provider-style array", func(t *testing.T) {
		config := cfg(map[string]interface{}{"providers": []interface{}{
			map[string]interface{}{"scope": []interface{}{"tf-probe-sentinel"}},
		}})
		outcome := classifyProbe(config, keys, []string{"tf-probe-sentinel"})
		if outcome.Class != "REPORTED ELSEWHERE" {
			t.Errorf("class = %q, want REPORTED ELSEWHERE (%s)", outcome.Class, outcome.Detail)
		}
	})
}

// classifyEmptyProbe distinguishes the generically handled pruning from the
// server-default substitution that needs a docs note.
func TestClassifyEmptyProbe(t *testing.T) {
	keys := []string{"selfservice", "methods", "domains"}

	previous := []string{"tf-probe-previous"}

	t.Run("empty value pruned", func(t *testing.T) {
		outcome := classifyEmptyProbe(cfg(map[string]interface{}{}), keys, []string{}, previous)
		if outcome.Class != "EMPTY PRUNED" {
			t.Errorf("class = %q, want EMPTY PRUNED (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("empty value round-trips", func(t *testing.T) {
		outcome := classifyEmptyProbe(cfg(map[string]interface{}{"domains": []interface{}{}}), keys, []string{}, previous)
		if outcome.Class != "EMPTY OK" {
			t.Errorf("class = %q, want EMPTY OK (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("server default substituted", func(t *testing.T) {
		outcome := classifyEmptyProbe(cfg(map[string]interface{}{"domains": []interface{}{"en", "de"}}), keys, []string{}, previous)
		if outcome.Class != "DEFAULT SUBSTITUTED" {
			t.Errorf("class = %q, want DEFAULT SUBSTITUTED (%s)", outcome.Class, outcome.Detail)
		}
	})

	t.Run("previous value stuck in place", func(t *testing.T) {
		outcome := classifyEmptyProbe(cfg(map[string]interface{}{"domains": []interface{}{"tf-probe-previous"}}), keys, []string{}, previous)
		if outcome.Class != "CANNOT CLEAR" {
			t.Errorf("class = %q, want CANNOT CLEAR (%s)", outcome.Class, outcome.Detail)
		}
	})
}

// probeValues heuristics keep validation-rejected probes rare.
func TestProbeValues(t *testing.T) {
	t.Run("one_of validator uses an allowed value", func(t *testing.T) {
		nonEmpty, empty := probeValues(Attribute{
			Name: "login_style", Type: typeString,
			Validators: &Validator{OneOf: []string{"unified", "identifier_first"}},
		})
		if nonEmpty != "unified" {
			t.Errorf("nonEmpty = %v, want the first one_of value", nonEmpty)
		}
		if empty != nil {
			t.Errorf("empty = %v, want nil (an empty string would fail the validator server-side)", empty)
		}
	})

	t.Run("url attributes get a resolvable https url", func(t *testing.T) {
		nonEmpty, _ := probeValues(Attribute{Name: "selfservice_flows_login_ui_url", Type: typeString, PatchPath: "/services/identity/config/x"})
		s, ok := nonEmpty.(string)
		if !ok || !strings.HasPrefix(s, "https://") {
			t.Errorf("nonEmpty = %v, want an https URL", nonEmpty)
		}
	})

	t.Run("duration attributes get a duration string", func(t *testing.T) {
		nonEmpty, _ := probeValues(Attribute{Name: "session_lifespan", Type: typeString, PatchPath: "/services/identity/config/session/lifespan"})
		if nonEmpty != "42m0s" {
			t.Errorf("nonEmpty = %v, want a Go duration string", nonEmpty)
		}
	})

	t.Run("plain string gets a per-attribute sentinel and an empty pass", func(t *testing.T) {
		nonEmpty, empty := probeValues(Attribute{Name: "oauth2_token_prefix", Type: typeString, PatchPath: "/services/oauth2/config/oauth2/token_prefix"})
		if nonEmpty != "tf-probe-oauth2_token_prefix" {
			t.Errorf("nonEmpty = %v, want the per-attribute sentinel", nonEmpty)
		}
		if empty != "" {
			t.Errorf("empty = %v, want the empty string", empty)
		}
	})
}
