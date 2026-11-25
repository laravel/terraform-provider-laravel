package provider

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strings"
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
			validateCreate: func(body map[string]any) string {
				cacheType, _ := body["type"].(string)
				size, _ := body["size"].(string)
				return cacheHibernationError(body, cacheType, size)
			},
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				// A new cache reports its connection details while it is
				// still "creating", which it stays for two more reads (see
				// beforeGet). Until then it refuses updates (see
				// validatePatch) and environments refuse to attach it (see
				// cacheProvisioningError).
				a["status"] = "creating"
				a["connection"] = map[string]any{
					"hostname": "cache.test.local", "port": 6379,
					"protocol": "tcp", "username": "default", "password": "cache-secret",
				}
				// The API's defaults: allkeys-lru for Valkey (an empty
				// string arrives as null), and scale to zero off unless
				// asked for, whatever the size.
				if p, _ := a["eviction_policy"].(string); p == "" {
					a["eviction_policy"] = nil
					if a["type"] == "laravel_valkey" {
						a["eviction_policy"] = "allkeys-lru"
					}
				}
				if a["uses_hibernation"] != true {
					a["uses_hibernation"] = false
					a["hibernation_timeout"] = nil
				}
				// ElastiCache caches are always private, whatever is sent,
				// and never report a hostname, even once available.
				if t, _ := a["type"].(string); strings.HasPrefix(t, "aws_elasticache_") {
					a["is_public"] = false
					a["connection"].(map[string]any)["hostname"] = nil
				}
				return a
			},
			afterCreate: func(f *fakeCloud, id string) { f.busy[id] = 2 },
			beforeGet: func(f *fakeCloud, id string) {
				attrs := f.objects["cache"][id]
				if !f.refuseIfBusy(id) && attrs["status"] == "creating" {
					attrs["status"] = "available"
				}
			},
			// A refused request stands for the time the cache takes to
			// finish, so the next one succeeds.
			validatePatch: func(attrs, patch map[string]any) string {
				if s := attrs["status"]; s == "creating" || s == "updating" {
					attrs["status"] = "available"
					return "An update is already in progress for this cache. Try again in a few seconds."
				}
				size, _ := attrs["size"].(string)
				if s, ok := patch["size"].(string); ok {
					size = s
				}
				cacheType, _ := attrs["type"].(string)
				// Only Laravel Valkey has a public endpoint setting.
				if p, ok := patch["is_public"]; ok && cacheType != "laravel_valkey" && p != attrs["is_public"] {
					return "Public endpoint settings are not available for this cache type."
				}
				return cacheHibernationError(patch, cacheType, size)
			},
			// With scale to zero off the API clears the timeout. An update
			// leaves the cache "updating" until a refused request.
			patchFn: func(attrs, patch map[string]any) {
				for k, v := range patch {
					attrs[k] = v
				}
				if attrs["uses_hibernation"] != true {
					attrs["hibernation_timeout"] = nil
				}
				attrs["status"] = "updating"
			},
		},
		{
			typeName: clusterType, idPrefix: "cluster",
			collectionPattern: "/databases/clusters", itemPattern: "/databases/clusters/{id}",
			patch: true, del: true,
			seed: seedCluster,
			// The API refuses to shrink storage, which matters once
			// autoscaling has grown it past the configured value.
			validatePatch: func(attrs, patch map[string]any) string {
				cfg, _ := attrs["config"].(map[string]any)
				body, _ := patch["config"].(map[string]any)
				want, sent := body["storage"].(float64)
				if have, ok := cfg["storage"].(float64); sent && ok && want < have {
					return "The storage value must be between the current value and 1000."
				}
				return ""
			},
			// The API merges the sent config over the cluster's current one.
			patchFn: func(attrs, patch map[string]any) {
				cfg, _ := attrs["config"].(map[string]any)
				merged := cloneMap(cfg)
				body, _ := patch["config"].(map[string]any)
				for k, v := range body {
					merged[k] = v
				}
				attrs["config"] = merged
			},
			// The platform creates a database inside every new cluster unless
			// create_default_database is false. It is not managed by
			// Terraform, and it is what makes a cluster impossible to delete
			// without sweeping its databases first.
			afterCreate: func(f *fakeCloud, clusterID string) {
				attrs := f.objects[clusterType][clusterID]
				createDefault, sent := attrs["create_default_database"]
				// A create-only request field: never reported back.
				delete(attrs, "create_default_database")
				// A new cluster is "creating", which the API counts as busy.
				f.busy[clusterID] = clusterCreatingRefusals
				f.addClusterSuperuser(clusterID)
				if sent && createDefault == false {
					return
				}
				f.addClusterDatabase(clusterID, autoCreatedDatabaseName)
			},
			beforeDelete: func(f *fakeCloud, clusterID string) string {
				if f.refuseIfBusy(clusterID) {
					return clusterBusyMessage
				}
				if len(f.clusterDatabaseIDs(clusterID)) > 0 {
					return "The database cluster still has databases and cannot be deleted."
				}
				return ""
			},
		},
		{
			typeName: "storage_bucket", idPrefix: "bucket",
			collectionPattern: "/buckets", itemPattern: "/buckets/{id}",
			patch: true, del: true,
			// Every bucket is created with an access key, which is listed under
			// it like any other key, and a bucket cannot be deleted while it has
			// keys. key_name and key_permission are request-only fields.
			afterCreate: func(f *fakeCloud, bucketID string) {
				attrs := f.objects["storage_bucket"][bucketID]
				f.seq++
				id := fmt.Sprintf("key-%d", f.seq)
				if f.objects["storage_bucket_key"] == nil {
					f.objects["storage_bucket_key"] = map[string]map[string]any{}
				}
				f.objects["storage_bucket_key"][id] = map[string]any{
					"name": attrs["key_name"], "permission": attrs["key_permission"],
					"access_key_id": "AKIATEST0000", "access_key_secret": "initial-key-secret",
				}
				key := "storage_bucket_key:" + bucketID
				f.children[key] = append(f.children[key], id)
				if f.parents["storage_bucket_key"] == nil {
					f.parents["storage_bucket_key"] = map[string]string{}
				}
				f.parents["storage_bucket_key"][id] = bucketID
				delete(attrs, "key_name")
				delete(attrs, "key_permission")
			},
			beforeDelete: func(f *fakeCloud, bucketID string) string {
				for _, id := range f.children["storage_bucket_key:"+bucketID] {
					if _, ok := f.objects["storage_bucket_key"][id]; ok {
						return "The bucket has access keys attached and cannot be deleted. Delete all its access keys first."
					}
				}
				return ""
			},
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["status"] = "available"
				if _, ok := a["location_hint"]; !ok {
					a["location_hint"] = "automatic"
				}
				return a
			},
			// The real API MERGES a cors_settings body into the bucket's
			// current rules rather than replacing it: an absent sub-field is
			// left alone, so only an explicit [] empties a list. It also keeps
			// the deprecated top-level allowed_origins as a live alias of
			// cors_settings.allowed_origins -- writing either updates both.
			patchFn: func(attrs, patch map[string]any) {
				for k, v := range patch {
					if k != "cors_settings" {
						attrs[k] = v
						continue
					}
					incoming, ok := v.(map[string]any)
					if !ok {
						continue // null and {} are ignored by the API
					}
					cors, _ := attrs["cors_settings"].(map[string]any)
					if cors == nil {
						cors = map[string]any{}
					} else {
						cors = cloneMap(cors)
					}
					for ck, cv := range incoming {
						cors[ck] = cv
					}
					attrs["cors_settings"] = cors
				}
				if origins, ok := patch["allowed_origins"]; ok {
					if cors, ok := attrs["cors_settings"].(map[string]any); ok {
						cors["allowed_origins"] = origins
					}
				}
				if cors, ok := attrs["cors_settings"].(map[string]any); ok {
					if origins, ok := cors["allowed_origins"]; ok {
						attrs["allowed_origins"] = origins
					}
				}
			},
		},
		{
			typeName: "websocket_server", idPrefix: "wss",
			collectionPattern: "/websocket-servers", itemPattern: "/websocket-servers/{id}",
			patch: true, del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["status"] = "creating"
				a["hostname"] = "ws.test.local"
				a["connection_distribution_strategy"] = "round_robin"
				return a
			},
			// A new server is "creating", which the API counts as busy, for
			// two requests; reading it uses those up, like time passing.
			afterCreate: func(f *fakeCloud, id string) { f.busy[id] = 2 },
			beforeGet: func(f *fakeCloud, id string) {
				f.refuseIfBusy(id)
				if f.busy[id] == 0 {
					f.objects["websocket_server"][id]["status"] = "available"
				}
			},
			beforeDelete: func(f *fakeCloud, id string) string {
				if f.refuseIfBusy(id) {
					return websocketDeleteBusyMessage
				}
				return ""
			},
		},
		// -------- environment-nested --------
		{
			typeName: "instance", idPrefix: "instance", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/instances", itemPattern: "/instances/{id}",
			patch: true, del: true,
			relationshipName: "environment", relationshipType: "environments",
			list:           true,
			validateCreate: validateInstanceCreate,
			afterCreate:    afterManagedQueueCreate,
			afterPatch:     dropQueueTimeoutsIfNotEntitled,
			// Like Cloud, an environment's app instance goes only with the
			// environment.
			beforeDelete: func(f *fakeCloud, id string) string {
				if f.objects["instance"][id]["type"] == "app" {
					return "The default App cluster cannot be deleted."
				}
				return ""
			},
			seed: func(body map[string]any, _ string) map[string]any {
				// Echo the body; managed-queue / status pointer fields are left
				// absent so they decode to null (not zero values) -- except
				// is_default, which a new managed queue reports as false.
				a := cloneMap(body)
				// The worker is a background process of its own, not an
				// attribute of the instance.
				delete(a, "background_processes")
				// The API stores a FIFO queue's name with a ".fifo" suffix and
				// reports the queue's type; fifo itself is not an attribute.
				delete(a, "fifo")
				if a["type"] == "managed_queue" {
					// A new queue's SQS queue is creating; see afterCreate.
					a["is_default"], a["paused"], a["queue_status"] = false, false, "creating"
					a["queue_type"] = "standard"
					if body["fifo"] == true {
						a["name"] = fmt.Sprint(a["name"]) + ".fifo"
						a["queue_type"] = "fifo"
					}
					// Current-generation queues have no polling interval; the
					// API ignores one and reports null.
					if size, _ := a["size"].(string); strings.HasPrefix(size, "mq.") {
						delete(a, "polling_interval")
					}
				}
				syncInstanceScaling(a)
				syncAppInstanceSettings(a)
				return a
			},
			patchFn: func(attrs, patch map[string]any) {
				for k, v := range patch {
					attrs[k] = v
				}
				syncInstanceScaling(attrs)
				syncAppInstanceSettings(attrs)
				// Only app instances take health checks; anything else has
				// them silently dropped. An empty object clears them.
				if probes, ok := attrs["probes"].(map[string]any); ok && (attrs["type"] != "app" || len(probes) == 0) {
					attrs["probes"] = nil
				}
			},
		},
		{
			typeName: "domain", idPrefix: "domain", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/domains", itemPattern: "/domains/{id}",
			patch: true, del: true,
			relationshipName: "environment", relationshipType: "environments",
			// A domain verifies on the third check; see POST /domains/{id}/verify.
			afterCreate: func(f *fakeCloud, id string) { f.busy[id] = 2 },
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				// The request sends www_redirect; the API echoes it as `redirect`.
				if v, ok := body["www_redirect"]; ok {
					a["redirect"] = v
				}
				a["type"] = "root"
				a["stage"] = "pre_verification"
				a["hostname_status"] = "pending"
				a["ssl_status"] = "pending"
				a["origin_status"] = "pending"
				a["action_required"] = "add_txt_records"
				// dns_records is required in the response and is how an
				// operator learns which records to create.
				a["dns_records"] = map[string]any{
					"ssl": []any{
						map[string]any{
							"type":  "TXT",
							"name":  "_acme-challenge.example.com",
							"value": "dcv-token",
						},
					},
					"pre_verification": map[string]any{
						"type":  "TXT",
						"name":  "_cf-custom-hostname.example.com",
						"value": "verify-token",
					},
					"origin": nil,
					"origin_cname": map[string]any{
						"type":  "CNAME",
						"name":  "example.com",
						"value": "origin.example.net",
					},
					"dcv": nil,
				}
				return a
			},
		},
		{
			typeName: "deployment", idPrefix: "deploy", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/deployments", itemPattern: "/deployments/{id}",
			relationshipName: "environment", relationshipType: "environments",
			// No PATCH, no DELETE (deployments are immutable and cannot be undone).
			seed:       seedDeployment,
			wrapCreate: queueDeploymentCreate,
			beforeGet:  finishStartDeployment,
		},
		{
			typeName: "command", idPrefix: "command", parentWildcard: "eid",
			collectionPattern: "/environments/{eid}/commands", itemPattern: "/commands/{id}",
			relationshipName: "environment", relationshipType: "environments",
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
			// legitimately 404s there. list stays on for the resource's
			// adopt-by-name logic.
			collectionPattern: "/databases/clusters/{cid}/databases",
			itemPattern:       "/databases/clusters/{cid}/databases/{id}",
			list:              true, del: true,
			// POST returns the *cluster* id, not the new schema id, while the
			// schema is addressable (and listed) under its real "db-schema-*" id.
			// The resource must resolve the canonical id from the listing rather
			// than trusting the create response.
			createRespID: func(_, parentID string) string { return parentID },
			// A schema is only deletable via the cluster-nested route; the flat
			// DELETE /databases/{id} does not accept it. The resource must use
			// the nested route.
			deletePattern: "/databases/clusters/{cid}/databases/{id}",
			// Creating or deleting a database is refused while its cluster is
			// busy, and deleting one starts an update of the cluster.
			beforeCreate: func(f *fakeCloud, clusterID string) string {
				if f.refuseIfBusy(clusterID) {
					return databaseClusterBusyMessage
				}
				return ""
			},
			beforeDelete: func(f *fakeCloud, id string) string {
				clusterID := f.parents["database"][id]
				if f.refuseIfBusy(clusterID) {
					return databaseClusterBusyMessage
				}
				f.busy[clusterID] = 1
				return ""
			},
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
			// web catch-all, which made keys unupdatable and undeletable, and
			// left the bucket undeletable behind them.
			collectionPattern: "/buckets/{bid}/keys", itemPattern: "/bucket-keys/{id}",
			list: true, patch: true, del: true,
			beforeDelete: func(f *fakeCloud, _ string) string {
				if f.refuseBucketKeyDeletes > 0 {
					f.refuseBucketKeyDeletes--
					return "This key is used by an environment and cannot be deleted."
				}
				return ""
			},
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
			relationshipName: "instance", relationshipType: "instances",
			// The API generates a worker's command from its config and ignores
			// any command sent, stores no config for a custom process (an empty
			// config comes back as []), and regenerates the command whenever a
			// worker's type or config is written. A PATCH replaces the whole
			// config, so keys sent as null are stored and reported as null.
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				if a["type"] == "custom" {
					a["config"] = []any{}
				} else {
					a["command"] = fakeWorkerCommand(a["config"])
				}
				return a
			},
			patchFn: func(attrs, patch map[string]any) {
				for k, v := range patch {
					attrs[k] = v
				}
				_, typeSent := patch["type"]
				_, configSent := patch["config"]
				if attrs["type"] == "worker" && (typeSent || configSent) {
					attrs["command"] = fakeWorkerCommand(attrs["config"])
				}
			},
		},
		{
			typeName: "scheduled_command", idPrefix: "sched", parentWildcard: "iid",
			collectionPattern: "/instances/{iid}/scheduled-commands", itemPattern: "/scheduled-commands/{id}",
			patch: true, del: true,
			relationshipName: "instance", relationshipType: "instances",
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				// name is nullable and reported as null when it was not sent.
				if _, ok := a["name"]; !ok {
					a["name"] = nil
				}
				a["created_at"] = "2026-01-01T00:00:00Z"
				return a
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
			// key and secret are merged into the create response only; every
			// later read omits them.
			createOnlyAttrs: []string{"key", "secret"},
			// Creating or deleting an application is refused while its server
			// is busy, and starts an update of the server.
			beforeCreate: func(f *fakeCloud, serverID string) string {
				if f.refuseIfBusy(serverID) {
					return websocketUpdateBusyMessage
				}
				f.markWebsocketServerBusy(serverID)
				return ""
			},
			beforeDelete: func(f *fakeCloud, id string) string {
				serverID := f.parents["websocket_application"][id]
				if f.refuseIfBusy(serverID) {
					return websocketDeleteBusyMessage
				}
				f.markWebsocketServerBusy(serverID)
				return ""
			},
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
					a["activity_timeout"] = 30
				}
				return a
			},
		},
		{
			typeName: "database_snapshot", idPrefix: "snap", parentWildcard: "cid",
			collectionPattern: "/databases/clusters/{cid}/snapshots", itemPattern: "/database-snapshots/{id}",
			relationshipName: "database", relationshipType: "databases",
			beforeCreate: func(f *fakeCloud, clusterID string) string {
				if f.refuseIfBusy(clusterID) {
					return clusterBusyMessage
				}
				return ""
			},
			beforeDelete: func(f *fakeCloud, id string) string {
				if f.refuseIfBusy(f.parents["database_snapshot"][id]) {
					return clusterBusyMessage
				}
				return ""
			},
			del: true,
			seed: func(body map[string]any, _ string) map[string]any {
				a := cloneMap(body)
				a["type"] = "manual"
				// Snapshots are created pending, with no size reported yet --
				// storage_bytes is null until the snapshot completes.
				a["status"] = "pending"
				a["pitr_enabled"] = false
				a["storage_bytes"] = nil
				return a
			},
		},
	}

	for _, s := range specs {
		f.registerCRUD(mux, s)
	}

	registerDataSourceRoutes(mux)

	// A managed queue's actions: making it the environment's default (which
	// clears the flag on the environment's others), pausing and resuming. Each
	// is refused while the queue is not available and when it would change
	// nothing, and answers with the instance.
	queueAction := func(action string, apply func(f *fakeCloud, id string, attrs map[string]any) string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			id := r.PathValue("id")
			attrs, ok := f.objects["instance"][id]
			if !ok {
				writeError(w, http.StatusNotFound, fmt.Errorf("instance not found"))
				return
			}
			if attrs["type"] != "managed_queue" {
				writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("Only managed queues can be %s.", action))
				return
			}
			if f.busy[id] > 0 {
				if f.busy[id]--; f.busy[id] == 0 {
					attrs["queue_status"] = "available"
				}
				writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("This queue is not available yet. Wait a few seconds and try again."))
				return
			}
			if f.refuseQueueActions > 0 {
				f.refuseQueueActions--
				writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("The queue could not be %s.", action))
				return
			}
			if msg := apply(f, id, attrs); msg != "" {
				writeError(w, http.StatusUnprocessableEntity, errors.New(msg))
				return
			}
			writeJSON(w, http.StatusOK, jsonAPIObject(id, "instances", attrs))
		}
	}
	mux.HandleFunc("POST /instances/{id}/default", queueAction("set as the default", func(f *fakeCloud, id string, attrs map[string]any) string {
		if attrs["is_default"] == true {
			return "This queue is already the default."
		}
		envID := f.parents["instance"][id]
		for other, otherAttrs := range f.objects["instance"] {
			if f.parents["instance"][other] == envID && otherAttrs["type"] == "managed_queue" {
				otherAttrs["is_default"] = false
			}
		}
		attrs["is_default"] = true
		return ""
	}))
	mux.HandleFunc("POST /instances/{id}/pause", queueAction("paused", func(_ *fakeCloud, _ string, attrs map[string]any) string {
		if attrs["paused"] == true {
			return "This queue is already paused."
		}
		attrs["paused"] = true
		return ""
	}))
	mux.HandleFunc("POST /instances/{id}/resume", queueAction("resumed", func(_ *fakeCloud, _ string, attrs map[string]any) string {
		if attrs["paused"] != true {
			return "This queue is not paused."
		}
		attrs["paused"] = false
		return ""
	}))

	// Checking a domain's DNS records. Every domain verifies on its third
	// check, except one named never.*, whose records are never created.
	var domainSpec crudSpec
	for _, s := range specs {
		if s.typeName == "domain" {
			domainSpec = s
		}
	}
	mux.HandleFunc("POST /domains/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id := r.PathValue("id")
		attrs, ok := f.objects["domain"][id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("domain not found"))
			return
		}
		if f.busy[id] > 0 {
			f.busy[id]--
		} else if name, _ := attrs["name"].(string); !strings.HasPrefix(name, "never.") {
			attrs["stage"], attrs["action_required"] = "origin", nil
			attrs["hostname_status"], attrs["ssl_status"], attrs["origin_status"] = "verified", "verified", "verified"
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": f.dataWithParent(domainSpec, id, attrs)})
	})

	// -------- bespoke endpoints --------

	// Environment variables. "set" writes the keys it is given and leaves the
	// rest alone -- the upsert semantics the separate delete route implies --
	// so a test that drops a key from config only sees it disappear if the
	// provider deletes it by name. The response is discarded by the provider
	// (no GET endpoint exists); f.vars is what assertions read. Like the real
	// API, an empty list is a 422: it takes 1 to 210 variables. The
	// environment as a whole holds at most 210, and a write that would go over
	// stores nothing.
	mux.HandleFunc("POST /environments/{id}/variables", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		body := decodeBody(r)
		invalid := func(msg string) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"message": msg,
				"errors":  map[string]any{"variables": []string{msg}},
			})
		}

		list, _ := body["variables"].([]any)
		switch {
		case len(list) == 0:
			invalid("The variables field is required.")
			return
		case len(list) > 210:
			invalid("A maximum of 210 variables may be provided per request.")
			return
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		merged := maps.Clone(f.vars[id])
		if merged == nil {
			merged = map[string]string{}
		}
		for _, item := range list {
			v, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key, _ := v["key"].(string)
			value, _ := v["value"].(string)
			merged[key] = value
		}
		if existing := len(f.vars[id]); len(merged) > 210 {
			invalid(fmt.Sprintf("An environment may not have more than 210 variables. "+
				"This environment has %d and you are trying to add %d.", existing, len(merged)-existing))
			return
		}
		f.vars[id] = merged

		writeJSON(w, http.StatusOK, jsonAPIObject(id, "environments", map[string]any{}))
	})
	mux.HandleFunc("POST /environments/{id}/variables/delete", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		body := decodeBody(r)

		// Like the real API, the delete is atomic: if any key is missing,
		// nothing is deleted and the answer is a 422 naming the missing keys.
		var keys, missing []string
		list, _ := body["keys"].([]any)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, k := range list {
			key, _ := k.(string)
			keys = append(keys, key)
			if _, ok := f.vars[id][key]; !ok {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			msg := "The following keys were not found: " + strings.Join(missing, ", ")
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"message": msg,
				"errors":  map[string]any{"keys": []string{msg}},
			})
			return
		}
		for _, key := range keys {
			delete(f.vars[id], key)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	// Database restore: creates a NEW cluster (returned as DatabaseClusterData)
	// that the normal cluster GET/DELETE handlers then serve. It must store the
	// object under clusterType so those routes find it.
	mux.HandleFunc("POST /databases/clusters/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(r)
		sourceID := r.PathValue("id")

		f.mu.Lock()
		defer f.mu.Unlock()

		if f.refuseIfBusy(sourceID) {
			writeError(w, http.StatusUnprocessableEntity, errors.New(clusterBusyMessage))
			return
		}

		// A Serverless Postgres restore keeps the source's ID, so the API
		// answers every later request for it with the source.
		if t, _ := f.objects[clusterType][sourceID]["type"].(string); strings.HasPrefix(t, "neon_serverless_postgres") {
			writeJSON(w, http.StatusCreated, jsonAPIObject(sourceID, clusterType, f.objects[clusterType][sourceID]))
			return
		}

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
		// The restored cluster holds copies of the source's databases, and is
		// busy while it restores.
		for _, dbID := range f.clusterDatabaseIDs(sourceID) {
			f.addClusterDatabase(id, f.objects["database"][dbID]["name"])
		}
		f.busy[id] = clusterCreatingRefusals
		writeJSON(w, http.StatusCreated, jsonAPIObject(id, clusterType, attrs))
	})
}

