package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

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
	// appUpdates records every PATCH /applications body, in order, so tests
	// can assert on what the provider sent rather than only on its result.
	appUpdates []client.UpdateApplicationRequest
	// appCreates records every POST /applications body.
	appCreates []client.CreateApplicationRequest
	// envUpdates does the same for PATCH /environments.
	envUpdates []client.UpdateEnvironmentRequest
	// envUpdateBodies holds the same bodies as sent, where an explicit null is
	// still distinguishable from an omitted field.
	envUpdateBodies []map[string]any
	// failEnvUpdates makes that many of the next PATCH /environments calls
	// fail with a 500, to model a follow-up update that does not land.
	failEnvUpdates int
	// attachments holds each environment's attached database, cache and
	// WebSocket application by relationship name, reported only when the GET
	// asks for them with include, as the API does.
	attachments map[string]map[string]string
	// cannotHibernate marks environments whose app instance has a size that
	// cannot hibernate (a Pro size), by environment ID.
	cannotHibernate map[string]bool

	// Generic JSON:API store used by every resource other than applications
	// and environments (which have bespoke handlers above). Keyed by
	// type name -> object ID -> attributes. See mock_generic_test.go.
	objects  map[string]map[string]map[string]any
	children map[string][]string // "<parentKey>" -> ordered child object IDs
	// parents maps "<typeName>" -> child ID -> parent ID, so responses can
	// carry the JSON:API relationship the real API reports.
	parents map[string]map[string]string
	// patches records every PATCH body the generic engine received, by type,
	// so tests can assert on what the provider sent.
	patches map[string][]map[string]any

	// vars holds environment variables as the API does: environment ID -> key
	// -> value. Modelled as an upsert store rather than a whole-set replace,
	// because that is what the separate /variables/delete route implies.
	vars map[string]map[string]string
	// hideVariables makes environment responses leave out
	// environment_variables, as the API does for a token without the "View
	// environment variables" permission.
	hideVariables bool

	// secrets models the organization secrets API; see mock_secrets_test.go.
	secrets *fakeSecrets

	// busy counts, per database cluster or WebSocket server, how many more
	// requests that change it are refused as busy -- see
	// mock_resources_test.go.
	busy map[string]int

	// queueDeployments is how many of the next deployment requests the API
	// queues, as it does while the environment's resources are provisioning;
	// queued holds those queued deployments. See mock_deployments_test.go.
	queueDeployments int
	queued           map[string]*fakeQueuedDeployment

	// throttleReads makes that many of the next GETs answer 429 as Laravel
	// Cloud's rate limiter does, and throttleWrites that many of the next
	// writes answer the bare 302 to the app root it sends for a throttled
	// write.
	throttleReads, throttleWrites int
}

// throttle wraps the fake's routes with Laravel Cloud's rate-limit responses
// while throttleReads or throttleWrites last.
func (f *fakeCloud) throttle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		read := r.Method == http.MethodGet && f.throttleReads > 0
		write := r.Method != http.MethodGet && f.throttleWrites > 0
		if read {
			f.throttleReads--
		}
		if write {
			f.throttleWrites--
		}
		f.mu.Unlock()
		switch {
		case read:
			w.Header().Set("Retry-After", "0")
			w.Header().Set("X-RateLimit-Limit", "120")
			w.Header().Set("X-RateLimit-Remaining", "0")
			writeError(w, http.StatusTooManyRequests, fmt.Errorf("Too Many Attempts."))
		case write:
			w.Header().Set("Location", "https://cloud.laravel.com")
			w.WriteHeader(http.StatusFound)
		default:
			next.ServeHTTP(w, r)
		}
	})
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
		parents:  map[string]map[string]string{},
		vars:     map[string]map[string]string{},
		secrets:  newFakeSecrets(),

		busy:   map[string]int{},
		queued: map[string]*fakeQueuedDeployment{},
	}

	// The fake answers busy for a request or two, not twenty minutes, so the
	// provider need not wait between retries.
	interval := databaseBusyInterval
	databaseBusyInterval = time.Millisecond
	t.Cleanup(func() { databaseBusyInterval = interval })
	queuedInterval := queuedDeploymentPollInterval
	queuedDeploymentPollInterval = time.Millisecond
	t.Cleanup(func() { queuedDeploymentPollInterval = queuedInterval })
	wsInterval := websocketBusyInterval
	websocketBusyInterval = time.Millisecond
	t.Cleanup(func() { websocketBusyInterval = wsInterval })
	envInterval := environmentBusyInterval
	environmentBusyInterval = time.Millisecond
	t.Cleanup(func() { environmentBusyInterval = envInterval })
	cacheInterval := cacheBusyInterval
	cacheBusyInterval = time.Millisecond
	t.Cleanup(func() { cacheBusyInterval = cacheInterval })

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
	mux.HandleFunc("PUT /environments/{id}/vanity-domain", f.setVanityDomain)
	mux.HandleFunc("DELETE /environments/{id}", f.deleteEnvironment)
	mux.HandleFunc("GET /queued-deployments/{id}", f.getQueuedDeployment)

	// All other resources are served by the generic JSON:API engine.
	f.registerGenericResources(mux)
	f.registerSecretRoutes(mux)
	f.registerDatabaseUserRoutes(mux)
	f.registerEnvironmentExtraRoutes(mux)

	// Pacing would only slow the fake down; TestRateLimitedReadsPlan covers
	// what the provider does when Laravel Cloud's limits are reached.
	disablePacing(t)

	srv := httptest.NewServer(f.throttle(mux))
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
			Name:          req.Name,
			Slug:          slugify(req.Name),
			Region:        req.Region,
			Repository:    &client.ApplicationRepository{FullName: req.Repository},
			RootDirectory: req.RootDirectory,
		},
	}
	// starter_kit is create-only and not reported back.
	f.appCreates = append(f.appCreates, req)
	f.apps[id] = app

	// Like the real API, the application comes with a default environment on
	// the repository's default branch, which the API refuses to delete.
	branch := fakeDefaultBranch(req.Repository)
	env := f.newEnvironment(id, defaultEnvironmentName(branch), branch)
	app.Relationships.DefaultEnvironment = client.Relationship{
		Data: &client.ResourceIdentifier{Type: "environments", ID: env.ID},
	}
	writeJSON(w, http.StatusCreated, client.Document[client.ApplicationData]{Data: f.applicationView(r, app)})
}

