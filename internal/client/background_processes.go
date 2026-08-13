package client

import (
	"context"
	"fmt"
)

func (c *Client) ListBackgroundProcesses(ctx context.Context, instanceID string) ([]BackgroundProcessData, error) {
	return listAll[BackgroundProcessData](ctx, c, fmt.Sprintf("/instances/%s/background-processes", instanceID))
}

func (c *Client) GetBackgroundProcess(ctx context.Context, id string) (*BackgroundProcessData, error) {
	var doc Document[BackgroundProcessData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/background-processes/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateBackgroundProcess(ctx context.Context, instanceID string, req CreateBackgroundProcessRequest) (*BackgroundProcessData, error) {
	var doc Document[BackgroundProcessData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/instances/%s/background-processes", instanceID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateBackgroundProcess(ctx context.Context, id string, req UpdateBackgroundProcessRequest) (*BackgroundProcessData, error) {
	var doc Document[BackgroundProcessData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/background-processes/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteBackgroundProcess(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/background-processes/%s", id), nil, nil)
}
