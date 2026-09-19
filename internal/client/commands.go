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
	// include=environment is required to recover the parent id on import.
	if err := c.do(ctx, "GET", fmt.Sprintf("/commands/%s?include=environment", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateCommand(ctx context.Context, environmentID string, req CreateCommandRequest) (*CommandData, error) {
	return fetch[CommandData](ctx, c, "POST", fmt.Sprintf("/environments/%s/commands", environmentID), req)
}
