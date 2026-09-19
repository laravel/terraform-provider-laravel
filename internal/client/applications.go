package client

import (
	"context"
	"fmt"
)

func (c *Client) ListApplications(ctx context.Context) ([]ApplicationData, error) {
	return listAll[ApplicationData](ctx, c, "/applications")
}

func (c *Client) GetApplication(ctx context.Context, id string) (*ApplicationData, error) {
	return fetch[ApplicationData](ctx, c, "GET", fmt.Sprintf("/applications/%s", id), nil)
}

func (c *Client) CreateApplication(ctx context.Context, req CreateApplicationRequest) (*ApplicationData, error) {
	return fetch[ApplicationData](ctx, c, "POST", "/applications", req)
}

func (c *Client) UpdateApplication(ctx context.Context, id string, req UpdateApplicationRequest) (*ApplicationData, error) {
	return fetch[ApplicationData](ctx, c, "PATCH", fmt.Sprintf("/applications/%s", id), req)
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/applications/%s", id), nil, nil)
}