// The API's 422 messages for a busy cluster: one from the database routes,
// another from deleting or restoring the cluster itself.
const (
	databaseClusterBusyMessage = "This database cluster is already being updated. Try again shortly."
	clusterBusyMessage         = "An update is already in progress for this database. Try again in a few seconds."
)

// clusterCreatingRefusals is how many requests a new or restoring cluster
// refuses as busy before it becomes available.
const clusterCreatingRefusals = 2

// refuseIfBusy reports whether the database cluster or WebSocket server is
// busy, and uses up one refusal.
func (f *fakeCloud) refuseIfBusy(id string) bool {
	if f.busy[id] == 0 {
		return false
	}
	f.busy[id]--
	return true
}

// cacheProvisioningError is the API's 422 message when an environment is asked
// to attach a cache that is creating or updating, or "" when it can attach it.
// A refused request stands for the time the cache takes to finish, so the next
// one succeeds.
func (f *fakeCloud) cacheProvisioningError(id string) string {
	attrs, ok := f.objects["cache"][id]
	if !ok {
		return ""
	}
	if s := attrs["status"]; s == "creating" || s == "updating" {
		attrs["status"] = "available"
		delete(f.busy, id)
		return "The selected cache is still being provisioned. Try again in a few seconds."
	}
	return ""
}

