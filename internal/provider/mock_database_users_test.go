package provider

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
)

// databaseUserType is the store key for database users. Each stored user has
// "username", "type", "status", "password" and "databases" ([]string), and its
// cluster in f.parents.
const databaseUserType = "database_user"

// databaseUserBusyMessage is the 422 the user routes answer while the cluster
// is busy.
const databaseUserBusyMessage = "An update is already in progress for this database cluster. Try again in a few seconds."

var fakeUsernameRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// registerDatabaseUserRoutes serves /databases/clusters/{cid}/users the way
// the live API did:
//   - a new user is "creating" and becomes "available" on a later read;
//   - only the create response carries the password;
//   - the databases a user may access are a to-many relationship, reported
//     only with include=databases, and a deleted database drops out of it;
//   - the cluster's superuser is listed but cannot be changed or deleted;
//   - every write is refused with a 422 while the cluster is busy.
func (f *fakeCloud) registerDatabaseUserRoutes(mux *http.ServeMux) {
	const collection = "/databases/clusters/{cid}/users"
	const item = collection + "/{id}"

	mux.HandleFunc("POST "+collection, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		f.mu.Lock()
		defer f.mu.Unlock()

		clusterID := r.PathValue("cid")
		if _, ok := f.objects[clusterType][clusterID]; !ok {
			writeError(w, http.StatusNotFound, errors.New("database cluster not found"))
			return
		}
		username, _ := body["username"].(string)
		if !fakeUsernameRe.MatchString(username) || len(username) < 3 {
			writeError(w, http.StatusUnprocessableEntity, errors.New("The username must start with a letter or underscore and may only contain lowercase letters, numbers, and underscores."))
			return
		}
		for _, id := range f.children[databaseUserType+":"+clusterID] {
			if u, ok := f.objects[databaseUserType][id]; ok && u["username"] == username {
				writeError(w, http.StatusUnprocessableEntity, errors.New("This username is unavailable. Choose another."))
				return
			}
		}
		databases, msg := f.validateUserAccess(clusterID, body)
		if msg != "" {
			writeError(w, http.StatusUnprocessableEntity, errors.New(msg))
			return
		}
		if f.refuseIfBusy(clusterID) {
			writeError(w, http.StatusUnprocessableEntity, errors.New(databaseUserBusyMessage))
			return
		}

		f.seq++
		id := fmt.Sprintf("dbu-%d", f.seq)
		f.storeDatabaseUser(clusterID, id, map[string]any{
			"username":   username,
			"type":       body["type"],
			"status":     "creating",
			"password":   fmt.Sprintf("password-%d", f.seq),
			"created_at": "2026-01-01T00:00:00.000000Z",
			"databases":  databases,
		})
		writeJSON(w, http.StatusCreated, map[string]any{"data": f.databaseUserView(r, id, true)})
	})

	mux.HandleFunc("GET "+collection, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := []any{}
		for _, id := range f.children[databaseUserType+":"+r.PathValue("cid")] {
			if _, ok := f.objects[databaseUserType][id]; ok {
				out = append(out, f.databaseUserView(r, id, false))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	})

	mux.HandleFunc("GET "+item, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id, ok := f.findDatabaseUser(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": f.databaseUserView(r, id, false)})
		// The cluster finishes creating the user a few seconds later.
		if u := f.objects[databaseUserType][id]; u["status"] == "creating" {
			u["status"] = "available"
		}
	})

	mux.HandleFunc("PATCH "+item, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		id, ok := f.findDatabaseUser(w, r)
		if !ok {
			return
		}
		u := f.objects[databaseUserType][id]
		if f.patches == nil {
			f.patches = map[string][]map[string]any{}
		}
		f.patches[databaseUserType] = append(f.patches[databaseUserType], body)
		clusterID := r.PathValue("cid")
		databases, msg := f.validateUserAccess(clusterID, body)
		switch {
		case msg != "":
		case u["type"] == "superuser":
			msg = "The superuser cannot be updated."
		case f.refuseIfBusy(clusterID):
			msg = databaseUserBusyMessage
		}
		if msg != "" {
			writeError(w, http.StatusUnprocessableEntity, errors.New(msg))
			return
		}
		u["type"] = body["type"]
		u["databases"] = databases
		writeJSON(w, http.StatusOK, map[string]any{"data": f.databaseUserView(r, id, false)})
	})

	mux.HandleFunc("DELETE "+item, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id, ok := f.findDatabaseUser(w, r)
		if !ok {
			return
		}
		if f.objects[databaseUserType][id]["type"] == "superuser" {
			writeError(w, http.StatusUnprocessableEntity, errors.New("The superuser cannot be deleted."))
			return
		}
		if f.refuseIfBusy(r.PathValue("cid")) {
			writeError(w, http.StatusUnprocessableEntity, errors.New(databaseUserBusyMessage))
			return
		}
		delete(f.objects[databaseUserType], id)
		w.WriteHeader(http.StatusNoContent)
	})
}

