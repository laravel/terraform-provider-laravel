package client

import (
	"context"
	"fmt"
)

func (c *Client) ListCommands(ctx context.Context, environmentID string) ([]CommandData, error) {
	return listAll[CommandData](ctx, c, fmt.Sprintf("/environments/%s/commands", environmentID))
}

func (c *Client) GetCommand(ctx context.Context, id string) (*CommandData, error) {
	var doc Document[CommandData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/commands/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateCommand(ctx context.Context, environmentID string, req CreateCommandRequest) (*CommandData, error) {
	var doc Document[CommandData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/commands", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}
