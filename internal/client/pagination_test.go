package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every collection route in the public API is paginated at 100 items per page,
// and per_page is not honoured. Reading only the first page made a Read that
// resolves a resource from a listing report "not found" for a resource that
// exists, which dropped it from state and proposed recreating live
// infrastructure. listAll must walk to the last page.
func TestListAllFollowsEveryPage(t *testing.T) {
	const perPage, total = 2, 5
	var gotPages []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		gotPages = append(gotPages, page)
		if page == "" {
			page = "1"
		}
		var n int
		if _, err := fmt.Sscanf(page, "%d", &n); err != nil {
			t.Errorf("unparseable page %q", page)
		}

		lastPage := (total + perPage - 1) / perPage
		var items []string
		for i := (n - 1) * perPage; i < n*perPage && i < total; i++ {
			items = append(items, fmt.Sprintf(`{"id":"app-%d","type":"applications"}`, i))
		}

		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprintf(w, `{"data":[%s],"meta":{"current_page":%d,"last_page":%d,"per_page":%d,"total":%d}}`,
			strings.Join(items, ","), n, lastPage, perPage, total)
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	got, err := listAll[ApplicationData](context.Background(), c, "/applications")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != total {
		t.Fatalf("got %d items, want %d — pagination is being truncated", len(got), total)
	}
	for i, item := range got {
		if want := fmt.Sprintf("app-%d", i); item.ID != want {
			t.Errorf("item %d: got id %q, want %q", i, item.ID, want)
		}
	}
	// Page 1 is requested without an explicit page param, then 2 and 3.
	if want := 3; len(gotPages) != want {
		t.Errorf("made %d requests (%v), want %d", len(gotPages), gotPages, want)
	}
	if gotPages[0] != "" {
		t.Errorf("first request should carry no page param, got %q", gotPages[0])
	}
}

// A response with no paginator meta is a complete result. Non-paginated routes
// and hand-written fakes rely on this, so a missing last_page must not be read
// as "there are more pages".
func TestListAllStopsWithoutPaginatorMeta(t *testing.T) {
	var requests int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":[{"id":"app-1","type":"applications"}]}`))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	got, err := listAll[ApplicationData](context.Background(), c, "/applications")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d items, want 1", len(got))
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1", requests)
	}
}

// A server that reports a last_page it never delivers must not spin forever.
func TestListAllStopsOnEmptyPage(t *testing.T) {
	var requests int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/vnd.api+json")
		// Claims 500 pages but returns nothing after the first.
		if requests == 1 {
			_, _ = w.Write([]byte(`{"data":[{"id":"app-1","type":"applications"}],"meta":{"current_page":1,"last_page":500}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[],"meta":{"current_page":2,"last_page":500}}`))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	got, err := listAll[ApplicationData](context.Background(), c, "/applications")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d items, want 1", len(got))
	}
	if requests != 2 {
		t.Errorf("made %d requests, want 2 (it must give up on an empty page)", requests)
	}
}

