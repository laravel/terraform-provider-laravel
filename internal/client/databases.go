package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDatabases(ctx context.Context, clusterID string) ([]DatabaseData, error) {
	return listAll[DatabaseData](ctx, c, fmt.Sprintf("/databases/clusters/%s/databases", clusterID))
}

// GetDatabase fetches a single database within a cluster.
//
// The flat GET /databases/{id} route this used to call is the *deprecated
// database-cluster* show route, not a per-database one: in the API's vocabulary a
// "database" is the cluster (JSON:API type "databases") and the thing inside it
// is a schema (type "databaseSchemas"). Passing a schema id to the cluster route
// therefore 404s correctly -- which had been read as an API bug (issue #30) and
// worked around by scanning the cluster listing. The real per-database route is
// nested and needs both ids.
func (c *Client) GetDatabase(ctx context.Context, clusterID, id string) (*DatabaseData, error) {
	var doc Document[DatabaseData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/databases/clusters/%s/databases/%s", clusterID, id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateDatabase(ctx context.Context, clusterID string, req CreateDatabaseRequest) (*DatabaseData, error) {
	var doc Document[DatabaseData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/databases/clusters/%s/databases", clusterID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteDatabase(ctx context.Context, clusterID, id string) error {
	// The flat DELETE /databases/{id} route is not accepted by the API; a
	// schema is removed through its cluster-nested route (issue #30).
	return c.do(ctx, "DELETE", fmt.Sprintf("/databases/clusters/%s/databases/%s", clusterID, id), nil, nil)
}