// The API's 422 messages for a WebSocket server that is creating or updating:
// one from creating or changing an application or the server, another from
// deleting either.
const (
	websocketUpdateBusyMessage = "An update operation is already in progress for this WebSocket cluster."
	websocketDeleteBusyMessage = "An operation is already in progress for this WebSocket cluster."
)

// markWebsocketServerBusy starts an update of the server, as creating or
// deleting an application on it does. The server refuses the next request that
// changes it, and reports "updating" until then.
func (f *fakeCloud) markWebsocketServerBusy(serverID string) {
	f.busy[serverID] = 1
	f.objects["websocket_server"][serverID]["status"] = "updating"
}

// addClusterDatabase stores a database inside a cluster, as the platform does
// when it creates a cluster or restores one.
func (f *fakeCloud) addClusterDatabase(clusterID string, name any) {
	f.seq++
	id := fmt.Sprintf("db-schema-%d", f.seq)
	if f.objects["database"] == nil {
		f.objects["database"] = map[string]map[string]any{}
	}
	f.objects["database"][id] = map[string]any{"name": name, "status": "available"}
	key := "database:" + clusterID
	f.children[key] = append(f.children[key], id)
	if f.parents["database"] == nil {
		f.parents["database"] = map[string]string{}
	}
	f.parents["database"][id] = clusterID
}

