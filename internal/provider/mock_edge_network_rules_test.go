package provider

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// fakeEdgeNetworkID is the dedicated edge network the fake takes rules on.
// GET /edge-networks lists it as acme-demo.cloud.
const fakeEdgeNetworkID = "edgenet-0002"

// fakeEdgeRuleLimit is how many custom rules an edge network takes.
const fakeEdgeRuleLimit = 50

type fakeEdgeRule struct {
	ruleset     string
	name        string
	description *string
	isActive    bool
	settings    map[string]any
}

// fakeEdgeRules models the rulesets of the dedicated edge network.
type fakeEdgeRules struct {
	rules map[string]*fakeEdgeRule
	// order holds each ruleset's rule IDs in evaluation order.
	order map[string][]string
	// busy is how many of the next rule changes are refused with a 409, as
	// the API does while another change to the ruleset is in progress.
	busy int
	// writes counts every POST, PATCH and DELETE, refused ones included.
	writes int
}

// registerEdgeNetworkRuleRoutes serves the rule routes of a dedicated edge
// network the way the live API answered them:
//   - the ruleset is a hyphenated slug in the URL and an underscored type in
//     the rule;
//   - responses carry every settings key the ruleset knows, null when unset,
//     and a rate limit gets counting_key "visitor" when none is given;
//   - a new rule is active and added last unless a position is given;
//   - names are unique within a ruleset, and the edge network takes at most
//     50 custom rules;
//   - while another change is in progress a change is refused with a 409;
//   - PATCH and DELETE of a missing rule answer with a 302 to the
//     dashboard, while GET answers 404.
func (f *fakeCloud) registerEdgeNetworkRuleRoutes(mux *http.ServeMux) {
	const collection = "/edge-networks/{edge}/rulesets/{ruleset}/rules"
	const item = collection + "/{id}"
	f.edgeRules = &fakeEdgeRules{rules: map[string]*fakeEdgeRule{}, order: map[string][]string{}}

	ruleset := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		rs := strings.ReplaceAll(r.PathValue("ruleset"), "-", "_")
		if r.PathValue("edge") != fakeEdgeNetworkID || !slices.Contains([]string{"firewall", "rate_limit", "cache"}, rs) ||
			strings.Contains(r.PathValue("ruleset"), "_") {
			writeError(w, http.StatusNotFound, fmt.Errorf("not found"))
			return "", false
		}
		return rs, true
	}
	refuseIfBusy := func(w http.ResponseWriter) bool {
		f.edgeRules.writes++
		if f.edgeRules.busy > 0 {
			f.edgeRules.busy--
			writeError(w, http.StatusConflict, fmt.Errorf("Another change to this ruleset is in progress. Try again once it finishes."))
			return true
		}
		return false
	}
	redirect := func(w http.ResponseWriter) {
		w.Header().Set("Location", "https://cloud.laravel.com")
		w.WriteHeader(http.StatusFound)
	}

	mux.HandleFunc("POST "+collection, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		rs, ok := ruleset(w, r)
		if !ok || refuseIfBusy(w) {
			return
		}
		name, _ := body["name"].(string)
		settings, _ := body["settings"].(map[string]any)
		if msg := f.validateEdgeRule(rs, "", name, settings, body["position"]); msg != "" {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("%s", msg))
			return
		}
		if len(f.edgeRules.rules) >= fakeEdgeRuleLimit {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("The edge network has reached the maximum of %d custom rules.", fakeEdgeRuleLimit))
			return
		}

		f.seq++
		id := fmt.Sprintf("nru-%d", f.seq)
		rule := &fakeEdgeRule{ruleset: rs, name: name, isActive: true, settings: settings}
		if d, ok := body["description"].(string); ok {
			rule.description = &d
		}
		f.edgeRules.rules[id] = rule
		f.edgeRules.order[rs] = append(f.edgeRules.order[rs], id)
		f.moveEdgeRule(rs, id, body["position"])
		writeJSON(w, http.StatusCreated, map[string]any{"data": f.edgeRuleView(id)})
	})

	mux.HandleFunc("GET "+collection, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		rs, ok := ruleset(w, r)
		if !ok {
			return
		}
		out := []any{}
		for _, id := range f.edgeRules.order[rs] {
			out = append(out, f.edgeRuleView(id))
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})

	mux.HandleFunc("GET "+item, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		rs, ok := ruleset(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if rule, ok := f.edgeRules.rules[id]; !ok || rule.ruleset != rs {
			writeError(w, http.StatusNotFound, fmt.Errorf("No query results for model [App\\Models\\NetworkRule] %s", id))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": f.edgeRuleView(id)})
	})

	mux.HandleFunc("PATCH "+item, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		rs, ok := ruleset(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		rule, ok := f.edgeRules.rules[id]
		if !ok || rule.ruleset != rs {
			redirect(w)
			return
		}
		if refuseIfBusy(w) {
			return
		}
		name := rule.name
		if n, ok := body["name"].(string); ok {
			name = n
		}
		settings := rule.settings
		if s, ok := body["settings"].(map[string]any); ok {
			settings = s
		}
		if msg := f.validateEdgeRule(rs, id, name, settings, body["position"]); msg != "" {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("%s", msg))
			return
		}
		rule.name, rule.settings = name, settings
		if d, ok := body["description"]; ok {
			s, _ := d.(string)
			rule.description = nil
			if d != nil {
				rule.description = &s
			}
		}
		if a, ok := body["is_active"].(bool); ok {
			rule.isActive = a
		}
		f.moveEdgeRule(rs, id, body["position"])
		writeJSON(w, http.StatusOK, map[string]any{"data": f.edgeRuleView(id)})
	})

	mux.HandleFunc("DELETE "+item, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		rs, ok := ruleset(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if rule, ok := f.edgeRules.rules[id]; !ok || rule.ruleset != rs {
			redirect(w)
			return
		}
		if refuseIfBusy(w) {
			return
		}
		delete(f.edgeRules.rules, id)
		f.edgeRules.order[rs] = slices.DeleteFunc(f.edgeRules.order[rs], func(o string) bool { return o == id })
		w.WriteHeader(http.StatusNoContent)
	})
}

