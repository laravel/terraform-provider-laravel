package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://cloud.laravel.com/api"

// maxResponseSize limits response body reads to 1MB to prevent unbounded memory allocation
// from unexpected large responses (e.g. HTML error pages).
const maxResponseSize = 1 << 20

// Client is the Laravel Cloud API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	// Client-side pacing; see WithRateLimits. The limiters are nil when
	// pacing is off.
	maxPerMinute, maxCreatePerMinute    int
	limitAll, limitCreate, limitCommand *limiter
}

// NewClient creates a new Laravel Cloud API client.
func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	// Never follow redirects. Some API write routes answer with a 302 back to
	// the app root; following it downgrades POST/DELETE to GET (per RFC 7231)
	// and lands on the dashboard SPA, which hides the redirect behind an
	// "unexpected HTML" error. Surfacing the 302 is the actual diagnosis.
	if c.httpClient.CheckRedirect == nil {
		c.httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	key := c.baseURL + "\x00" + c.token
	c.limitAll = sharedLimiter(key+"\x00all", c.maxPerMinute)
	c.limitCreate = sharedLimiter(key+"\x00create", c.maxCreatePerMinute)
	if c.limitAll != nil {
		c.limitCommand = sharedLimiter(key+"\x00command", commandRequestsPerMinute)
	}
	return c
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default API base URL.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = h }
}

// ---------------------------------------------------------------------
// Low-level HTTP helpers
// ---------------------------------------------------------------------

// do sends one API request, paced by the client's limiters. A read that the
// API refused because of load (429, 502, 503, 504) is retried after a delay;
// a 429 also holds back every other request this process sends to the
// organization until the limit resets. Writes are sent once.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	create, command := rateClass(method, path)
	for retries := 0; ; retries++ {
		if err := c.waitToSend(ctx, create, command); err != nil {
			return err
		}

		status, header, err := c.send(ctx, method, path, body, out)
		delay := retryDelay(status, header, retries+1, time.Now())
		// Any 429, a write's or a last retry's too, holds back the other
		// requests, which would otherwise keep sending into the limit.
		if status == http.StatusTooManyRequests && c.limitAll != nil {
			c.limitAll.pause(time.Now().Add(delay))
		}
		if err == nil || !shouldRetry(method, status, retries) {
			return err
		}
		if err := sleepCtx(ctx, delay); err != nil {
			return err
		}
	}
}

// waitToSend waits on every limiter a request counts against. If a 429
// pauses the organization's budget while the request waits on the creation
// or command limit, it waits on the overall budget again before sending.
func (c *Client) waitToSend(ctx context.Context, create, command bool) error {
	for {
		if err := c.limitAll.wait(ctx); err != nil {
			return err
		}
		if create {
			if err := c.limitCreate.wait(ctx); err != nil {
				return err
			}
		}
		if command {
			if err := c.limitCommand.wait(ctx); err != nil {
				return err
			}
		}
		if !c.limitAll.paused(time.Now()) {
			return nil
		}
	}
}

// send makes one HTTP request and decodes its response into out. It returns
// the response status and headers, when there was a response, with any error.
func (c *Client) send(ctx context.Context, method, path string, body any, out any) (status int, header http.Header, err error) {
	// Split any query string off before joining: url.JoinPath treats the whole
	// argument as path segments and would escape "?" to "%3F".
	reqPath, rawQuery := path, ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		reqPath, rawQuery = path[:i], path[i+1:]
	}
	u, err := url.JoinPath(c.baseURL, reqPath)
	if err != nil {
		return 0, nil, fmt.Errorf("building URL: %w", err)
	}
	if rawQuery != "" {
		u += "?" + rawQuery
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	status, header = resp.StatusCode, resp.Header

	// Read one byte past the cap so a truncated body is reported as such
	// rather than as the JSON syntax error truncation would otherwise cause.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return status, header, fmt.Errorf("reading response body: %w", err)
	}
	if len(respBody) > maxResponseSize {
		return status, header, fmt.Errorf("response from %s %s exceeds the %d byte limit", method, u, maxResponseSize)
	}

	// A redirect means the request never reached the API handler it was aimed
	// at -- report it verbatim rather than whatever the redirect target serves.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc == "" {
			loc = "(no Location header)"
		}
		// Laravel Cloud answers a write it throttled this way (with no
		// rate-limit headers), so say so: it is the likeliest cause.
		return status, header, fmt.Errorf("%s %s was answered with an unexpected %d redirect to %s; the API did not handle this request. "+
			"It may be a rate limit: Laravel Cloud answers a write that exceeds one this way. Wait a minute and retry, "+
			"or lower max_requests_per_minute if other runs share the organization", method, u, resp.StatusCode, loc)
	}

	// Detect non-JSON responses (e.g. HTML error pages, SPA catch-all).
	// This can happen on both error and success status codes when the
	// request hits a frontend route instead of the API.
	if len(respBody) > 0 && respBody[0] == '<' {
		ct := resp.Header.Get("Content-Type")
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return status, header, fmt.Errorf("expected JSON response from %s %s but received Content-Type %q (status %d): %s", method, u, ct, resp.StatusCode, preview)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return status, header, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if out != nil && len(respBody) > 0 {
		ct := resp.Header.Get("Content-Type")
		if err := json.Unmarshal(respBody, out); err != nil {
			return status, header, fmt.Errorf("decoding response from %s %s (Content-Type: %q, status %d): %w", method, u, ct, resp.StatusCode, err)
		}
	}
	return status, header, nil
}