// clusterDatabaseIDs returns the ids of the databases still in a cluster.
func (f *fakeCloud) clusterDatabaseIDs(clusterID string) []string {
	var ids []string
	for _, id := range f.children["database:"+clusterID] {
		if _, ok := f.objects["database"][id]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// seedCluster builds the stored attributes for a database cluster, including a
// connection object (with the cluster-only `driver` field) and a stable status.
// The `config` field is echoed from the request as a map so the provider's
// Read, which re-marshals it to a JSON string, round-trips cleanly.
// autoCreatedDatabaseName is the database the platform creates alongside every
// new cluster. It is what makes a cluster impossible to delete without first
// removing its databases -- and it is not managed by Terraform, which is why
// sweeping databases on destroy has to be opt-in.
const autoCreatedDatabaseName = "production"

func seedCluster(body map[string]any, _ string) map[string]any {
	a := cloneMap(body)
	a["status"] = "available"
	// The API answers with the base DatabaseType enum value, even when the
	// cluster was created with a retired identifier such as laravel_mysql_84.
	if t, ok := body["type"].(string); ok {
		a["type"] = baseDatabaseType(t)
	}
	// version is a create-only request field and is not echoed back.
	delete(a, "version")
	// The API returns the full effective config, including defaults the caller
	// never set. Echoing back only what was sent hid a config diff that could
	// never converge.
	if cfg, ok := a["config"].(map[string]any); ok {
		merged := map[string]any{"suspend_seconds": float64(0), "storage_autoscale_max_gb": nil}
		for k, v := range cfg {
			merged[k] = v
		}
		a["config"] = merged
	}
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

// baseDatabaseType maps a retired, version-bearing type identifier
// (laravel_mysql_84) onto the base type the API reports (laravel_mysql).
func baseDatabaseType(t string) string {
	for _, base := range []string{
		"neon_serverless_postgres",
		"aws_rds_postgres",
		"aws_rds_mysql",
		"laravel_mysql",
	} {
		if t == base || strings.HasPrefix(t, base+"_") {
			return base
		}
	}
	return t
}

// validateManagedQueueCreate applies the API's managed-queue create rules. A
// managed queue needs exactly one worker with one process on the cloud
// connection; when background_processes is left out the API fills in an entry
// with only the connection and queue, and rejects it with the error below.
// Previous-generation ("mq-") sizes require polling_interval and reject
// min_replicas.
// managedQueueCreatingRefusals is how many action requests a new managed
// queue refuses as not available yet. Live, its SQS queue is creating for a
// second or more after the create request returns.
const managedQueueCreatingRefusals = 2

// afterManagedQueueCreate models what the API does when it creates a managed
// queue: its SQS queue is creating for a while, and an environment's first
// queue becomes its default.
func afterManagedQueueCreate(f *fakeCloud, id string) {
	attrs := f.objects["instance"][id]
	if attrs["type"] != "managed_queue" {
		return
	}
	dropQueueTimeoutsIfNotEntitled(f, id)
	f.busy[id] = managedQueueCreatingRefusals
	envID := f.parents["instance"][id]
	for other, otherAttrs := range f.objects["instance"] {
		if other != id && f.parents["instance"][other] == envID && otherAttrs["is_default"] == true {
			return
		}
	}
	attrs["is_default"] = true
}

// dropQueueTimeoutsIfNotEntitled keeps the platform's visibility and shutdown
// timeouts on a managed queue, as the API does for an organization without the
// advanced managed queues entitlement; see queueTimeoutsNotEntitled.
func dropQueueTimeoutsIfNotEntitled(f *fakeCloud, id string) {
	attrs := f.objects["instance"][id]
	if !f.queueTimeoutsNotEntitled || attrs["type"] != "managed_queue" {
		return
	}
	attrs["visibility_timeout"], attrs["shutdown_timeout"] = 180, 90
}

func validateManagedQueueCreate(body map[string]any) string {
	if body["type"] != "managed_queue" {
		if body["fifo"] == true {
			return "FIFO is only available for managed queues."
		}
		return ""
	}
	if name, _ := body["name"].(string); body["fifo"] == true && len(name) > 34 {
		return "The queue name may have at most 34 characters and contain only alphanumeric characters, hyphens, and underscores."
	}
	procs, _ := body["background_processes"].([]any)
	if len(procs) != 1 {
		return "The type value is invalid. (and 1 more error)"
	}
	p, _ := procs[0].(map[string]any)
	cfg, _ := p["config"].(map[string]any)
	if p["type"] != "worker" || p["processes"] != float64(1) || cfg["connection"] != "cloud" {
		return "Managed queue background processes must be workers with exactly one process on the cloud connection."
	}
	if size, _ := body["size"].(string); strings.HasPrefix(size, "mq-") {
		if _, ok := body["polling_interval"]; !ok {
			return "The queue polling interval must be an integer between 1 and 60 seconds."
		}
		if _, ok := body["min_replicas"]; ok {
			return "The min_replicas field cannot be set for previous-generation managed queues, which always scale to zero when idle."
		}
	}
	return ""
}

// fakeWorkerCommand builds a worker command the way the API does, such as
// "php artisan queue:work --queue=default --tries=3 --quiet".
func fakeWorkerCommand(config any) string {
	cfg, _ := config.(map[string]any)
	parts := []string{"php artisan queue:work"}
	if c, ok := cfg["connection"].(string); ok {
		parts = append(parts, c)
	}
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v := cfg[k]; v != nil && k != "connection" {
			parts = append(parts, fmt.Sprintf("--%s=%v", k, v))
		}
	}
	return strings.Join(append(parts, "--quiet"), " ")
}

// fakeAutoMaxReplicas stands in for the plan's replica limit, which the API
// reports as max_replicas for an automatically scaled instance (1000 on the
// test organization's plan).
const fakeAutoMaxReplicas = 10

// validateInstanceCreate refuses what Cloud's create route does: it creates
// only service and managed_queue instances, as an environment's app instance
// comes with the environment.
func validateInstanceCreate(body map[string]any) string {
	if body["type"] != "service" && body["type"] != "managed_queue" {
		return "The type value is invalid."
	}
	return validateManagedQueueCreate(body)
}

// newAppInstance stores the app instance Cloud creates with every
// environment, as it reports it on the test organization. The caller holds
// f.mu.
func (f *fakeCloud) newAppInstance(envID string) {
	// Not from f.seq, which tests count on for other IDs.
	id := "instance-app-" + envID
	if f.objects["instance"] == nil {
		f.objects["instance"] = map[string]map[string]any{}
	}
	f.objects["instance"][id] = map[string]any{
		"name": "App", "type": "app", "size": "flex-512mb", "scaling_type": "none",
		"min_replicas": 1, "max_replicas": 1, "uses_scheduler": false,
		"uses_octane": false, "uses_inertia_ssr": false, "hibernation_timeout": 5,
	}
	f.children["instance:"+envID] = append(f.children["instance:"+envID], id)
	if f.parents["instance"] == nil {
		f.parents["instance"] = map[string]string{}
	}
	f.parents["instance"][id] = envID
}

// syncAppInstanceSettings does what the API does with the settings only app
// instances have: every other type ignores them and reports null. A null
// hibernation_timeout, which turns hibernation off, is kept as null.
func syncAppInstanceSettings(a map[string]any) {
	if a["type"] != "app" {
		a["uses_octane"], a["uses_inertia_ssr"], a["hibernation_timeout"] = nil, nil, nil
	}
}

// syncInstanceScaling does what the API does after every instance create and
// update: "auto" runs 1 replica up to the plan's limit and "none" exactly 1,
// whatever the instance had before, and "none" clears both thresholds.
func syncInstanceScaling(a map[string]any) {
	switch a["scaling_type"] {
	case "auto":
		a["min_replicas"], a["max_replicas"] = 1, fakeAutoMaxReplicas
	case "none":
		a["min_replicas"], a["max_replicas"] = 1, 1
		a["scaling_cpu_threshold_percentage"], a["scaling_memory_threshold_percentage"] = nil, nil
	}
}

// cacheHibernationError is the API's 422 for a create or update body that
// turns scale to zero on: it needs a timeout in the same request, a Valkey
// cache, and a size that scales to zero (the Flex sizes).
func cacheHibernationError(body map[string]any, cacheType, size string) string {
	if body["uses_hibernation"] != true {
		return ""
	}
	if body["hibernation_timeout"] == nil {
		return "The hibernation timeout field is required when uses hibernation is true."
	}
	if cacheType != "laravel_valkey" {
		return "Scale to zero is not available for this cache type."
	}
	if !strings.HasPrefix(size, "valkey-flex-") {
		return "Scale to zero is not available for this cache size."
	}
	return ""
}
