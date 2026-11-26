package provider

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/laravel/terraform-provider-laravel/internal/client"
)

// registerInternalNetworkingRoutes serves /environments/{id}/internal-networking
// the way the live API answered it: POST turns it on (and also changes the
// access of an environment that has it), PATCH is refused unless it is on,
// DELETE turns it off, and each write answers with the environment. A write
// to a missing environment is answered with a 302 to the dashboard. The fake
// does not model dedicated clusters or the entitlement, both of which the
// live API checks.
func (f *fakeCloud) registerInternalNetworkingRoutes(mux *http.ServeMux) {
	const route = "/environments/{id}/internal-networking"
	f.internalNetworkingWrites = map[string]int{}

	write := func(enable bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var req client.InternalNetworkingRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			id := r.PathValue("id")
			env, ok := f.envs[id]
			if !ok {
				w.Header().Set("Location", "https://cloud.laravel.com")
				w.WriteHeader(http.StatusFound)
				return
			}
			f.internalNetworkingWrites[r.Method]++
			if !enable && (env.Attributes.InternalNetworking == nil || !env.Attributes.InternalNetworking.Enabled) {
				writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("Internal networking is not enabled for this environment."))
				return
			}
			switch req.AccessLevel {
			case "all":
				req.EnvironmentIDs = []string{}
			case "specific":
				if len(req.EnvironmentIDs) == 0 {
					writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("Select at least one environment that can send internal traffic."))
					return
				}
				for _, other := range req.EnvironmentIDs {
					if _, ok := f.envs[other]; !ok || other == id {
						writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("One or more of the selected environments are not available for internal networking."))
						return
					}
				}
			default:
				writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("The access level field is required."))
				return
			}
			level := req.AccessLevel
			env.Attributes.InternalNetworking = &client.EnvironmentInternalNetworking{
				Enabled:        true,
				Hostname:       ptr(id + ".private.laravel.cloud"),
				AccessLevel:    &level,
				EnvironmentIDs: req.EnvironmentIDs,
			}
			writeJSON(w, http.StatusOK, f.environmentDoc(env))
		}
	}
	mux.HandleFunc("POST "+route, write(true))
	mux.HandleFunc("PATCH "+route, write(false))

	mux.HandleFunc("DELETE "+route, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		env, ok := f.envs[r.PathValue("id")]
		if !ok {
			w.Header().Set("Location", "https://cloud.laravel.com")
			w.WriteHeader(http.StatusFound)
			return
		}
		f.internalNetworkingWrites[r.Method]++
		env.Attributes.InternalNetworking = &client.EnvironmentInternalNetworking{EnvironmentIDs: []string{}}
		w.WriteHeader(http.StatusNoContent)
	})
}
