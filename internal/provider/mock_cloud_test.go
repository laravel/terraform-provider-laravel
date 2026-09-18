package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// fakeCloud is an in-memory stand-in for the Laravel Cloud API. It implements
// exactly the application + environment endpoints the provider calls during a
// plan/apply cycle, backed by maps, so plan-check tests can drive a real
// terraform plan/apply without a token or network access.
//
// It is deliberately faithful to the real API's observable behavior where that
// behavior matters to the provider:
//   - PATCH /environments/{id} with php_version "8.4:1" makes a later GET report
//     php_major_version "8.4" (the major:minor -> major transform).
//   - Empty-string build_command/deploy_command on PATCH are coalesced to null,
//     modeling an API that treats clearing an optional text field as unsetting
//     it. The provider sends "" for these unset-but-computed fields on create;
//     without this coalescing a spurious "" -> null diff would appear on the
//     next refresh. If the real API instead echoes "", that is a provider bug
//     worth surfacing — this mock assumes the common "empty clears" semantics.
type fakeCloud struct {
	mu      sync.Mutex
	seq     int
	apps    map[string]*client.ApplicationData
	envs    map[string]*client.EnvironmentData
	appEnvs map[string][]string // application ID -> environment IDs

	// Generic JSON:API store used by every resource other than applications
	// and environments (which have bespoke handlers above). Keyed by
	// type name -> object ID -> attributes. See mock_generic_test.go.
	objects  map[string]map[string]map[string]any
	children map[string][]string // "<parentKey>" -> ordered child object IDs
}

// newFakeCloud starts an httptest.Server backed by a fresh in-memory store and
// returns the store and the server's base URL. The server is closed on test
// cleanup.
func newFakeCloud(t *testing.T) (*fakeCloud, string) {
	t.Helper()

	f := &fakeCloud{
		apps:     map[string]*client.ApplicationData{},
		envs:     map[string]*client.EnvironmentData{},
		appEnvs:  map[string][]string{},
		objects:  map[string]map[string]map[string]any{},
		children: map[string][]string{},
	}

	mux := http.NewServeMux()
	// Applications
	mux.HandleFunc("POST /applications", f.createApplication)
	mux.HandleFunc("GET /applications/{id}", f.getApplication)
	mux.HandleFunc("PATCH /applications/{id}", f.updateApplication)
	mux.HandleFunc("DELETE /applications/{id}", f.deleteApplication)
	// Environments
	mux.HandleFunc("GET /applications/{id}/environments", f.listEnvironments)
	mux.HandleFunc("POST /applications/{id}/environments", f.createEnvironment)
	mux.HandleFunc("GET /environments/{id}", f.getEnvironment)
	mux.HandleFunc("PATCH /environments/{id}", f.updateEnvironment)
	mux.HandleFunc("DELETE /environments/{id}", f.deleteEnvironment)

	// All other resources are served by the generic JSON:API engine.
	f.registerGenericResources(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv.URL
}

// ---------------------------------------------------------------------------
// Application handlers
// ---------------------------------------------------------------------------

func (f *fakeCloud) createApplication(w http.ResponseWriter, r *http.Request) {
	var req client.CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.seq++
	id := fmt.Sprintf("app-%d", f.seq)
	app := &client.ApplicationData{
		ID:   id,
		Type: "applications",
		Attributes: client.ApplicationAttributes{
			Name:   req.Name,
			Slug:   slugify(req.Name),
			Region: req.Region,
		},
	}
	f.apps[id] = app
	writeJSON(w, http.StatusCreated, client.Document[client.ApplicationData]{Data: *app})
}

func (f *fakeCloud) getApplication(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	app, ok := f.apps[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("application not found"))
		return
	}
	writeJSON(w, http.StatusOK, client.Document[client.ApplicationData]{Data: *app})
}

func (f *fakeCloud) updateApplication(w http.ResponseWriter, r *http.Request) {
	var req client.UpdateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	app, ok := f.apps[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("application not found"))
		return
	}
	if req.Name != nil {
		app.Attributes.Name = *req.Name
	}
	if req.Slug != nil {
		app.Attributes.Slug = *req.Slug
	}
	if req.SlackChannel != nil {
		app.Attributes.SlackChannel = req.SlackChannel
	}
	writeJSON(w, http.StatusOK, client.Document[client.ApplicationData]{Data: *app})
}

