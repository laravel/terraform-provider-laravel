package client

import "context"

// ListDedicatedClusters returns the organization's dedicated clusters. filters
// maps "region", "type" and "status" to a value each must contain; the API
// matches them as substrings, not exactly.
func (c *Client) ListDedicatedClusters(ctx context.Context, filters map[string]string) ([]DedicatedClusterData, error) {
	return listAll[DedicatedClusterData](ctx, c, "/dedicated-clusters"+filterQuery(filters))
}