// A path that already carries a query string must gain "&page=", not a second
// "?", and url.JoinPath must not escape the "?" into %3F.
func TestListAllAppendsPageToAnExistingQueryString(t *testing.T) {
	var gotURLs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURLs = append(gotURLs, r.URL.String())
		page := r.URL.Query().Get("page")
		last := 2
		n := 1
		if page == "2" {
			n = 2
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprintf(w, `{"data":[{"id":"app-%d","type":"applications"}],"meta":{"current_page":%d,"last_page":%d}}`, n, n, last)
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	if _, err := listAll[ApplicationData](context.Background(), c, "/applications?include=environments"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotURLs) != 2 {
		t.Fatalf("made %d requests (%v), want 2", len(gotURLs), gotURLs)
	}
	if want := "/api/applications?include=environments"; gotURLs[0] != want {
		t.Errorf("first request URL = %q, want %q", gotURLs[0], want)
	}
	if want := "/api/applications?include=environments&page=2"; gotURLs[1] != want {
		t.Errorf("second request URL = %q, want %q", gotURLs[1], want)
	}
}

// The bucket-key and per-database item routes are flat/nested in ways the
// provider previously got wrong; pin the paths so a regression is loud.
func TestItemRoutesUseTheDocumentedPaths(t *testing.T) {
	var gotPaths []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data":{"id":"x","type":"t","attributes":{}}}`))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))
	ctx := context.Background()

	if _, err := c.GetStorageBucketKey(ctx, "flsk-1"); err != nil {
		t.Fatalf("GetStorageBucketKey: %v", err)
	}
	if _, err := c.UpdateStorageBucketKey(ctx, "flsk-1", UpdateStorageBucketKeyRequest{Name: "n"}); err != nil {
		t.Fatalf("UpdateStorageBucketKey: %v", err)
	}
	if err := c.DeleteStorageBucketKey(ctx, "flsk-1"); err != nil {
		t.Fatalf("DeleteStorageBucketKey: %v", err)
	}
	if _, err := c.GetDatabase(ctx, "db-1", "db-schema-1"); err != nil {
		t.Fatalf("GetDatabase: %v", err)
	}

	want := []string{
		// Key item routes are top-level, not nested under the bucket.
		"GET /api/bucket-keys/flsk-1",
		"PATCH /api/bucket-keys/flsk-1",
		"DELETE /api/bucket-keys/flsk-1",
		// A per-database route IS nested; flat /databases/{id} is the
		// deprecated cluster route.
		"GET /api/databases/clusters/db-1/databases/db-schema-1",
	}
	for i, w := range want {
		if i >= len(gotPaths) {
			t.Fatalf("missing request %d, want %q", i, w)
		}
		if gotPaths[i] != w {
			t.Errorf("request %d = %q, want %q", i, gotPaths[i], w)
		}
	}
}

// listAll being correct is worth nothing if the List* methods don't route
// through it. This walks each converted method against a two-page server and
// fails if any returns only the first page. A method added later that hand-rolls
// its own single c.do will show up here.
func TestEveryListMethodPaginates(t *testing.T) {
	// Two pages of one item each, for whatever collection is asked for.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if r.URL.Query().Get("page") == "2" {
			page = 2
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = fmt.Fprintf(w, `{"data":[{"id":"id-%d","type":"t","attributes":{}}],`+
			`"meta":{"current_page":%d,"last_page":2,"per_page":1,"total":2}}`, page, page)
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))
	ctx := context.Background()

	// Every list method the provider exposes, against a paginated collection.
	cases := map[string]func() (int, error){
		"ListApplications":          func() (int, error) { v, e := c.ListApplications(ctx); return len(v), e },
		"ListEnvironments":          func() (int, error) { v, e := c.ListEnvironments(ctx, "app-1"); return len(v), e },
		"ListInstances":             func() (int, error) { v, e := c.ListInstances(ctx, "env-1"); return len(v), e },
		"ListBackgroundProcesses":   func() (int, error) { v, e := c.ListBackgroundProcesses(ctx, "inst-1"); return len(v), e },
		"ListCaches":                func() (int, error) { v, e := c.ListCaches(ctx); return len(v), e },
		"ListCommands":              func() (int, error) { v, e := c.ListCommands(ctx, "env-1"); return len(v), e },
		"ListDeployments":           func() (int, error) { v, e := c.ListDeployments(ctx, "env-1"); return len(v), e },
		"ListDomains":               func() (int, error) { v, e := c.ListDomains(ctx, "env-1"); return len(v), e },
		"ListDatabaseClusters":      func() (int, error) { v, e := c.ListDatabaseClusters(ctx); return len(v), e },
		"ListDatabases":             func() (int, error) { v, e := c.ListDatabases(ctx, "db-1"); return len(v), e },
		"ListDatabaseSnapshots":     func() (int, error) { v, e := c.ListDatabaseSnapshots(ctx, "db-1"); return len(v), e },
		"ListDedicatedClusters":     func() (int, error) { v, e := c.ListDedicatedClusters(ctx); return len(v), e },
		"ListStorageBuckets":        func() (int, error) { v, e := c.ListStorageBuckets(ctx); return len(v), e },
		"ListStorageBucketKeys":     func() (int, error) { v, e := c.ListStorageBucketKeys(ctx, "fls-1"); return len(v), e },
		"ListWebsocketServers":      func() (int, error) { v, e := c.ListWebsocketServers(ctx); return len(v), e },
		"ListWebsocketApplications": func() (int, error) { v, e := c.ListWebsocketApplications(ctx, "ws-1"); return len(v), e },
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			n, err := call()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != 2 {
				t.Errorf("returned %d items, want 2 — %s is not following the paginator", n, name)
			}
		})
	}
}
