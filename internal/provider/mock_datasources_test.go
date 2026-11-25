package provider

import (
	"net/http"
	"strings"
)

// registerDataSourceRoutes wires the read-only routes the data sources use.
//
// Every payload here is modelled on a real response captured from a live
// Laravel Cloud API, including the shapes that are easy to get wrong:
//   - /instances/sizes is an object keyed by instance class, not a list, and
//     managed-queue sizes report a FRACTIONAL cpu_count.
//   - /meta/regions and the */types routes are flat {"data": [...]} documents
//     with no paginator, unlike the JSON:API collections.
//   - /databases/types carries a versions list, empty for retired identifiers.
//   - /ip is a bare region-keyed object and is not JSON:API at all.
func registerDataSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /meta/organization", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"id":         "org-0001",
				"type":       "organizations",
				"attributes": map[string]any{"name": "Acme", "slug": "acme"},
			},
		})
	})

	// Note the property is "region", not "name".
	mux.HandleFunc("GET /meta/regions", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{
			map[string]any{"region": "us-east-2", "label": "Ohio", "flag": "🇺🇸"},
			map[string]any{"region": "eu-west-1", "label": "Ireland", "flag": "🇮🇪"},
		}})
	})

	mux.HandleFunc("GET /instances/sizes", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"general": []any{
				map[string]any{
					"name": "flex.c-1vcpu-256mb", "label": "1 vCPU / 256 MB",
					"description": "Flexible", "cpu_type": "shared",
					"compute_class": "general", "cpu_count": 1, "memory_mib": 256,
				},
			},
			"managed_queue": []any{
				map[string]any{
					"name": "mq-pro-256mb", "label": "Queue 256 MB",
					"description": "Managed queue", "cpu_type": "shared",
					"compute_class": "general", "cpu_count": 0.0625, "memory_mib": 256,
				},
			},
		}})
	})

	mux.HandleFunc("GET /caches/types", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{
			map[string]any{
				"name": "laravel_valkey", "type": "laravel_valkey", "label": "Laravel Valkey",
				"regions": []any{"us-east-2"},
				"sizes": []any{
					map[string]any{"value": "valkey-flex-250mb", "label": "250 MB", "supports_hibernation": true},
					map[string]any{"value": "valkey-pro.250mb", "label": "250 MB", "supports_hibernation": false},
				},
				"supports_auto_upgrade":    true,
				"supports_eviction_policy": true,
				"supports_hibernation":     true,
			},
			map[string]any{
				"type": "upstash_redis", "label": "Upstash Redis",
				"regions":                  []any{"us-east-2"},
				"sizes":                    []any{map[string]any{"value": "250mb", "label": "250 MB", "supports_hibernation": false}},
				"supports_auto_upgrade":    false,
				"supports_eviction_policy": false,
				"supports_hibernation":     false,
			},
		}})
	})

	mux.HandleFunc("GET /databases/types", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{
			map[string]any{
				"type": "laravel_mysql", "label": "Laravel MySQL",
				"versions": []any{"8.4"},
				"regions":  []any{"us-east-2"},
				"config_schema": []any{
					map[string]any{"name": "size", "type": "string", "enum": []any{"mysql-flex-512mb", "mysql-flex-1gb"}},
				},
			},
			// A retired identifier: still listed, but with no versions.
			map[string]any{
				"type": "laravel_mysql_84", "label": "Laravel MySQL 8.4",
				"versions":      []any{},
				"regions":       []any{"us-east-2"},
				"config_schema": []any{},
			},
		}})
	})

	// Both list routes take filter[...] query parameters, matched as
	// substrings, and refuse any filter they do not allow with a 400.
	mux.HandleFunc("GET /dedicated-clusters", func(w http.ResponseWriter, r *http.Request) {
		listFiltered(w, r, []string{"region", "type", "status"}, []map[string]any{
			{
				"id": "cluster-0001", "type": "clusters",
				"attributes": map[string]any{
					"name": "prd-use2-ent-acme", "region": "us-east-2", "type": "applications",
					"tenancy_type": "dedicated", "status": "active",
					"created_at": "2026-02-02T22:37:57.000000Z",
				},
			},
			{
				"id": "cluster-0002", "type": "clusters",
				"attributes": map[string]any{
					"name": "prd-use2-ent-acme", "region": "us-east-2", "type": "rds-databases",
					"tenancy_type": "dedicated", "status": "active",
					"created_at": "2026-02-09T20:45:55.000000Z",
				},
			},
		})
	})

	// The list includes the shared edge network every organization can use.
	mux.HandleFunc("GET /edge-networks", func(w http.ResponseWriter, r *http.Request) {
		listFiltered(w, r, []string{"name", "domain", "status"}, []map[string]any{
			{
				"id": "edgenet-0001", "type": "edge_networks",
				"attributes": map[string]any{
					"name": "Laravel Cloud shared zone", "domain": "laravel.cloud",
					"tenancy_type": "shared", "status": "available",
					"created_at": "2025-05-20T12:41:11.000000Z",
				},
			},
			{
				"id": "edgenet-0002", "type": "edge_networks",
				"attributes": map[string]any{
					"name": "acme-demo.cloud", "domain": "acme-demo.cloud",
					"tenancy_type": "dedicated", "status": "available",
					"created_at": "2026-02-04T15:33:29.000000Z",
				},
			},
		})
	})

	mux.HandleFunc("GET /websocket-servers/types", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{
			map[string]any{
				"type": "reverb", "label": "Laravel Reverb", "available": true,
				"regions": []any{"us-east-1", "us-east-2"},
				"max_connections": []any{
					map[string]any{"value": 100, "label": "100", "available": true},
					map[string]any{"value": 2000, "label": "2,000", "available": true},
					map[string]any{"value": 10000, "label": "10,000", "available": false},
				},
			},
		}})
	})

	// /databases/presets takes the unversioned type identifiers only, and
	// requires both type and region.
	mux.HandleFunc("GET /databases/presets", func(w http.ResponseWriter, r *http.Request) {
		presets := map[string][]any{
			"laravel_mysql": {
				map[string]any{"key": "dev", "label": "Dev", "config": map[string]any{
					"suspend_seconds": 300, "retention_days": 1, "size": "mysql-flex-512mb", "storage": 5,
					"uses_scheduled_snapshots": true, "is_public": false,
				}},
				map[string]any{"key": "prod", "label": "Prod", "config": map[string]any{
					"retention_days": 7, "size": "mysql-flex-2gb", "storage": 20,
					"uses_scheduled_snapshots": true, "is_public": false,
				}},
			},
			"neon_serverless_postgres": {
				map[string]any{"key": "dev", "label": "Dev", "config": map[string]any{
					"cu_min": 0.25, "cu_max": 0.25, "suspend_seconds": 300, "retention_days": 1,
				}},
			},
		}
		q := r.URL.Query()
		list, ok := presets[q.Get("type")]
		if !ok || q.Get("region") == "" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"message": "The selected type is invalid.",
				"errors":  map[string]any{"type": []any{"The selected type is invalid."}},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": list})
	})

	mux.HandleFunc("GET /resend/domains", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{map[string]any{
				"id": "rdom-0001", "type": "resend_domains",
				"attributes": map[string]any{
					"name": "mail.example.com", "region": "us-east-1", "status": "verified",
					"records": []any{
						map[string]any{
							"record": "DKIM", "name": "resend._domainkey", "type": "TXT",
							"ttl": "Auto", "status": "verified", "value": "p=MIGf",
						},
						map[string]any{
							"record": "SPF", "name": "send", "type": "MX", "ttl": "Auto",
							"status": "verified", "value": "feedback-smtp.us-east-1.amazonses.com", "priority": 10,
						},
					},
					"last_verified_at": "2026-03-01T10:00:00.000000Z",
					"created_at":       "2026-03-01T09:00:00.000000Z",
				},
			}},
			"meta": map[string]any{"current_page": 1, "last_page": 1, "per_page": 100, "total": 1},
		})
	})

	// /ip is unauthenticated, is absent from the OpenAPI spec, and answers with
	// a bare region-keyed object -- or, when region is passed, just that
	// region's addresses.
	mux.HandleFunc("GET /ip", func(w http.ResponseWriter, r *http.Request) {
		if region := r.URL.Query().Get("region"); region != "" {
			writeJSON(w, http.StatusOK, map[string]any{
				"ipv4": []any{"52.70.166.119"}, "ipv6": []any{"2600:1f18:124c:a900::/56"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"us-east-2": map[string]any{"ipv4": []any{"3.135.136.57"}, "ipv6": []any{}},
			"us-east-1": map[string]any{"ipv4": []any{"52.70.166.119"}, "ipv6": []any{"2600:1f18:124c:a900::/56"}},
		})
	})
}

// listFiltered answers a paginated list route, keeping the items whose
// attributes contain every filter[...] value given, as the API's partial
// filters do.
func listFiltered(w http.ResponseWriter, r *http.Request, allowed []string, items []map[string]any) {
	filters := map[string]string{}
	for key, values := range r.URL.Query() {
		name, ok := strings.CutPrefix(key, "filter[")
		if !ok {
			continue
		}
		name = strings.TrimSuffix(name, "]")
		permitted := false
		for _, a := range allowed {
			permitted = permitted || a == name
		}
		if !permitted {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"message": "Requested filter(s) `" + name + "` are not allowed.",
			})
			return
		}
		filters[name] = values[0]
	}

	out := []any{}
	for _, item := range items {
		attrs := item["attributes"].(map[string]any)
		match := true
		for name, value := range filters {
			v, _ := attrs[name].(string)
			match = match && strings.Contains(v, value)
		}
		if match {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": out,
		"meta": map[string]any{"current_page": 1, "last_page": 1, "per_page": 100, "total": len(out)},
	})
}
