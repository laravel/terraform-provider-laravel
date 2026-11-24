package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryDelay pins where the wait before a retried read comes from:
// Retry-After first, then X-RateLimit-Reset, then exponential backoff, and
// never more than maxRetryWait.
func TestRetryDelay(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	h := func(kv ...string) http.Header {
		out := http.Header{}
		for i := 0; i < len(kv); i += 2 {
			out.Set(kv[i], kv[i+1])
		}
		return out
	}
	cases := []struct {
		name    string
		status  int
		header  http.Header
		attempt int
		want    time.Duration
	}{
		{"retry-after seconds", 429, h("Retry-After", "7", "X-RateLimit-Reset", strconv.FormatInt(now.Unix()+30, 10)), 1, 7 * time.Second},
		{"retry-after date", 429, h("Retry-After", now.Add(12*time.Second).UTC().Format(http.TimeFormat)), 1, 12 * time.Second},
		{"reset when no retry-after", 429, h("X-RateLimit-Reset", strconv.FormatInt(now.Unix()+20, 10)), 1, 20 * time.Second},
		{"reset in the past", 429, h("X-RateLimit-Reset", strconv.FormatInt(now.Unix()-5, 10)), 1, 0},
		{"429 without headers backs off", 429, h(), 3, 4 * retryBaseDelay},
		{"capped", 429, h("Retry-After", "3600"), 1, maxRetryWait},
		// A 503's Retry-After is not a rate-limit window; back off instead.
		{"503 backs off", 503, h("Retry-After", "40"), 2, 2 * retryBaseDelay},
		{"backoff capped", 502, h(), 10, maxRetryWait},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := retryDelay(tc.status, tc.header, tc.attempt, now); got != tc.want {
				t.Errorf("retryDelay = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRateClass(t *testing.T) {
	cases := []struct {
		method, path    string
		create, command bool
	}{
		{"POST", "/applications", true, false},
		{"GET", "/applications", false, false},
		{"PATCH", "/applications/app-1", false, false},
		{"POST", "/applications/app-1/environments", true, false},
		{"PATCH", "/instances/inst-1", true, false},
		{"DELETE", "/instances/inst-1", false, false},
		{"POST", "/environments/env-1/commands", false, true},
		{"POST", "/environments/env-1/deployments", false, false},
		{"GET", "/environments/env-1/logs?from=1", true, false},
		{"POST", "/databases/clusters", true, false},
		{"POST", "/databases/clusters/db-1/databases", false, false},
		{"POST", "/databases/clusters/db-1/snapshots", true, false},
		{"POST", "/databases/clusters/db-1/users?include=databases", true, false},
		{"PATCH", "/databases/clusters/db-1/users/dbu-1", false, false},
		{"POST", "/buckets/b-1/keys", false, false},
	}
	for _, tc := range cases {
		create, command := rateClass(tc.method, tc.path)
		if create != tc.create || command != tc.command {
			t.Errorf("rateClass(%s %s) = create %v, command %v; want %v, %v", tc.method, tc.path, create, command, tc.create, tc.command)
		}
	}
}

// TestLimiterSpacing: after the burst, requests are spaced one interval
// apart, and a pause holds every later request back until it ends.
func TestLimiterSpacing(t *testing.T) {
	l := newLimiter(60) // one a second, burst 6
	now := time.Unix(1_800_000_000, 0)
	for i := 0; i < 6; i++ {
		if at := l.reserve(now); !at.Equal(now) {
			t.Fatalf("request %d of the burst delayed to +%v", i+1, at.Sub(now))
		}
	}
	for i := 1; i <= 3; i++ {
		if at := l.reserve(now); at.Sub(now) != time.Duration(i)*time.Second {
			t.Fatalf("request %d after the burst at +%v, want +%ds", i, at.Sub(now), i)
		}
	}

	// After a long quiet spell the burst is available again, but not while
	// paused.
	later := now.Add(time.Hour)
	l.pause(later.Add(30 * time.Second))
	if at := l.reserve(later); at.Sub(later) != 30*time.Second {
		t.Fatalf("request during a pause at +%v, want +30s", at.Sub(later))
	}
}

// TestLimiterSharedPerOrganization: clients with the same token and URL share
// one budget; another token gets its own.
func TestLimiterSharedPerOrganization(t *testing.T) {
	a := NewClient("tok-shared-a", WithBaseURL("http://shared.test"), WithRateLimits(100, 25))
	b := NewClient("tok-shared-a", WithBaseURL("http://shared.test"), WithRateLimits(100, 25))
	c := NewClient("tok-shared-b", WithBaseURL("http://shared.test"), WithRateLimits(100, 25))
	if a.limitAll != b.limitAll || a.limitCreate != b.limitCreate || a.limitCommand != b.limitCommand {
		t.Error("two clients for the same organization have separate budgets")
	}
	if a.limitAll == c.limitAll {
		t.Error("clients with different tokens share a budget")
	}
	off := NewClient("tok-shared-a", WithBaseURL("http://shared.test"), WithRateLimits(0, 0))
	if off.limitAll != nil || off.limitCreate != nil || off.limitCommand != nil {
		t.Error("a rate of 0 did not turn pacing off")
	}
}

// fakeStatusServer answers with each status in turn, then 200 with {}.
func fakeStatusServer(t *testing.T, header http.Header, statuses ...int) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(atomic.AddInt32(&calls, 1))
		w.Header().Set("Content-Type", "application/json")
		if n <= len(statuses) {
			for k, v := range header {
				w.Header()[k] = v
			}
			w.WriteHeader(statuses[n-1])
			_, _ = w.Write([]byte(`{"message":"Too Many Attempts."}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func shortBackoff(t *testing.T) {
	old := retryBaseDelay
	retryBaseDelay = time.Millisecond
	t.Cleanup(func() { retryBaseDelay = old })
}

func TestGetRetriesThrottleAndHonorsRetryAfter(t *testing.T) {
	srv, calls := fakeStatusServer(t, http.Header{"Retry-After": {"1"}}, 429, 429)
	c := NewClient("t", WithBaseURL(srv.URL))

	start := time.Now()
	if err := c.do(context.Background(), "GET", "/applications/app-1", nil, nil); err != nil {
		t.Fatalf("GET after two 429s: %v", err)
	}
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Errorf("server saw %d requests, want 3", got)
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Errorf("retried after %v; Retry-After asked for 1s before each retry", elapsed)
	}
}

func TestGetRetryIsCapped(t *testing.T) {
	shortBackoff(t)
	statuses := make([]int, 20)
	for i := range statuses {
		statuses[i] = 503
	}
	srv, calls := fakeStatusServer(t, nil, statuses...)
	c := NewClient("t", WithBaseURL(srv.URL))

	err := c.do(context.Background(), "GET", "/applications/app-1", nil, nil)
	if err == nil {
		t.Fatal("GET succeeded although every response was 503")
	}
	if got := atomic.LoadInt32(calls); got != 1+maxUnavailableRetries {
		t.Errorf("server saw %d requests, want %d", got, 1+maxUnavailableRetries)
	}
}

func TestGetNotRetriedOnOtherErrors(t *testing.T) {
	shortBackoff(t)
	for _, status := range []int{400, 404, 422, 500} {
		srv, calls := fakeStatusServer(t, nil, status)
		c := NewClient("t", WithBaseURL(srv.URL))
		if err := c.do(context.Background(), "GET", "/x", nil, nil); err == nil {
			t.Errorf("GET %d: no error", status)
		}
		if got := atomic.LoadInt32(calls); got != 1 {
			t.Errorf("GET %d was sent %d times, want once", status, got)
		}
	}
}

// TestWritesNeverRetried: a write that failed may still have taken effect, so
// even a 429 or 503 on one is returned, not resent.
func TestWritesNeverRetried(t *testing.T) {
	shortBackoff(t)
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, status := range []int{429, 503} {
			srv, calls := fakeStatusServer(t, nil, status)
			c := NewClient("t", WithBaseURL(srv.URL))
			if err := c.do(context.Background(), method, "/applications", map[string]string{}, nil); err == nil {
				t.Errorf("%s answered %d: no error", method, status)
			}
			if got := atomic.LoadInt32(calls); got != 1 {
				t.Errorf("%s answered %d was sent %d times, want once", method, status, got)
			}
		}
	}
}

// TestThrottledReadPausesOtherRequests: after a 429, the next request through
// the same budget waits for the limit to reset rather than adding to it.
func TestThrottledReadPausesOtherRequests(t *testing.T) {
	srv, _ := fakeStatusServer(t, http.Header{"Retry-After": {"1"}}, 429)
	c := NewClient("tok-pause", WithBaseURL(srv.URL), WithRateLimits(6000, 0))
	other := NewClient("tok-pause", WithBaseURL(srv.URL), WithRateLimits(6000, 0))

	done := make(chan struct{})
	go func() {
		_ = c.do(context.Background(), "GET", "/a", nil, nil)
		close(done)
	}()
	time.Sleep(200 * time.Millisecond) // the 429 has arrived; c is waiting

	start := time.Now()
	if err := other.do(context.Background(), "POST", "/applications/app-1", map[string]string{}, nil); err != nil {
		t.Fatalf("POST: %v", err)
	}
	if waited := time.Since(start); waited < 500*time.Millisecond {
		t.Errorf("a request sent during the pause waited only %v", waited)
	}
	<-done
}

func TestPacingHonorsContext(t *testing.T) {
	srv, calls := fakeStatusServer(t, nil)
	c := NewClient("tok-ctx", WithBaseURL(srv.URL), WithRateLimits(1, 0))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := c.do(ctx, "GET", "/a", nil, nil); err != nil {
		t.Fatalf("first GET: %v", err)
	}
	if err := c.do(ctx, "GET", "/a", nil, nil); err == nil {
		t.Fatal("second GET at 1 a minute did not wait past the deadline")
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("server saw %d requests, want 1", got)
	}
}