// fakeDefaultBranch is the default branch of a repository. laravel/laravel's
// is really 13.x; the fake gives every other repository main.
func fakeDefaultBranch(repository string) string {
	if repository == "laravel/laravel" {
		return "13.x"
	}
	return "main"
}

// defaultEnvironmentName names an application's default environment the way
// Laravel Cloud does: "production" for a production-like branch, otherwise
// the branch with characters outside [A-Za-z0-9 _-] made spaces, so 13.x
// gives "13 x".
func defaultEnvironmentName(branch string) string {
	switch strings.ToLower(branch) {
	case "production", "prod", "main", "master", "live":
		return "production"
	}
	return strings.TrimSpace(regexp.MustCompile(`[^A-Za-z0-9 _-]`).ReplaceAllString(branch, " "))
}

// applicationView renders an application as the API does: relationships only
// when the request includes them.
func (f *fakeCloud) applicationView(r *http.Request, app *client.ApplicationData) client.ApplicationData {
	out := *app
	if !strings.Contains(r.URL.Query().Get("include"), "defaultEnvironment") {
		out.Relationships = client.ApplicationRelationships{}
	}
	return out
}

func (f *fakeCloud) getApplication(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	app, ok := f.apps[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("application not found"))
		return
	}
	writeJSON(w, http.StatusOK, client.Document[client.ApplicationData]{Data: f.applicationView(r, app)})
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
	f.appUpdates = append(f.appUpdates, req)
	if req.Name != nil {
		app.Attributes.Name = *req.Name
	}
	if req.Slug != nil {
		app.Attributes.Slug = *req.Slug
	}
	if req.SlackChannel != nil {
		app.Attributes.SlackChannel = req.SlackChannel
	}
	if req.Repository != nil {
		// Laravel Cloud clears root_directory when the new repository lacks
		// that directory. The fake's repositories hold no files, except that
		// one named like the directory's parent keeps it.
		if cur := app.Attributes.Repository; (cur == nil || cur.FullName != *req.Repository) &&
			app.Attributes.RootDirectory != nil && !strings.HasPrefix(*req.Repository, "acme/monorepo") {
			app.Attributes.RootDirectory = nil
		}
		app.Attributes.Repository = &client.ApplicationRepository{FullName: *req.Repository}
	}
	if req.DefaultEnvironmentID != nil {
		app.Relationships.DefaultEnvironment = client.Relationship{
			Data: &client.ResourceIdentifier{Type: "environments", ID: *req.DefaultEnvironmentID},
		}
	}
	writeJSON(w, http.StatusOK, client.Document[client.ApplicationData]{Data: f.applicationView(r, app)})
}

