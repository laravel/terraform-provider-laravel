package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Laravel Cloud API answers some write routes with a 302 back to the app
// root instead of a JSON document (e.g. POST /instances/{id}/background-processes
// and POST /environments/{id}/domains on dev). Go's http.Client follows
// redirects by default and downgrades POST/DELETE to GET, so the provider used
// to land on the dashboard and report a baffling "expected JSON ... got
// text/html" with a dump of the SPA shell. The redirect itself is the
// diagnosis, so surface it verbatim.
func TestDoReportsRedirectInsteadOfFollowingIt(t *testing.T) {
	var followed bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/instances/inst-1/background-processes" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		// The dashboard SPA shell the redirect would land on.
		followed = true
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><head></head></html>"))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	err := c.do(context.Background(), "POST", "/instances/inst-1/background-processes", map[string]any{"type": "worker"}, nil)
	if err == nil {
		t.Fatal("expected an error for a redirected write, got nil")
	}
	if followed {
		t.Error("client followed the redirect; it must not turn a POST into a GET on the app root")
	}
	for _, want := range []string{"302", "redirect", "/"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
	if strings.Contains(err.Error(), "<!DOCTYPE") {
		t.Errorf("error should not dump the SPA shell, got %q", err.Error())
	}
	// Laravel Cloud answers a throttled request with a 429, so blaming a
	// redirect on a rate limit sends the user after the wrong cause.
	if strings.Contains(strings.ToLower(err.Error()), "rate limit") {
		t.Errorf("error %q blames a rate limit; Laravel Cloud answers those with a 429", err.Error())
	}
}

// A 302 to the app root on a request that names a repository is most likely
// the API failing to reach the source control provider, so the error points
// at that connection. Any other redirect, or one for a request without a
// repository, must not send the user after a source control problem.
func TestDoRedirectNamesSourceControlOnlyForRepositoryAtAppRoot(t *testing.T) {
	createApp := map[string]any{"name": "app", "region": "us-east-2", "repository": "acme/app"}
	for _, tc := range []struct {
		name     string
		path     string
		body     any
		location string
		wantHint bool
	}{
		{"repository, app root", "/applications", createApp, "https://cloud.laravel.com", true},
		{"repository, relative app root", "/applications", createApp, "/", true},
		{"repository, sign-in page", "/applications", createApp, "https://cloud.laravel.com/login", false},
		{"repository, another API URL", "/applications", createApp, "https://cloud.laravel.com/api/applications", false},
		{"no repository, app root", "/instances/inst-1/background-processes", map[string]any{"type": "worker"}, "/", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", tc.location)
				w.WriteHeader(http.StatusFound)
			}))
			defer srv.Close()

			c := NewClient("token", WithBaseURL(srv.URL+"/api"))
			err := c.do(context.Background(), "POST", tc.path, tc.body, nil)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.location) {
				t.Errorf("error %q does not include the Location %q", msg, tc.location)
			}
			if got := strings.Contains(msg, "source control provider"); got != tc.wantHint {
				t.Errorf("source control hint = %v, want %v, in %q", got, tc.wantHint, msg)
			}
			if strings.Contains(strings.ToLower(msg), "rate limit") {
				t.Errorf("error %q blames a rate limit", msg)
			}
		})
	}
}

// A 302 is only meaningful for the caller if the Location survives into the
// message, since that is what identifies the misrouted request.
func TestDoRedirectErrorIncludesLocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://app.example.test/login", http.StatusFound)
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	err := c.do(context.Background(), "DELETE", "/buckets/b-1/keys/k-1", nil, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "https://app.example.test/login") {
		t.Errorf("error %q does not include the Location header", err.Error())
	}
}

// Ordinary 2xx JSON responses must keep working unchanged.
func TestDoStillDecodesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"bp-1","type":"background-processes"}}`))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	var doc Document[BackgroundProcessData]
	if err := c.do(context.Background(), "GET", "/background-processes/bp-1", nil, &doc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Data.ID != "bp-1" {
		t.Errorf("got id %q, want %q", doc.Data.ID, "bp-1")
	}
}