func (f *fakeCloud) deleteApplication(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.apps, r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Environment handlers
// ---------------------------------------------------------------------------

func (f *fakeCloud) listEnvironments(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	appID := r.PathValue("id")
	out := []client.EnvironmentData{}
	for _, envID := range f.appEnvs[appID] {
		if env, ok := f.envs[envID]; ok {
			out = append(out, *env)
		}
	}
	writeJSON(w, http.StatusOK, client.ListDocument[client.EnvironmentData]{Data: out})
}

func (f *fakeCloud) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var req client.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	appID := r.PathValue("id")
	f.seq++
	id := fmt.Sprintf("env-%d", f.seq)
	// Defaults model a freshly-created environment before any settings PATCH.
	env := &client.EnvironmentData{
		ID:   id,
		Type: "environments",
		Attributes: client.EnvironmentAttributes{
			Name:            req.Name,
			Slug:            slugify(req.Name),
			Status:          "active",
			PHPMajorVersion: "8.4",
			NodeVersion:     "20",
			NetworkSettings: newEnvironmentNetwork("default"),
		},
	}
	f.envs[id] = env
	f.appEnvs[appID] = append(f.appEnvs[appID], id)
	writeJSON(w, http.StatusCreated, client.Document[client.EnvironmentData]{Data: *env})
}

func (f *fakeCloud) getEnvironment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	env, ok := f.envs[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
		return
	}
	writeJSON(w, http.StatusOK, client.Document[client.EnvironmentData]{Data: *env})
}

func (f *fakeCloud) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req client.UpdateEnvironmentRequest
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
	a := &env.Attributes
	if req.Name != nil {
		a.Name = *req.Name
	}
	if req.Slug != nil {
		a.Slug = *req.Slug
	}
	if req.NodeVersion != nil {
		a.NodeVersion = *req.NodeVersion
	}
	// cache_strategy is written at the top level but only ever read back from
	// network_settings.cache.strategy -- mirror that asymmetry faithfully.
	if req.CacheStrategy != nil {
		a.NetworkSettings = newEnvironmentNetwork(*req.CacheStrategy)
	}
	if req.UsesPushToDeploy != nil {
		a.UsesPushToDeploy = *req.UsesPushToDeploy
	}
	if req.UsesDeployHook != nil {
		a.UsesDeployHook = *req.UsesDeployHook
	}
	if req.UsesOctane != nil {
		a.UsesOctane = *req.UsesOctane
	}
	// color, timeout, sleep_timeout, shutdown_timeout and
	// uses_purge_edge_cache_on_deploy are accepted here and deliberately
	// dropped: the real API has no response attribute for any of them, and a
	// fake that echoed them back would mask a non-converging plan.
	// php_version is sent as "major:minor"; the API only ever reports the major
	// version back via php_major_version.
	if req.PHPVersion != nil {
		a.PHPMajorVersion = majorPHPVersion(*req.PHPVersion)
	}
	// Optional text fields: an empty string clears the value (reported as null).
	if req.BuildCommand != nil {
		a.BuildCommand = nilIfEmpty(*req.BuildCommand)
	}
	if req.DeployCommand != nil {
		a.DeployCommand = nilIfEmpty(*req.DeployCommand)
	}
	// Branch is write-only and not part of the API response, so it is accepted
	// and discarded.
	writeJSON(w, http.StatusOK, client.Document[client.EnvironmentData]{Data: *env})
}

func (f *fakeCloud) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.envs, r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
}

func slugify(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

func majorPHPVersion(v string) string {
	return strings.SplitN(v, ":", 2)[0]
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// newEnvironmentNetwork builds the response-side network_settings object with
// the given cache strategy.
func newEnvironmentNetwork(strategy string) client.EnvironmentNetwork {
	var n client.EnvironmentNetwork
	n.Cache.Strategy = strategy
	return n
}
