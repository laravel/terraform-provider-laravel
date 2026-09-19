package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDatabaseSnapshots(ctx context.Context, clusterID string) ([]DatabaseSnapshotData, error) {
	return listAll[DatabaseSnapshotData](ctx, c, fmt.Sprintf("/databases/clusters/%s/snapshots", clusterID))
}

func (c *Client) GetDatabaseSnapshot(ctx context.Context, id string) (*DatabaseSnapshotData, error) {
	var doc Document[DatabaseSnapshotData]
	// include=database is required to recover the cluster id on import. The
	// spec names the relationship "database" because a cluster is the
	// `databases` resource; see the create route above.
	if err := c.do(ctx, "GET", fmt.Sprintf("/database-snapshots/%s?include=database", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateDatabaseSnapshot(ctx context.Context, clusterID string, req CreateDatabaseSnapshotRequest) (*DatabaseSnapshotData, error) {
	var doc Document[DatabaseSnapshotData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/databases/clusters/%s/snapshots", clusterID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteDatabaseSnapshot(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/database-snapshots/%s", id), nil, nil)
}