func (f *fakeCloud) deleteApplication(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Deleting an application deletes its environments with it.
	id := r.PathValue("id")
	for _, envID := range f.appEnvs[id] {
		delete(f.envs, envID)
	}
	delete(f.appEnvs, id)
	delete(f.apps, id)
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

// environmentDoc renders an environment the way the API does for
// include=application,branch: the branch is a relationship, so its name is
// only reachable through the document's included section.
func environmentDoc(env *client.EnvironmentData) client.Document[client.EnvironmentData] {
	doc := client.Document[client.EnvironmentData]{Data: *env}
	if id := env.Relationships.Branch.RelatedID(); id != "" {
		attrs, _ := json.Marshal(map[string]string{"name": env.BranchName})
		doc.Included = append(doc.Included, client.ResourceObject{
			ID:         id,
			Type:       "branches",
			Attributes: attrs,
		})
	}
	return doc
}

func (f *fakeCloud) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var req client.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	env := f.newEnvironment(r.PathValue("id"), req.Name, req.Branch)
	writeJSON(w, http.StatusCreated, environmentDoc(env))
}

// newEnvironment stores a freshly created environment. The caller holds f.mu.
func (f *fakeCloud) newEnvironment(appID, name, branch string) *client.EnvironmentData {
	f.seq++
	id := fmt.Sprintf("env-%d", f.seq)
	// Like the real API, every new environment gets a generated vanity
	// hostname in the organization's zone.
	vanity := fmt.Sprintf("%s-%s-%d.%s", appID, slugify(name), f.seq, fakeVanityZone)
	// Defaults model a freshly-created environment before any settings PATCH.
	env := &client.EnvironmentData{
		ID:   id,
		Type: "environments",
		Attributes: client.EnvironmentAttributes{
			Name:   name,
			Slug:   slugify(name),
			Status: "active",
			// The real API picks a random color; the timeouts are its defaults.
			Color:           ptr("blue"),
			Timeout:         ptr(int64(20)),
			ShutdownTimeout: ptr(int64(30)),
			// A new environment's app instance is the smallest Flex size,
			// which hibernates.
			UsesHibernation: ptr(true),
			PHPMajorVersion: "8.4",
			NodeVersion:     "20",
			VanityDomain:    &vanity,
			NetworkSettings: newEnvironmentNetwork("default"),
			Serving:         &client.EnvironmentServing{Mode: "process"},
		},
		// The real API reports the owning application under
		// relationships.application when include=application is requested.
		// Import depends on it: the import id names the environment alone.
		Relationships: client.EnvironmentRelationships{
			Application: client.Relationship{
				Data: &client.ResourceIdentifier{Type: "applications", ID: appID},
			},
			Branch: client.Relationship{
				Data: &client.ResourceIdentifier{Type: "branches", ID: "branch-" + id},
			},
		},
		// The API models branch as a relationship, so the name is only
		// reachable through the document's included section.
		BranchName: branch,
	}
	f.envs[id] = env
	f.appEnvs[appID] = append(f.appEnvs[appID], id)
	return env
}

// fakeVanityZone is the zone vanity hostnames live in. An organization's zone
// can be laravel-demo.cloud rather than laravel.cloud, so the provider must
// not assume either.
const fakeVanityZone = "laravel-demo.cloud"

