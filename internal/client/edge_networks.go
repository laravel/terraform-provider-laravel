package client

import "context"

// ListEdgeNetworks returns every edge network in the organization.
//
// Not to be confused with ListIPAddresses: an edge network is a CDN zone
// (name, domain, tenancy, status) and carries no IP addresses at all. The
// egress IP ranges come from the separate, unauthenticated /ip route.
func (c *Client) ListEdgeNetworks(ctx context.Context) ([]EdgeNetworkData, error) {
	return listAll[EdgeNetworkData](ctx, c, "/edge-networks")
}
