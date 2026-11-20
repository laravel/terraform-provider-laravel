package client

import (
	"context"
	"fmt"
)

// Database users live under their cluster, and every route needs both ids.
// The API reports a user's databases only with include=databases.

func (c *Client) ListDatabaseUsers(ctx context.Context, clusterID string) ([]DatabaseUserData, error) {
	return listAll[DatabaseUserData](ctx, c, fmt.Sprintf("/databases/clusters/%s/users", clusterID))
}

func (c *Client) GetDatabaseUser(ctx context.Context, clusterID, id string) (*DatabaseUserData, error) {
	var doc Document[DatabaseUserData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/databases/clusters/%s/users/%s?include=databases", clusterID, id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// CreateDatabaseUser creates a user. The response is the only one that
// carries the user's password.
func (c *Client) CreateDatabaseUser(ctx context.Context, clusterID string, req CreateDatabaseUserRequest) (*DatabaseUserData, error) {
	var doc Document[DatabaseUserData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/databases/clusters/%s/users?include=databases", clusterID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateDatabaseUser(ctx context.Context, clusterID, id string, req UpdateDatabaseUserRequest) (*DatabaseUserData, error) {
	var doc Document[DatabaseUserData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/databases/clusters/%s/users/%s?include=databases", clusterID, id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteDatabaseUser(ctx context.Context, clusterID, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/databases/clusters/%s/users/%s", clusterID, id), nil, nil)
}