func (f *fakeCloud) setVanityDomain(w http.ResponseWriter, r *http.Request) {
	var req client.UpdateVanityDomainRequest
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
	// The real API takes one DNS label and reports the full hostname.
	if !regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`).MatchString(req.Name) || len(req.Name) < 3 {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("the domain must have 63 characters or fewer, contain only lowercase letters, numbers, and dashes"))
		return
	}
	host := req.Name + "." + fakeVanityZone
	env.Attributes.VanityDomain = &host
	writeJSON(w, http.StatusOK, environmentDoc(env))
}

func (f *fakeCloud) getEnvironment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := r.PathValue("id")
	env, ok := f.envs[id]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
		return
	}
	// The real API reports the environment's variables here -- there is no GET
	// on /environments/{id}/variables, that route answers 405 -- so this is
	// where the variables resource reads them back from.
	keys := make([]string, 0, len(f.vars[id]))
	for k := range f.vars[id] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	vars := make([]client.EnvironmentVariable, 0, len(keys))
	for _, k := range keys {
		vars = append(vars, client.EnvironmentVariable{Key: k, Value: f.vars[id][k]})
	}
	env.Attributes.EnvironmentVariables = &vars
	if f.hideVariables {
		env.Attributes.EnvironmentVariables = nil
	}
	doc := environmentDoc(env)
	include := r.URL.Query().Get("include")
	for name, rel := range map[string]**client.Relationship{
		"database":             &doc.Data.Relationships.Database,
		"cache":                &doc.Data.Relationships.Cache,
		"websocketApplication": &doc.Data.Relationships.WebsocketApplication,
	} {
		if !strings.Contains(include, name) {
			continue
		}
		*rel = &client.Relationship{}
		if related := f.attachments[id][name]; related != "" {
			(*rel).Data = &client.ResourceIdentifier{ID: related}
		}
	}
	writeJSON(w, http.StatusOK, f.withSecretsRelationship(r, id, doc))
}

func (f *fakeCloud) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req client.UpdateEnvironmentRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	// A JSON null leaves a pointer field nil, which reads as "not sent". The
	// API treats null as "clear", so restore that from the raw body.
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	restoreNulls(&req, body)

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failEnvUpdates > 0 {
		f.failEnvUpdates--
		writeError(w, http.StatusInternalServerError, fmt.Errorf("server error"))
		return
	}
	if req.CacheID != nil {
		var cacheID string
		_ = json.Unmarshal(req.CacheID, &cacheID)
		if msg := f.cacheProvisioningError(cacheID); msg != "" {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("%s", msg))
			return
		}
	}
	f.envUpdateBodies = append(f.envUpdateBodies, body)

	env, ok := f.envs[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
		return
	}
	f.envUpdates = append(f.envUpdates, req)
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
		a.NetworkSettings.Cache.Strategy = *req.CacheStrategy
	}
	for name, raw := range map[string]json.RawMessage{
		"database":             req.DatabaseSchemaID,
		"cache":                req.CacheID,
		"websocketApplication": req.WebsocketApplicationID,
	} {
		if raw == nil {
			continue
		}
		var related string
		_ = json.Unmarshal(raw, &related) // null leaves it empty: detached
		if f.attachments == nil {
			f.attachments = map[string]map[string]string{}
		}
		if f.attachments[env.ID] == nil {
			f.attachments[env.ID] = map[string]string{}
		}
		f.attachments[env.ID][name] = related
	}
	if err := f.applyEnvironmentSettings(env, req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	// sleep_timeout is never reported, but it turns sleep mode on -- unless
	// the size cannot hibernate, in which case the real API turns sleep mode
	// and the wake-up interval off and still answers 200. An interval sent
	// without sleep_timeout is stored either way (applied above).
	if req.SleepTimeout != nil {
		a.UsesHibernation = ptr(!f.cannotHibernate[env.ID])
		if f.cannotHibernate[env.ID] {
			a.HibernationWakeUpInterval = nil
		}
	}
	if req.UsesPushToDeploy != nil {
		a.UsesPushToDeploy = *req.UsesPushToDeploy
	}
	if req.UsesDeployHook != nil {
		a.UsesDeployHook = *req.UsesDeployHook
		a.DeployHookURL = nil
		if a.UsesDeployHook {
			a.DeployHookURL = ptr("https://cloud.laravel.com/deploy-hooks/" + env.ID + "/token")
		}
	}
	if req.UsesOctane != nil {
		a.UsesOctane = *req.UsesOctane
	}
	if req.Color != nil {
		a.Color = req.Color
	}
	if req.Timeout != nil {
		a.Timeout = ptr(int64(*req.Timeout))
	}
	if req.ShutdownTimeout != nil {
		a.ShutdownTimeout = ptr(int64(*req.ShutdownTimeout))
	}
	// uses_purge_edge_cache_on_deploy is accepted here and deliberately
	// dropped: the real API has no response attribute for it, and a fake that
	// echoed it back would mask a non-converging plan.
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
	writeJSON(w, http.StatusOK, environmentDoc(env))
}

func (f *fakeCloud) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := r.PathValue("id")
	for _, app := range f.apps {
		if app.Relationships.DefaultEnvironment.RelatedID() == id {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("You cannot delete the production environment. "+
				"To delete it, first set another environment as the production environment."))
			return
		}
	}
	delete(f.envs, id)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// disablePacing turns off the provider's client-side rate limits for a test
// against a fake API.
func disablePacing(t *testing.T) {
	t.Setenv("LARAVEL_CLOUD_MAX_REQUESTS_PER_MINUTE", "0")
	t.Setenv("LARAVEL_CLOUD_MAX_CREATE_REQUESTS_PER_MINUTE", "0")
}

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
// the given cache strategy and the platform's defaults for everything else.
func newEnvironmentNetwork(strategy string) client.EnvironmentNetwork {
	var n client.EnvironmentNetwork
	n.Cache.Strategy = strategy
	n.ResponseHeaders = client.EnvironmentResponseHeaders{
		Frame:       ptr("deny"),
		ContentType: ptr("nosniff"),
		RobotsTag:   ptr("index, follow"),
		HSTS:        &client.HSTS{},
	}
	n.Firewall = client.EnvironmentFirewall{BlockPath: ptr(true), BrowserIntegrityCheck: ptr(true)}
	return n
}

func ptr[T any](v T) *T { return &v }
