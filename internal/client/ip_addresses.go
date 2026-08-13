package client

import (
	"context"
	"net/url"
	"sort"
)

// ListIPAddresses returns Laravel Cloud's egress addresses for firewall
// allowlisting. A non-empty region limits the result to that region and is
// rejected with a 422 if the region does not exist; an empty region returns
// every region's addresses.
//
// GET /ip is absent from the OpenAPI spec -- it is unauthenticated and sits
// outside the token-authed public.* route group -- but it is live and documented
// in prose at https://laravel.com/cloud/docs/network. Unlike every other route it
// returns a bare object rather than a JSON:API document, and the shape depends on
// whether a region was requested: keyed by region without one, the addresses
// directly with one.
//
// The result is sorted because Go randomises map iteration order, and an unstable
// order would surface as a spurious diff on every refresh. IPv6 entries are CIDR
// blocks rather than single addresses.
func (c *Client) ListIPAddresses(ctx context.Context, region string) ([]string, error) {
	if region != "" {
		var one RegionIPAddresses
		if err := c.do(ctx, "GET", "/ip?region="+url.QueryEscape(region), nil, &one); err != nil {
			return nil, err
		}
		return sortedAddresses(one), nil
	}

	var byRegion IPAddressesResponse
	if err := c.do(ctx, "GET", "/ip", nil, &byRegion); err != nil {
		return nil, err
	}
	var all []string
	for _, r := range byRegion {
		all = append(all, sortedAddresses(r)...)
	}
	sort.Strings(all)
	return all, nil
}

func sortedAddresses(r RegionIPAddresses) []string {
	out := make([]string, 0, len(r.IPv4)+len(r.IPv6))
	out = append(out, r.IPv4...)
	out = append(out, r.IPv6...)
	sort.Strings(out)
	return out
}
