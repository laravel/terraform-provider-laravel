package client

import "context"

func (c *Client) ListDedicatedClusters(ctx context.Context) ([]DedicatedClusterData, error) {
	return listAll[DedicatedClusterData](ctx, c, "/dedicated-clusters")
}