// listAll walks every page of a paginated collection route and returns the
// concatenated items.
//
// Every collection route in the public API is paginated: the response carries
// links/meta and defaults to 100 items per page, and per_page is not honoured,
// so a single GET only ever sees the first page. Reading just that page makes a
// Read that filters a listing report "not found" for a resource that exists,
// which drops it from state and proposes recreating live infrastructure.
//
// A response with no paginator meta (LastPage zero) is treated as one complete
// page, so non-paginated routes and test fakes keep working unchanged.
func listAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		p := path
		if page > 1 {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			p = fmt.Sprintf("%s%spage=%d", path, sep, page)
		}

		var doc ListDocument[T]
		if err := c.do(ctx, "GET", p, nil, &doc); err != nil {
			return nil, err
		}
		all = append(all, doc.Data...)

		// An empty page also terminates: it stops a server that reports a
		// last_page it never reaches from looping.
		if doc.Meta.LastPage <= page || len(doc.Data) == 0 {
			return all, nil
		}
	}
}

// APIError is returned when the API responds with a non-2xx status code.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("laravel cloud API error (status %d): %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

// IsBusy returns true if the API refused the request because the resource, or
// the database cluster it belongs to, has another operation in progress -- a
// cluster that is still being created counts. The same request succeeds once
// that operation finishes.
//
// The API has no error code for this, only a 422 validation message, and the
// wording differs by route: "This database cluster is already being updated.
// Try again shortly." when creating or deleting a database, and "An update is
// already in progress for this database. Try again in a few seconds." when
// deleting a cluster or restoring one. A WebSocket server that is creating or
// updating answers "An update operation is already in progress for this
// WebSocket cluster." or, on delete, "An operation is already in progress for
// this WebSocket cluster." An environment refuses to attach a database or cache
// that is creating or updating with "The selected cache is still being
// provisioned. Try again in a few seconds." (or database, or database cluster).
// A managed queue whose SQS queue is still creating or updating refuses to be
// paused, resumed or made the default with "This queue is not available yet.
// Wait a few seconds and try again."
func IsBusy(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	return strings.Contains(apiErr.Body, "already being updated") ||
		strings.Contains(apiErr.Body, "already in progress") ||
		strings.Contains(apiErr.Body, "still being provisioned") ||
		strings.Contains(apiErr.Body, "not available yet")
}

// IsBranchUnavailable returns true if the API refused to create an environment
// because Laravel Cloud's own record of the application's branches has no row
// for the requested branch: `422 {"branch": ["The selected branch is no longer
// available."]}`. Cloud answers this while it has not yet synced a non-default
// branch of a newly connected repository, and also for an application whose
// repository is null.
func IsBranchUnavailable(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity &&
		strings.Contains(apiErr.Body, "The selected branch is no longer available")
}

// IsClusterNotEmpty returns true if the API refused to delete a database
// cluster because it still has databases.
func IsClusterNotEmpty(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnprocessableEntity &&
		strings.Contains(apiErr.Body, "still has databases")
}

// RetryOnConflict retries operation up to maxAttempts when isRetryable returns
// true for the error. It respects context cancellation between attempts.
//
// An error isRetryable rejects is returned immediately. Falling through to the
// next attempt instead would re-run the operation with no delay until the
// attempt budget ran out -- turning one rejected create into 30 identical POSTs
// in a few microseconds, each able to leave behind a resource of its own.
func RetryOnConflict(ctx context.Context, maxAttempts int, interval time.Duration, operation func() error, isRetryable func(error) bool) error {
	for attempt := 1; ; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		if attempt >= maxAttempts || !isRetryable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
