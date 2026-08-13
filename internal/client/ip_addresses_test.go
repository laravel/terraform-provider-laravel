package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Verbatim shape of the live GET /ip response: a bare region-keyed object with
// no JSON:API "data" envelope. The client used to decode {"data": []string},
// which unmarshals this successfully into nothing -- so the data source returned
// an empty list with no error, and a firewall rule built from it had no
// addresses at all.
const ipBody = `{
  "us-east-2": {"ipv4": ["3.147.129.242", "18.219.49.68"], "ipv6": ["2600:1f16:147c:c700::/56"]},
  "eu-west-1": {"ipv4": ["52.19.1.1"],                     "ipv6": ["2a05:d018::/56"]}
}`

func TestListIPAddressesDecodesRegionKeyedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ip" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("region"); got != "" {
			t.Errorf("expected no region param, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ipBody))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	got, err := c.ListIPAddresses(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("returned no addresses — the response envelope is being misread")
	}
	// Every IPv4 and IPv6 entry from both regions, sorted.
	want := []string{
		"18.219.49.68", "2600:1f16:147c:c700::/56", "2a05:d018::/56", "3.147.129.242", "52.19.1.1",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d addresses %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("address %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A region-filtered request drops the outer keying and returns the addresses
// directly, so it needs a different decode path than the unfiltered call.
func TestListIPAddressesRegionFiltered(t *testing.T) {
	var gotQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ipv4":["52.70.166.119","35.169.193.93"],"ipv6":["2600:1f18:124c:a900::/56"]}`))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	got, err := c.ListIPAddresses(context.Background(), "us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "region=us-east-1" {
		t.Errorf("query = %q, want %q", gotQuery, "region=us-east-1")
	}
	want := []string{"2600:1f18:124c:a900::/56", "35.169.193.93", "52.70.166.119"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("address %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// Go randomises map iteration, so without an explicit sort the same response
// would yield a different order per call and show up as a diff on every refresh.
func TestListIPAddressesIsStableAcrossCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ipBody))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	first, err := c.ListIPAddresses(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 12; i++ {
		next, err := c.ListIPAddresses(context.Background(), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Join(next, ",") != strings.Join(first, ",") {
			t.Fatalf("call %d returned a different order:\n first: %v\n  then: %v", i+2, first, next)
		}
	}
}

// An unknown region is a 422 from the API, and that must surface rather than
// being reported as an empty allowlist.
func TestListIPAddressesPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"The selected region is invalid."}`))
	}))
	defer srv.Close()

	c := NewClient("token", WithBaseURL(srv.URL+"/api"))

	if _, err := c.ListIPAddresses(context.Background(), "nope"); err == nil {
		t.Fatal("expected an error for an invalid region, got nil")
	}
}
