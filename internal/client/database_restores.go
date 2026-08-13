package client

import (
	"context"
	"fmt"
)

func (c *Client) CreateDatabaseRestore(ctx context.Context, databaseClusterID string, req CreateDatabaseRestoreRequest) (*DatabaseClusterData, error) {
	var doc Document[DatabaseClusterData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/databases/clusters/%s/restore", databaseClusterID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}
