package client

import (
	"context"
	"fmt"
)

func (c *Client) ListApplications(ctx context.Context) ([]ApplicationData, error) {
	return listAll[ApplicationData](ctx, c, "/applications")
}

// GetApplication fetches a single application. include=defaultEnvironment is
// required: the API omits the relationships block unless it is asked for.
func (c *Client) GetApplication(ctx context.Context, id string) (*ApplicationData, error) {
	var doc Document[ApplicationData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/applications/%s?include=defaultEnvironment", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateApplication(ctx context.Context, req CreateApplicationRequest) (*ApplicationData, error) {
	var doc Document[ApplicationData]
	if err := c.do(ctx, "POST", "/applications", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateApplication(ctx context.Context, id string, req UpdateApplicationRequest) (*ApplicationData, error) {
	var doc Document[ApplicationData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/applications/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/applications/%s", id), nil, nil)
}
