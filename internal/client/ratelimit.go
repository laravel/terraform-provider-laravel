package client

import (
	"context"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Laravel Cloud limits each organization to 120 API requests a minute, 30 a
// minute on most create routes, and 10 command runs a minute per user. The
// defaults stay a little under those so one Terraform run never reaches them.
const (
	DefaultMaxRequestsPerMinute       = 100
	DefaultMaxCreateRequestsPerMinute = 25
	commandRequestsPerMinute          = 8
)

// WithRateLimits paces requests client-side: at most perMinute requests a
// minute overall, and createPerMinute a minute on the routes Laravel Cloud
// throttles as resource creation. Zero turns that limit off.
//
// Pacing is how writes stay under the limits: Laravel Cloud answers a
// throttled request with a 429, and while a throttled read is retried, a
// write is sent only once, so a write that hits the limit fails the apply.
//
// Every client in the process with the same token, base URL and rates shares
// the same budget, so all the provider instances an acceptance test run
// starts draw from one bucket. Separate processes, such as two terraform
// runs against one organization, do not share it.
func WithRateLimits(perMinute, createPerMinute int) ClientOption {
	return func(c *Client) {
		c.maxPerMinute = perMinute
		c.maxCreatePerMinute = createPerMinute
	}
}

// limiter is a token bucket (as GCRA): requests are spaced interval apart,
// with up to burst of them allowed back to back after a quiet spell.
type limiter struct {
	interval time.Duration
	burst    int

	mu sync.Mutex
	// tat is the theoretical arrival time of the next request: when it
	// could go if every earlier request had been perfectly spaced.
	tat time.Time
	// pausedUntil holds every request back after the API reported a limit.
	pausedUntil time.Time
}

func newLimiter(perMinute int) *limiter {
	// A burst of a tenth of a minute's budget keeps any 60-second window
	// within perMinute+burst, still under Cloud's limit for the defaults.
	return &limiter{
		interval: time.Minute / time.Duration(perMinute),
		burst:    max(1, perMinute/10),
	}
}

var (
	sharedLimitersMu sync.Mutex
	sharedLimiters   = map[string]*limiter{}
)

// sharedLimiter returns the process-wide limiter for key at perMinute, or nil
// when perMinute turns pacing off.
func sharedLimiter(key string, perMinute int) *limiter {
	if perMinute <= 0 {
		return nil
	}
	key += "\x00" + strconv.Itoa(perMinute)
	sharedLimitersMu.Lock()
	defer sharedLimitersMu.Unlock()
	l, ok := sharedLimiters[key]
	if !ok {
		l = newLimiter(perMinute)
		sharedLimiters[key] = l
	}
	return l
}

// reserve claims the next slot and returns when it may be used.
func (l *limiter) reserve(now time.Time) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	tat := l.tat
	if tat.Before(now) {
		tat = now
	}
	at := tat.Add(-time.Duration(l.burst-1) * l.interval)
	if at.Before(now) {
		at = now
	}
	if at.Before(l.pausedUntil) {
		at = l.pausedUntil
		if tat.Before(at) {
			tat = at
		}
	}
	l.tat = tat.Add(l.interval)
	return at
}

// pause holds every request back until t. The requests waiting for it then
// resume one interval apart, without the burst a quiet spell would allow:
// they were throttled, so the budget is not quiet.
func (l *limiter) pause(t time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t.After(l.pausedUntil) {
		l.pausedUntil = t
		if spaced := t.Add(time.Duration(l.burst-1) * l.interval); l.tat.Before(spaced) {
			l.tat = spaced
		}
	}
}

// paused reports whether a pause holds requests back at now.
func (l *limiter) paused(now time.Time) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return now.Before(l.pausedUntil)
}

// wait blocks until a request may be sent, or ctx is done. A pause that
// starts while it sleeps holds it back too: it then takes a new slot after
// the pause instead of going at the time it reserved.
func (l *limiter) wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	for {
		if err := sleepCtx(ctx, time.Until(l.reserve(time.Now()))); err != nil {
			return err
		}
		if !l.paused(time.Now()) {
			return nil
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// createRoutes are the routes the provider calls that Laravel Cloud throttles
// with api-resource-creation (30 a minute per organization), from
// routes/public-api.php. A path segment of * is an ID.
var createRoutes = []string{
	"POST /applications",
	"POST /applications/*/environments",
	"POST /environments/*/instances",
	"PATCH /instances/*",
	"GET /environments/*/logs",
	"POST /databases",
	"POST /databases/clusters",
	"POST /databases/clusters/*/users",
	"POST /databases/clusters/*/snapshots",
	"POST /databases/clusters/*/restore",
	"POST /buckets",
	"POST /caches",
	"POST /websocket-servers",
	"POST /secrets",
}

// rateClass reports whether a request counts against Cloud's creation limit,
// its command limit, or neither. p is the path relative to the base URL.
func rateClass(method, p string) (create, command bool) {
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	p = "/" + strings.Trim(p, "/")
	if method == http.MethodPost {
		if ok, _ := path.Match("/environments/*/commands", p); ok {
			return false, true
		}
	}
	for _, r := range createRoutes {
		m, pattern, _ := strings.Cut(r, " ")
		if ok, _ := path.Match(pattern, p); ok && m == method {
			return true, false
		}
	}
	return false, false
}

// Retries of reads that failed only because of load. Writes are never
// retried: whether a failed write took effect is unknown.
const (
	maxThrottleRetries    = 5
	maxUnavailableRetries = 3
	maxRetryWait          = time.Minute
)

// retryBaseDelay is the first backoff when the response says nothing about
// when to retry. A variable so tests can shorten it.
var retryBaseDelay = time.Second

// retryDelay is how long to wait before retry number attempt (from 1) of a
// read that got status and header. A 429 names the wait in Retry-After, or the reset time
// in X-RateLimit-Reset; otherwise the delay doubles each attempt. It never
// exceeds maxRetryWait.
func retryDelay(status int, header http.Header, attempt int, now time.Time) time.Duration {
	d := time.Duration(-1)
	if status == http.StatusTooManyRequests {
		if s := header.Get("Retry-After"); s != "" {
			if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
				d = time.Duration(secs) * time.Second
			} else if t, err := http.ParseTime(s); err == nil {
				d = max(0, t.Sub(now))
			}
		}
		if d < 0 {
			if s := header.Get("X-RateLimit-Reset"); s != "" {
				if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
					d = max(0, time.Unix(unix, 0).Sub(now))
				}
			}
		}
	}
	if d < 0 {
		d = retryBaseDelay << (attempt - 1)
	}
	return min(d, maxRetryWait)
}

// shouldRetry reports whether a read that got status may be retried, given
// how many retries it has had.
func shouldRetry(method string, status, retries int) bool {
	if method != http.MethodGet {
		return false
	}
	switch status {
	case http.StatusTooManyRequests:
		return retries < maxThrottleRetries
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return retries < maxUnavailableRetries
	}
	return false
}
