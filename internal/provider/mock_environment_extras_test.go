package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// registerEnvironmentExtraRoutes serves the environment sub-resources that are
// not generic CRUD: secret attachments and the Resend integration.
func (f *fakeCloud) registerEnvironmentExtraRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /environments/{id}/secrets", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		id := r.PathValue("id")

		f.mu.Lock()
		defer f.mu.Unlock()
		env, ok := f.envs[id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
			return
		}
		list, _ := body["secrets"].([]any)
		if len(list) == 0 {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("secrets is required"))
			return
		}
		for _, item := range list {
			sid, _ := item.(string)
			if _, ok := f.objects["secret"][sid]; !ok {
				writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("the selected secret %q is invalid", sid))
				return
			}
		}
		if f.secrets.attached[id] == nil {
			f.secrets.attached[id] = map[string]bool{}
		}
		for _, item := range list {
			f.secrets.attached[id][item.(string)] = true
		}
		writeJSON(w, http.StatusOK, environmentDoc(env))
	})

	mux.HandleFunc("DELETE /environments/{id}/secrets/{secret}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id, sid := r.PathValue("id"), r.PathValue("secret")
		env, ok := f.envs[id]
		if !ok || !f.secrets.attached[id][sid] {
			writeError(w, http.StatusNotFound, fmt.Errorf("secret not attached"))
			return
		}
		delete(f.secrets.attached[id], sid)
		writeJSON(w, http.StatusOK, environmentDoc(env))
	})

	mux.HandleFunc("POST /environments/{id}/resend", func(w http.ResponseWriter, r *http.Request) {
		var req client.AttachResendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		env, ok := f.envs[r.PathValue("id")]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
			return
		}
		reuse := req.KeyStrategy != nil && *req.KeyStrategy == "reuse"
		switch {
		case reuse && req.ReuseKeyID == nil:
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("reuse_key_id is required when reusing a key"))
			return
		case !reuse && req.ReuseKeyID != nil:
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("reuse_key_id is prohibited unless reusing a key"))
			return
		case reuse && req.KeyName != nil:
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("key_name is prohibited when reusing a key"))
			return
		}
		keyName := "laravel-cloud-" + env.ID
		switch {
		case reuse:
			keyName = "shared-key-" + *req.ReuseKeyID
		case req.KeyName != nil:
			keyName = *req.KeyName
		}
		env.Attributes.Resend = &client.EnvironmentResend{FromAddress: req.FromAddress, FromName: req.FromName, KeyName: keyName}
		writeJSON(w, http.StatusOK, environmentDoc(env))
	})

	mux.HandleFunc("PATCH /environments/{id}/resend", func(w http.ResponseWriter, r *http.Request) {
		var req client.UpdateResendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		env, ok := f.envs[r.PathValue("id")]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
			return
		}
		if env.Attributes.Resend == nil {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("resend is not attached"))
			return
		}
		env.Attributes.Resend.FromAddress = req.FromAddress
		env.Attributes.Resend.FromName = req.FromName
		writeJSON(w, http.StatusOK, environmentDoc(env))
	})

	mux.HandleFunc("DELETE /environments/{id}/resend", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		env, ok := f.envs[r.PathValue("id")]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
			return
		}
		env.Attributes.Resend = nil
		w.WriteHeader(http.StatusNoContent)
	})
}

