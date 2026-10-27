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
	var doc Document[InstanceData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/instances/%s?include=environment", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateInstance(ctx context.Context, environmentID string, req CreateInstanceRequest) (*InstanceData, error) {
	var doc Document[InstanceData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/instances", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateInstance(ctx context.Context, id string, req UpdateInstanceRequest) (*InstanceData, error) {
	var doc Document[InstanceData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/instances/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// SetDefaultInstance makes a managed queue the environment's default, the one
// a job is sent to when it names no queue. There is no call to unset it: the
// default moves when another queue is made the default.
func (c *Client) SetDefaultInstance(ctx context.Context, id string) error {
	return c.do(ctx, "POST", fmt.Sprintf("/instances/%s/default", id), nil, nil)
}

func (c *Client) DeleteInstance(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/instances/%s", id), nil, nil)
}
