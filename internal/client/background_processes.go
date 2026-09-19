package client

import (
	"context"
	"fmt"
)

func (c *Client) ListBackgroundProcesses(ctx context.Context, instanceID string) ([]BackgroundProcessData, error) {
	return listAll[BackgroundProcessData](ctx, c, fmt.Sprintf("/instances/%s/background-processes", instanceID))
}

// GetBackgroundProcess fetches a single background process. include=instance
// is required to get the relationships block; see GetInstance.
func (c *Client) GetBackgroundProcess(ctx context.Context, id string) (*BackgroundProcessData, error) {
	return fetch[BackgroundProcessData](ctx, c, "GET", fmt.Sprintf("/background-processes/%s?include=instance", id), nil)
}

func (c *Client) CreateBackgroundProcess(ctx context.Context, instanceID string, req CreateBackgroundProcessRequest) (*BackgroundProcessData, error) {
	return fetch[BackgroundProcessData](ctx, c, "POST", fmt.Sprintf("/instances/%s/background-processes", instanceID), req)
}

func (c *Client) UpdateBackgroundProcess(ctx context.Context, id string, req UpdateBackgroundProcessRequest) (*BackgroundProcessData, error) {
	return fetch[BackgroundProcessData](ctx, c, "PATCH", fmt.Sprintf("/background-processes/%s", id), req)
}

func (c *Client) DeleteBackgroundProcess(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/background-processes/%s", id), nil, nil)
}
