package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every request must name its client in the User-Agent, so Laravel Cloud's
// request logs can tell the provider, and which release of it, from any other
// Go program. Without one, Go sends "Go-http-client/1.1".
func TestDoSendsUserAgent(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []ClientOption
		want string
	}{
		{"default", nil, defaultUserAgent},
		{"configured", []ClientOption{WithUserAgent("terraform-provider-laravel/1.2.3")}, "terraform-provider-laravel/1.2.3"},
		{"empty keeps the default", []ClientOption{WithUserAgent("")}, defaultUserAgent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.UserAgent()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[]}`))
			}))
			defer srv.Close()

			c := NewClient("token", append([]ClientOption{WithBaseURL(srv.URL + "/api")}, tc.opts...)...)
			if err := c.do(context.Background(), "GET", "/applications", nil, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("User-Agent = %q, want %q", got, tc.want)
			}
		})
	}
}
