package provider

import (
	"net/http"
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
				"regions":               []any{"us-east-2"},
				"sizes":                 []any{map[string]any{"value": "valkey-pro.250mb", "label": "250 MB"}},
				"supports_auto_upgrade": true,
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

	mux.HandleFunc("GET /dedicated-clusters", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{},
			"meta": map[string]any{"current_page": 1, "last_page": 1, "per_page": 100, "total": 0},
		})
	})

	mux.HandleFunc("GET /edge-networks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": []any{map[string]any{
				"id": "edgenet-0001", "type": "edge_networks",
				"attributes": map[string]any{
					"name": "Laravel Cloud shared zone", "domain": "laravel.cloud",
					"tenancy_type": "shared", "status": "available",
					"created_at": "2025-05-20T12:41:11.000000Z",
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
