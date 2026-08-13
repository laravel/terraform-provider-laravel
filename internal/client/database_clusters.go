package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDatabaseClusters(ctx context.Context) ([]DatabaseClusterData, error) {
	return listAll[DatabaseClusterData](ctx, c, "/databases/clusters")
}

func (c *Client) GetDatabaseCluster(ctx context.Context, id string) (*DatabaseClusterData, error) {
	var doc Document[DatabaseClusterData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/databases/clusters/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateDatabaseCluster(ctx context.Context, req CreateDatabaseClusterRequest) (*DatabaseClusterData, error) {
	var doc Document[DatabaseClusterData]
	if err := c.do(ctx, "POST", "/databases/clusters", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateDatabaseCluster(ctx context.Context, id string, req UpdateDatabaseClusterRequest) (*DatabaseClusterData, error) {
	var doc Document[DatabaseClusterData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/databases/clusters/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteDatabaseCluster(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/databases/clusters/%s", id), nil, nil)
}