// validateEdgeRule returns the API's validation message for a rule, or "".
// The caller holds f.mu.
func (f *fakeCloud) validateEdgeRule(rs, id, name string, settings map[string]any, position any) string {
	if name == "" {
		return "The name field is required."
	}
	for otherID, other := range f.edgeRules.rules {
		if otherID != id && other.ruleset == rs && other.name == name {
			return "The name has already been taken."
		}
	}
	if settings == nil || settings["action"] == nil {
		return "The settings.action field is required."
	}
	if groups, _ := settings["criteria_groups"].([]any); len(groups) == 0 {
		return "The settings.criteria groups field is required."
	}
	if p, ok := position.(map[string]any); ok {
		if len(p) != 1 {
			return "The position field must contain 1 items."
		}
		for _, key := range []string{"before", "after"} {
			if ref, ok := p[key].(string); ok && (ref == id || !slices.Contains(f.edgeRules.order[rs], ref)) {
				return fmt.Sprintf("The rule must be another rule in the same ruleset. (position.%s)", key)
			}
		}
	}
	return ""
}

// moveEdgeRule places a rule as position asks, if it asks. The caller holds
// f.mu.
func (f *fakeCloud) moveEdgeRule(rs, id string, position any) {
	p, ok := position.(map[string]any)
	if !ok {
		return
	}
	order := slices.DeleteFunc(slices.Clone(f.edgeRules.order[rs]), func(o string) bool { return o == id })
	at := len(order)
	switch {
	case p["index"] != nil:
		at = min(int(p["index"].(float64))-1, len(order))
	case p["before"] != nil:
		at = slices.Index(order, p["before"].(string))
	case p["after"] != nil:
		at = slices.Index(order, p["after"].(string)) + 1
	}
	f.edgeRules.order[rs] = slices.Insert(order, at, id)
}

// edgeRuleView renders a rule as the API does. The caller holds f.mu.
func (f *fakeCloud) edgeRuleView(id string) map[string]any {
	rule := f.edgeRules.rules[id]
	settings := map[string]any{}
	switch rule.ruleset {
	case "firewall":
		settings["skip_parameters"], settings["block_parameters"] = nil, nil
	case "rate_limit":
		settings["counting_key"], settings["counting_header"] = "visitor", nil
	case "cache":
		settings["edge_ttl"], settings["browser_ttl"] = nil, nil
	}
	for k, v := range rule.settings {
		settings[k] = v
	}
	return map[string]any{
		"id":   id,
		"type": "edge_network_rules",
		"attributes": map[string]any{
			"type":        rule.ruleset,
			"name":        rule.name,
			"description": rule.description,
			"is_active":   rule.isActive,
			"settings":    settings,
			"created_at":  "2026-11-20T06:51:56.000000Z",
			"updated_at":  "2026-11-20T06:51:57.000000Z",
		},
	}
}
