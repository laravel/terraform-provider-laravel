package client

import (
	"context"
	"fmt"
)

func (c *Client) ListInstances(ctx context.Context, environmentID string) ([]InstanceData, error) {
	return listAll[InstanceData](ctx, c, fmt.Sprintf("/environments/%s/instances", environmentID))
}

// GetInstance fetches a single instance.
//
// include=environment is required: the API omits the relationships block
// entirely unless it is asked for, and the owning environment id is not
// otherwise recoverable on import.
func (c *Client) GetInstance(ctx context.Context, id string) (*InstanceData, error) {
	return fetch[InstanceData](ctx, c, "GET", fmt.Sprintf("/instances/%s?include=environment", id), nil)
}

func (c *Client) CreateInstance(ctx context.Context, environmentID string, req CreateInstanceRequest) (*InstanceData, error) {
	return fetch[InstanceData](ctx, c, "POST", fmt.Sprintf("/environments/%s/instances", environmentID), req)
}

func (c *Client) UpdateInstance(ctx context.Context, id string, req UpdateInstanceRequest) (*InstanceData, error) {
	return fetch[InstanceData](ctx, c, "PATCH", fmt.Sprintf("/instances/%s", id), req)
}

func (c *Client) DeleteInstance(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/instances/%s", id), nil, nil)
}