// applyEnvironmentSettings applies the newer PATCH fields the way the API does.
// Response headers and firewall toggles merge into what is there. The serving
// mode is fixed at creation, and only a static site has a root to change.
// nightwatch_token and filesystem_keys are accepted but never reported back;
// tests read them from f.envUpdates.
func (f *fakeCloud) applyEnvironmentSettings(env *client.EnvironmentData, req client.UpdateEnvironmentRequest) error {
	a := &env.Attributes
	if req.Serving != nil {
		current := "process"
		if a.Serving != nil && a.Serving.Mode != "" {
			current = a.Serving.Mode
		}
		if req.Serving.Mode != current {
			return fmt.Errorf("the serving mode cannot be changed after creation")
		}
		if current != "static" && !req.Serving.Root.Null {
			return fmt.Errorf("serving.root is only allowed for static environments")
		}
		if current == "static" {
			root := "."
			if !req.Serving.Root.Null {
				root = req.Serving.Root.Value
			}
			a.Serving = &client.EnvironmentServing{Mode: "static", Root: &root}
		}
	}
	if req.StartCommand != nil {
		if req.StartCommand.Null {
			a.StartCommand = nil
		} else {
			a.StartCommand = ptr(req.StartCommand.Value)
		}
	}
	if req.HibernationWakeUpInterval != nil {
		if req.HibernationWakeUpInterval.Null {
			a.HibernationWakeUpInterval = nil
		} else {
			a.HibernationWakeUpInterval = ptr(int64(req.HibernationWakeUpInterval.Value))
		}
	}
	rh := &a.NetworkSettings.ResponseHeaders
	if req.ResponseHeadersFrame != nil {
		rh.Frame = req.ResponseHeadersFrame
	}
	if req.ResponseHeadersContentType != nil {
		rh.ContentType = req.ResponseHeadersContentType
	}
	if req.ResponseHeadersRobotsTag != nil {
		rh.RobotsTag = req.ResponseHeadersRobotsTag
	}
	if h := req.ResponseHeadersHSTS; h != nil {
		if (h.IncludeSubdomains || h.Preload) && h.MaxAge.Null {
			return fmt.Errorf("response_headers_hsts.max_age is required when include_subdomains or preload is set")
		}
		hsts := &client.HSTS{IncludeSubdomains: h.IncludeSubdomains, Preload: h.Preload}
		if !h.MaxAge.Null {
			hsts.MaxAge = ptr(int64(h.MaxAge.Value))
		}
		rh.HSTS = hsts
	}
	if req.FirewallBlockPath != nil {
		a.NetworkSettings.Firewall.BlockPath = req.FirewallBlockPath
	}
	if req.FirewallBrowserIntegrityCheck != nil {
		a.NetworkSettings.Firewall.BrowserIntegrityCheck = req.FirewallBrowserIntegrityCheck
	}
	return nil
}

// restoreNulls marks the nullable fields that arrived as an explicit null.
func restoreNulls(req *client.UpdateEnvironmentRequest, body map[string]any) {
	isNull := func(k string) bool {
		v, ok := body[k]
		return ok && v == nil
	}
	if isNull("start_command") {
		req.StartCommand = &client.NullableString{Null: true}
	}
	if isNull("nightwatch_token") {
		req.NightwatchToken = &client.NullableString{Null: true}
	}
	if isNull("hibernation_wake_up_interval") {
		req.HibernationWakeUpInterval = &client.NullableInt{Null: true}
	}
}

// withSecretsRelationship adds relationships.secrets to an encoded environment
// document, as the API does when include=secrets is requested.
func (f *fakeCloud) withSecretsRelationship(r *http.Request, envID string, doc client.Document[client.EnvironmentData]) any {
	if !strings.Contains(r.URL.Query().Get("include"), "secrets") {
		return doc
	}
	raw, _ := json.Marshal(doc)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	data := out["data"].(map[string]any)
	rels, _ := data["relationships"].(map[string]any)
	if rels == nil {
		rels = map[string]any{}
	}
	ids := make([]string, 0, len(f.secrets.attached[envID]))
	for sid := range f.secrets.attached[envID] {
		ids = append(ids, sid)
	}
	sort.Strings(ids)
	ref := make([]any, 0, len(ids))
	for _, sid := range ids {
		ref = append(ref, map[string]any{"type": "secrets", "id": sid})
	}
	rels["secrets"] = map[string]any{"data": ref}
	data["relationships"] = rels
	return out
}
