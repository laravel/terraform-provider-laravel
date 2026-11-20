package provider

import (
	"fmt"
	"net/http"

	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// queuedDeploymentPendingReads is how many reads a queued deployment stays
// pending before the fake promotes it, as the API does once the environment's
// resources finish provisioning.
const queuedDeploymentPendingReads = 2

// fakeQueuedDeployment is a deployment the API queued.
type fakeQueuedDeployment struct {
	envID        string
	pendingReads int
	// deploymentID is the deployment it started, once promoted.
	deploymentID string
}

// seedDeployment is what the API reports for a new deployment.
func seedDeployment(_ map[string]any, _ string) map[string]any {
	return map[string]any{
		"status":            "pending",
		"branch_name":       "main",
		"commit_hash":       "abc1234",
		"commit_message":    "Initial deploy",
		"commit_author":     "tester",
		"php_major_version": "8.4",
		"build_command":     "",
		"node_version":      "20",
		"uses_octane":       false,
	}
}

// queueDeploymentCreate wraps the deployment create. While f.queueDeployments
// is positive, the request is queued the way the API queues one while the
// environment's resources are still provisioning: a 202 with a queued
// deployment, whose qdepl- id GET /deployments does not know.
func queueDeploymentCreate(f *fakeCloud, create http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		if f.queueDeployments == 0 {
			f.mu.Unlock()
			create(w, r)
			return
		}
		defer f.mu.Unlock()
		f.queueDeployments--
		id := f.addQueuedDeployment(r.PathValue("eid"))
		w.Header().Set("Content-Location", "/queued-deployments/"+id)
		writeJSON(w, http.StatusAccepted, map[string]any{"data": f.queuedDeploymentData(id)})
	}
}

// addQueuedDeployment stores a pending queued deployment. The caller holds f.mu.
func (f *fakeCloud) addQueuedDeployment(envID string) string {
	f.seq++
	id := fmt.Sprintf("qdepl-%d", f.seq)
	f.queued[id] = &fakeQueuedDeployment{envID: envID, pendingReads: queuedDeploymentPendingReads}
	return id
}

func (f *fakeCloud) getQueuedDeployment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := r.PathValue("id")
	q, ok := f.queued[id]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("queued deployment not found"))
		return
	}
	if q.deploymentID == "" {
		if q.pendingReads > 0 {
			q.pendingReads--
		} else {
			// Promote it: start a real deployment on the environment.
			f.seq++
			q.deploymentID = fmt.Sprintf("deploy-%d", f.seq)
			if f.objects["deployment"] == nil {
				f.objects["deployment"] = map[string]map[string]any{}
			}
			f.objects["deployment"][q.deploymentID] = seedDeployment(nil, q.envID)
			f.children["deployment:"+q.envID] = append(f.children["deployment:"+q.envID], q.deploymentID)
			if f.parents["deployment"] == nil {
				f.parents["deployment"] = map[string]string{}
			}
			f.parents["deployment"][q.deploymentID] = q.envID
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": f.queuedDeploymentData(id)})
}

// queuedDeploymentData is a queued deployment as the API reports it with
// include=environment,deployment. The caller holds f.mu.
func (f *fakeCloud) queuedDeploymentData(id string) map[string]any {
	q := f.queued[id]
	status := "pending"
	rels := relationshipTo("environment", "environments", q.envID)
	if q.deploymentID != "" {
		status = "promoted"
		rels["deployment"] = map[string]any{"data": map[string]any{"type": "deployments", "id": q.deploymentID}}
	}
	d := jsonAPIData(id, client.QueuedDeploymentType, map[string]any{
		"status":         status,
		"branch_name":    "main",
		"commit_hash":    "abc1234",
		"commit_message": "Initial deploy",
		"commit_author":  "tester",
		"failure_reason": nil,
	})
	d["relationships"] = rels
	return d
}

// startEnvironment starts a stopped environment the way the API does: it
// answers with a new deployment, and the environment stays stopped until that
// deployment succeeds (see finishStartDeployment).
func (f *fakeCloud) startEnvironment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	envID := r.PathValue("id")
	env, ok := f.envs[envID]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
		return
	}
	if env.Attributes.Status != "stopped" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("The environment is already running."))
		return
	}
	f.seq++
	id := fmt.Sprintf("deploy-%d", f.seq)
	attrs := seedDeployment(nil, envID)
	attrs["status"] = "build.running"
	if f.objects["deployment"] == nil {
		f.objects["deployment"] = map[string]map[string]any{}
	}
	f.objects["deployment"][id] = attrs
	f.children["deployment:"+envID] = append(f.children["deployment:"+envID], id)
	if f.parents["deployment"] == nil {
		f.parents["deployment"] = map[string]string{}
	}
	f.parents["deployment"][id] = envID
	writeJSON(w, http.StatusOK, jsonAPIObject(id, "deployments", attrs))
}

// finishStartDeployment moves a deployment that starts an environment on to
// succeeded the first time it is read, and the environment to running.
func finishStartDeployment(f *fakeCloud, id string) {
	attrs := f.objects["deployment"][id]
	if attrs["status"] != "build.running" {
		return
	}
	attrs["status"] = "deployment.succeeded"
	if env := f.envs[f.parents["deployment"][id]]; env != nil {
		env.Attributes.Status = "running"
	}
}

// stopEnvironment stops an environment, which the API refuses for one that
// has never been deployed or is already stopped.
func (f *fakeCloud) stopEnvironment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := r.PathValue("id")
	env, ok := f.envs[id]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("environment not found"))
		return
	}
	if len(f.children["deployment:"+id]) == 0 {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("The environment does not have a current deployment."))
		return
	}
	if env.Attributes.Status == "stopped" {
		writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("The environment is already stopped."))
		return
	}
	env.Attributes.Status = "stopped"
	writeJSON(w, http.StatusOK, environmentDoc(env))
}
