package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// This file implements a small generic JSON:API store used to mock every
// Laravel Cloud resource other than applications and environments (which have
// bespoke handlers in mock_cloud_test.go).
//
// Each resource is described by a crudSpec: the URL patterns it lives at, how a
// created object's attributes are seeded (computed/default fields the API would
// return), and how a PATCH mutates them. The engine stores attributes as plain
// maps and echoes them back unchanged on every read, so an apply followed by a
// re-plan is a no-op by construction — unless a seed/patch deliberately models
// a transform (e.g. an API that reports a value differently than it was sent),
// which is exactly where real provider round-trip bugs would surface.

// crudSpec describes one resource's mock endpoints.
type crudSpec struct {
	typeName string // JSON:API "type" and store key
	idPrefix string // generated IDs look like "<idPrefix>-<n>"

	// collectionPattern is the path (no method) for create/list, e.g.
	// "/caches" or "/environments/{eid}/instances".
	collectionPattern string
	// itemPattern is the path for get/patch/delete, e.g. "/caches/{id}" or
	// "/buckets/{bid}/keys/{id}".
	itemPattern string
	// parentWildcard names the path value holding the parent ID in the
	// collection/list patterns (e.g. "eid"); empty for top-level resources.
	parentWildcard string

	list  bool // register GET on the collection
	patch bool // register PATCH on the item
	del   bool // register DELETE on the item

	// createRespID, if set, overrides the id returned in the POST response while
	// the object is still stored (and listed) under the generated id. Models an
	// API that returns a different id on create than it stores — e.g.
	// POST /databases/clusters/{cid}/databases returns the cluster id, not the
	// new schema id (see resource_database.go).
	createRespID func(storedID, parentID string) string

	// deletePattern, if set, registers DELETE at this path instead of itemPattern.
	// Models an API where the flat item route does not accept DELETE and the
	// resource must use a parent-nested route — e.g.
	// DELETE /databases/clusters/{cid}/databases/{id} (the flat /databases/{id}
	// route does not accept DELETE). It still deletes by the "{id}" path value.
	deletePattern string

	// seed produces the stored attributes for a newly created object from the
	// decoded request body and parent ID. It owns which computed/default
	// fields the API "returns". If nil, the body is stored verbatim.
	seed func(body map[string]any, parentID string) map[string]any
	// patchFn applies a PATCH body to the stored attributes. If nil, every key
	// present in the body is merged in (overwriting).
	patchFn func(attrs, patch map[string]any)

	// createOnlyAttrs names attributes the API returns when the object is
	// created and omits from every later response -- credentials, typically.
	// They are stripped from GET, PATCH and list responses. Modelling this is
	// what makes it possible to catch a provider that treats their absence as
	// an empty value and overwrites the stored secret.
	createOnlyAttrs []string
}

// withoutCreateOnly returns a copy of attrs with the create-only attributes
// removed, or attrs itself when there are none.
func (s crudSpec) withoutCreateOnly(attrs map[string]any) map[string]any {
	if len(s.createOnlyAttrs) == 0 {
		return attrs
	}
	out := cloneMap(attrs)
	for _, k := range s.createOnlyAttrs {
		delete(out, k)
	}
	return out
}

// registerCRUD wires a crudSpec's handlers onto the mux.
func (f *fakeCloud) registerCRUD(mux *http.ServeMux, s crudSpec) {
	mux.HandleFunc("POST "+s.collectionPattern, f.genCreate(s))
	mux.HandleFunc("GET "+s.itemPattern, f.genGet(s))
	if s.list {
		mux.HandleFunc("GET "+s.collectionPattern, f.genList(s))
	}
	if s.patch {
		mux.HandleFunc("PATCH "+s.itemPattern, f.genPatch(s))
	}
	if s.del {
		deletePattern := s.itemPattern
		if s.deletePattern != "" {
			deletePattern = s.deletePattern
		}
		mux.HandleFunc("DELETE "+deletePattern, f.genDelete(s))
	}
}

func (f *fakeCloud) genCreate(s crudSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)

		f.mu.Lock()
		defer f.mu.Unlock()

		parentID := ""
		if s.parentWildcard != "" {
			parentID = r.PathValue(s.parentWildcard)
		}

		f.seq++
		id := fmt.Sprintf("%s-%d", s.idPrefix, f.seq)

		var attrs map[string]any
		if s.seed != nil {
			attrs = s.seed(body, parentID)
		} else {
			attrs = body
		}

		if f.objects[s.typeName] == nil {
			f.objects[s.typeName] = map[string]map[string]any{}
		}
		f.objects[s.typeName][id] = attrs
		if s.parentWildcard != "" {
			key := s.typeName + ":" + parentID
			f.children[key] = append(f.children[key], id)
		}

		respID := id
		if s.createRespID != nil {
			respID = s.createRespID(id, parentID)
		}
		writeJSON(w, http.StatusCreated, jsonAPIObject(respID, s.typeName, attrs))
	}
}

func (f *fakeCloud) genGet(s crudSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		id := r.PathValue("id")
		attrs, ok := f.objects[s.typeName][id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("%s not found", s.typeName))
			return
		}
		writeJSON(w, http.StatusOK, jsonAPIObject(id, s.typeName, s.withoutCreateOnly(attrs)))
	}
}

func (f *fakeCloud) genPatch(s crudSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		patch := decodeBody(r)

		f.mu.Lock()
		defer f.mu.Unlock()

		id := r.PathValue("id")
		attrs, ok := f.objects[s.typeName][id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("%s not found", s.typeName))
			return
		}
		if s.patchFn != nil {
			s.patchFn(attrs, patch)
		} else {
			for k, v := range patch {
				attrs[k] = v
			}
		}
		writeJSON(w, http.StatusOK, jsonAPIObject(id, s.typeName, attrs))
	}
}

func (f *fakeCloud) genDelete(s crudSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		delete(f.objects[s.typeName], r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	}
}

func (f *fakeCloud) genList(s crudSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		parentID := ""
		if s.parentWildcard != "" {
			parentID = r.PathValue(s.parentWildcard)
		}

		out := []any{}
		if s.parentWildcard != "" {
			for _, id := range f.children[s.typeName+":"+parentID] {
				if attrs, ok := f.objects[s.typeName][id]; ok {
					out = append(out, jsonAPIData(id, s.typeName, s.withoutCreateOnly(attrs)))
				}
			}
		} else {
			for id, attrs := range f.objects[s.typeName] {
				out = append(out, jsonAPIData(id, s.typeName, s.withoutCreateOnly(attrs)))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": out})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// decodeBody reads a JSON object body, tolerating an empty body (some create
// endpoints, e.g. deployments, send none).
func decodeBody(r *http.Request) map[string]any {
	body := map[string]any{}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	return body
}

func jsonAPIData(id, typeName string, attrs map[string]any) map[string]any {
	return map[string]any{"id": id, "type": typeName, "attributes": attrs}
}

func jsonAPIObject(id, typeName string, attrs map[string]any) map[string]any {
	return map[string]any{"data": jsonAPIData(id, typeName, attrs)}
}
