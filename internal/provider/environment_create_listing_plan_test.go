package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestEnvironmentCreateListingFailurePlan pins the environment create path to
// failing loudly when it cannot see the environments that already exist.
//
// Laravel Cloud creates a default environment alongside an application, so the
// provider lists the existing ones and adopts a name match instead of creating
// a duplicate. That listing's error was discarded: `if err == nil { ... }` fell
// straight through to the create call, so a transient 500 produced exactly the
// duplicate the listing was there to prevent -- a second live, billing
// environment, invisible to the state file that had just been written.
//
// Erroring instead leaves the apply retryable and creates nothing.
func TestEnvironmentCreateListingFailurePlan(t *testing.T) {
	baseURL := newListingFailureServer(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      environmentListingFailureConfig(baseURL),
				ExpectError: regexp.MustCompile(`Error listing existing environments`),
			},
		},
	})
}

// newListingFailureServer serves an application create and then fails every
// environment listing, which is the only behaviour this test needs. A create
// handler is deliberately absent: reaching it at all is the bug.
func newListingFailureServer(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /applications", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusCreated, jsonAPIObject("app-1", "applications", map[string]any{
			"name":       "listing-app",
			"repository": map[string]any{"full_name": "laravel/laravel"},
			"region":     "us-east-2",
			"slug":       "listing-app",
		}))
	})
	mux.HandleFunc("GET /applications/{id}/environments", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("upstream unavailable"))
	})
	mux.HandleFunc("POST /applications/{id}/environments", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("environment create was reached even though the listing failed")
		writeError(w, http.StatusInternalServerError, fmt.Errorf("should not be called"))
	})
	// Enough for the framework's post-test destroy of the application that did
	// get created before the failing step.
	mux.HandleFunc("GET /applications/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, jsonAPIObject(r.PathValue("id"), "applications", map[string]any{
			"name":       "listing-app",
			"repository": map[string]any{"full_name": "laravel/laravel"},
			"region":     "us-east-2",
			"slug":       "listing-app",
		}))
	})
	mux.HandleFunc("DELETE /applications/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func environmentListingFailureConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "laravel" {
  token    = "test-token"
  base_url = %[1]q
}

resource "laravel_cloud_application" "app" {
  name       = "listing-app"
  repository = "laravel/laravel"
  region     = "us-east-2"
}

resource "laravel_cloud_environment" "env" {
  application_id = laravel_cloud_application.app.id
  name           = "production"
  branch         = "main"
}
`, baseURL)
}
