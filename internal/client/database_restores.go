package client

import (
	"context"
	"fmt"
)

func (c *Client) CreateDatabaseRestore(ctx context.Context, databaseClusterID string, req CreateDatabaseRestoreRequest) (*DatabaseClusterData, error) {
	return fetch[DatabaseClusterData](ctx, c, "POST", fmt.Sprintf("/databases/clusters/%s/restore", databaseClusterID), req)
}