// validateUserAccess checks a create or update body's type and databases, as
// the API's form requests do: databases is required (an empty list grants
// none), and every entry must be a database in this cluster.
func (f *fakeCloud) validateUserAccess(clusterID string, body map[string]any) ([]string, string) {
	if t := body["type"]; t != "read_write" && t != "read_only" {
		return nil, "The selected type is invalid."
	}
	list, ok := body["databases"].([]any)
	if !ok {
		return nil, "The databases field must be present."
	}
	databases := []string{}
	for _, d := range list {
		id, _ := d.(string)
		if f.parents["database"][id] != clusterID {
			return nil, "One of the selected databases is no longer available."
		}
		if _, ok := f.objects["database"][id]; !ok {
			return nil, "One of the selected databases is no longer available."
		}
		databases = append(databases, id)
	}
	return databases, ""
}

// findDatabaseUser resolves the {cid}/{id} path to a stored user, answering
// 404 when there is none.
func (f *fakeCloud) findDatabaseUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if _, ok := f.objects[databaseUserType][id]; !ok || f.parents[databaseUserType][id] != r.PathValue("cid") {
		writeError(w, http.StatusNotFound, fmt.Errorf("No query results for model [App\\Models\\DatabaseUser] %s", id))
		return "", false
	}
	return id, true
}

func (f *fakeCloud) storeDatabaseUser(clusterID, id string, attrs map[string]any) {
	if f.objects[databaseUserType] == nil {
		f.objects[databaseUserType] = map[string]map[string]any{}
	}
	f.objects[databaseUserType][id] = attrs
	key := databaseUserType + ":" + clusterID
	f.children[key] = append(f.children[key], id)
	if f.parents[databaseUserType] == nil {
		f.parents[databaseUserType] = map[string]string{}
	}
	f.parents[databaseUserType][id] = clusterID
}

// addClusterSuperuser stores the user every new cluster comes with.
func (f *fakeCloud) addClusterSuperuser(clusterID string) {
	f.seq++
	f.storeDatabaseUser(clusterID, fmt.Sprintf("dbu-%d", f.seq), map[string]any{
		"username":   "forge",
		"type":       "superuser",
		"status":     "available",
		"created_at": "2026-01-01T00:00:00.000000Z",
		"databases":  []string{},
	})
}

// databaseUserView renders a stored user as the API does: the password only
// on create, and the databases relationship only with include=databases,
// leaving out databases deleted since.
func (f *fakeCloud) databaseUserView(r *http.Request, id string, withPassword bool) map[string]any {
	u := f.objects[databaseUserType][id]
	attrs := map[string]any{
		"username":   u["username"],
		"type":       u["type"],
		"status":     u["status"],
		"created_at": u["created_at"],
	}
	if withPassword {
		attrs["password"] = u["password"]
	}
	d := jsonAPIData(id, "databaseUsers", attrs)
	if r.URL.Query().Get("include") == "databases" {
		ids, _ := u["databases"].([]string)
		sort.Strings(ids)
		related := []any{}
		for _, dbID := range ids {
			if _, ok := f.objects["database"][dbID]; ok {
				related = append(related, map[string]any{"type": "databaseSchemas", "id": dbID})
			}
		}
		d["relationships"] = map[string]any{"databases": map[string]any{"data": related}}
	}
	return d
}
