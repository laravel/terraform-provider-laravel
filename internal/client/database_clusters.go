package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDatabaseClusters(ctx context.Context) ([]DatabaseClusterData, error) {
	return listAll[DatabaseClusterData](ctx, c, "/databases/clusters")
}

func (c *Client) GetDatabaseCluster(ctx context.Context, id string) (*DatabaseClusterData, error) {
	return fetch[DatabaseClusterData](ctx, c, "GET", fmt.Sprintf("/databases/clusters/%s", id), nil)
}

func (c *Client) CreateDatabaseCluster(ctx context.Context, req CreateDatabaseClusterRequest) (*DatabaseClusterData, error) {
	return fetch[DatabaseClusterData](ctx, c, "POST", "/databases/clusters", req)
}

func (c *Client) UpdateDatabaseCluster(ctx context.Context, id string, req UpdateDatabaseClusterRequest) (*DatabaseClusterData, error) {
	return fetch[DatabaseClusterData](ctx, c, "PATCH", fmt.Sprintf("/databases/clusters/%s", id), req)
}

func (c *Client) DeleteDatabaseCluster(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/databases/clusters/%s", id), nil, nil)
}
