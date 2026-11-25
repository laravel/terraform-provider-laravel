package client

import "context"

// ListEdgeNetworks returns every edge network in the organization, including
// the shared one. filters maps "name", "domain" and "status" to a value each
// must contain; the API matches them as substrings and accepts no others.
//
// Not to be confused with ListIPAddresses: an edge network is a CDN zone
// (name, domain, tenancy, status) and carries no IP addresses at all. The
// egress IP ranges come from the separate, unauthenticated /ip route.
func (c *Client) ListEdgeNetworks(ctx context.Context, filters map[string]string) ([]EdgeNetworkData, error) {
	return listAll[EdgeNetworkData](ctx, c, "/edge-networks"+filterQuery(filters))
}
