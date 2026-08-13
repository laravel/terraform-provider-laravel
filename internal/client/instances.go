package client

import (
	"context"
	"fmt"
)

func (c *Client) ListInstances(ctx context.Context, environmentID string) ([]InstanceData, error) {
	return listAll[InstanceData](ctx, c, fmt.Sprintf("/environments/%s/instances", environmentID))
}

func (c *Client) GetInstance(ctx context.Context, id string) (*InstanceData, error) {
	var doc Document[InstanceData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/instances/%s", id), nil, &doc); err != nil {
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

func (c *Client) DeleteInstance(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/instances/%s", id), nil, nil)
}
