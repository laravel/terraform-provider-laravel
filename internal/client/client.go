package client

import (
	"bytes"
	"context"
	"encoding/json"
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

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	// Split any query string off before joining: url.JoinPath treats the whole
	// argument as path segments and would escape "?" to "%3F".
	reqPath, rawQuery := path, ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		reqPath, rawQuery = path[:i], path[i+1:]
	}
	u, err := url.JoinPath(c.baseURL, reqPath)
	if err != nil {
		return fmt.Errorf("building URL: %w", err)
	}
	if rawQuery != "" {
		u += "?" + rawQuery
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	// A redirect means the request never reached the API handler it was aimed
	// at -- report it verbatim rather than whatever the redirect target serves.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc == "" {
			loc = "(no Location header)"
		}
		return fmt.Errorf("%s %s was answered with an unexpected %d redirect to %s; the API did not handle this request", method, u, resp.StatusCode, loc)
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
		return fmt.Errorf("expected JSON response from %s %s but received Content-Type %q (status %d): %s", method, u, ct, resp.StatusCode, preview)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if out != nil && len(respBody) > 0 {
		ct := resp.Header.Get("Content-Type")
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response from %s %s (Content-Type: %q, status %d): %w", method, u, ct, resp.StatusCode, err)
		}
	}
	return nil
}

// fetch performs a request whose response is a single JSON:API document and
// returns the decoded resource.
func fetch[T any](ctx context.Context, c *Client, method, path string, body any) (*T, error) {
	var doc Document[T]
	if err := c.do(ctx, method, path, body, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
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
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}

// RetryOnConflict retries operation up to maxAttempts when isRetryable returns
// true for the error. It respects context cancellation between attempts.
func RetryOnConflict(ctx context.Context, maxAttempts int, interval time.Duration, operation func() error, isRetryable func(error) bool) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}
		if attempt < maxAttempts && isRetryable(err) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
				continue
			}
		}
	}
	return err
}
