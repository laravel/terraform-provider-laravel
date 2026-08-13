package provider

import (
	"fmt"
	"net/http"
)

// clusterType is the store key for database clusters. database_restore creates
// a new cluster object that the normal cluster GET/DELETE handlers then serve,
// so both must agree on this key.
const clusterType = "database_cluster"

// registerGenericResources wires every resource other than applications and
// environments onto the mux: the standard CRUD resources via crudSpec, plus the
// two endpoints (environment variables, database restore) that don't fit the
// generic shape.
func (f *fakeCloud) registerGenericResources(mux *http.ServeMux) {
	specs := []crudSpec{
		// -------- top-level resources --------
		{
			typeName: "cache", idPrefix: "cache",
			collectionPattern: "/caches", itemPattern: "/caches/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["status"] = "available"
				a["connection"] = map[string]any{
					"hostname": "cache.test.local", "port": 6379,
					"protocol": "tcp", "username": "default", "password": "cache-secret",
				}
				return a
			},
		},
		{
			typeName: clusterType, idPrefix: "cluster",
			collectionPattern: "/databases/clusters", itemPattern: "/databases/clusters/{id}",
			patch: true, del: true,
			seed: seedCluster,
		},
		{
			typeName: "storage_bucket", idPrefix: "bucket",
			collectionPattern: "/buckets", itemPattern: "/buckets/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["status"] = "available"
				return a
			},
		},
		{
			typeName: "websocket_server", idPrefix: "wss",
			collectionPattern: "/websocket-servers", itemPattern: "/websocket-servers/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["status"] = "available"
				a["hostname"] = "ws.test.local"
				a["connection_distribution_strategy"] = "round_robin"
				return a
			},
		},
		// -------- environment-nested --------
		{
			typeName: "instance", idPrefix: "instance", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/instances", itemPattern: "/instances/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				// Echo the body; managed-queue / status pointer fields are left
				// absent so they decode to null (not zero values).
				return cloneMap(body)
			},
		},
		{
			typeName: "domain", idPrefix: "domain", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/domains", itemPattern: "/domains/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				// The request sends www_redirect; the API echoes it as `redirect`.
				if v, ok := body["www_redirect"]; ok {
					a["redirect"] = v
				}
				a["type"] = "root"
				a["hostname_status"] = "pending"
				a["ssl_status"] = "pending"
				a["origin_status"] = "pending"
				return a
			},
		},
		{
			typeName: "deployment", idPrefix: "deploy", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/deployments", itemPattern: "/deployments/{id}",
			// No PATCH, no DELETE (deployments are immutable and cannot be undone).
			seed: func(_ map[string]any, _ string) map[string]any {
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
			},
		},
		{
			typeName: "command", idPrefix: "command", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/commands", itemPattern: "/commands/{id}",
			// No PATCH, no DELETE (commands are immutable and cannot be undone).
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body) // echoes `command` back unchanged
				a["status"] = "pending"
				return a
			},
		},
		// -------- deeply nested --------
		{
			typeName: "database", idPrefix: "db-schema", parentWildcard: "cid",
			// A "database" in the API is the cluster; the thing inside it is a schema,
			// reachable only through the cluster-nested item route. The flat
			// /databases/{id} is the deprecated *cluster* show route, so a schema id
			// legitimately 404s there -- which is why Read used to scan the listing
			// (issue #30). list stays on for the resource's adopt-by-name logic.
			collectionPattern: "/databases/clusters/{cid}/databases",
			itemPattern:       "/databases/clusters/{cid}/databases/{id}",
			list:              true, del: true,
			// Dev's POST returns the *cluster* id, not the new schema id, while the
			// schema is addressable (and listed) under its real "db-schema-*" id.
			// The resource must resolve the canonical id from the listing rather
			// than trusting the create response (issue #30).
			createRespID: func(_, parentID string) string { return parentID },
			// Dev only deletes a schema via the cluster-nested route; the flat
			// DELETE /databases/{id} 302s. The resource must use the nested route.
			deletePattern: "/databases/clusters/{cid}/databases/{id}",
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["status"] = "available"
				return a
			},
		},
		{
			typeName: "storage_bucket_key", idPrefix: "key", parentWildcard: "bid",
			// Only list and create are nested under the bucket; every per-item route
			// is flat, at /bucket-keys/{id}. The nested item forms are not registered
			// on the API at all: the GET 404s and the PATCH/DELETE fall through to the
			// web catch-all (302 to the app root), which made keys unupdatable and
			// undeletable, and left the bucket undeletable behind them (issue #31).
			collectionPattern: "/buckets/{bid}/keys", itemPattern: "/bucket-keys/{id}",
			list: true, patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["access_key_id"] = "AKIATEST0001"
				a["access_key_secret"] = "bucket-key-secret"
				return a
			},
		},
		{
			typeName: "background_process", idPrefix: "bgp", parentWildcard: "iid",
			collectionPattern: "/instances/{iid}/background-processes", itemPattern: "/background-processes/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				return cloneMap(body) // config is ignored on read-back by the provider
			},
		},
		{
			// Only list/create are nested under the server; the per-item routes are
			// flat. The nested GET 404s and the nested PATCH/DELETE fall through to
			// the web catch-all (302 to the app root), which dropped the application
			// from state and recreated it into a 422 name conflict.
			typeName: "websocket_application", idPrefix: "wsa", parentWildcard: "sid",
			collectionPattern: "/websocket-servers/{sid}/applications", itemPattern: "/websocket-applications/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["app_id"] = "wsapp-0001"
				a["key"] = "ws-key-0001"
				a["secret"] = "ws-secret-0001"
				a["max_message_size"] = 10000
				a["max_connections"] = 1000
				if _, ok := a["ping_interval"]; !ok {
					a["ping_interval"] = 60
				}
				if _, ok := a["activity_timeout"]; !ok {
					a["activity_timeout"] = 120
				}
				return a
			},
		},
		{
			typeName: "database_snapshot", idPrefix: "snap", parentWildcard: "cid",
			collectionPattern: "/databases/clusters/{cid}/snapshots", itemPattern: "/database-snapshots/{id}",
			del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["type"] = "manual"
				a["status"] = "available"
				a["pitr_enabled"] = false
				a["storage_bytes"] = 0
				return a
			},
		},
	}

	for _, s := range specs {
		f.registerCRUD(mux, s)
	}

	// -------- bespoke endpoints --------

	// Environment variables: a single "set" POST replaces the whole set; the
	// response is discarded by the provider (no GET endpoint exists).
	mux.HandleFunc("POST /environments/{id}/variables", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, jsonAPIObject(r.PathValue("id"), "environments", map[string]any{}))
	})
	mux.HandleFunc("POST /environments/{id}/variables/delete", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Database restore: creates a NEW cluster (returned as DatabaseClusterData)
	// that the normal cluster GET/DELETE handlers then serve. It must store the
	// object under clusterType so those routes find it.
	mux.HandleFunc("POST /databases/clusters/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)

		f.mu.Lock()
		defer f.mu.Unlock()

		f.seq++
		id := fmt.Sprintf("cluster-%d", f.seq)
		attrs := map[string]any{
			"name":   body["name"],
			"type":   "laravel_mysql_84",
			"region": "us-east-1",
			"status": "creating",
			"config": map[string]any{"size": "db-1vcpu-1gb"},
			"connection": map[string]any{
				"hostname": "db.test.local", "port": 3306, "protocol": "tcp",
				"driver": "mysql", "username": "forge", "password": "db-secret",
			},
		}
		if f.objects[clusterType] == nil {
			f.objects[clusterType] = map[string]map[string]any{}
		}
		f.objects[clusterType][id] = attrs
		writeJSON(w, http.StatusCreated, jsonAPIObject(id, clusterType, attrs))
	})
}

// seedCluster builds the stored attributes for a database cluster, including a
// connection object (with the cluster-only `driver` field) and a stable status.
// The `config` field is echoed from the request as a map so the provider's
// Read, which re-marshals it to a JSON string, round-trips cleanly.
func seedCluster(body map[string]any, _ string) map[string]any {
	a := cloneMap(body)
	a["status"] = "available"
	a["connection"] = map[string]any{
		"hostname": "db.test.local", "port": 3306, "protocol": "tcp",
		"driver": "mysql", "username": "forge", "password": "db-secret",
	}
	return a
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+4)
	for k, v := range m {
		out[k] = v
	}
	return out
}
