package client

import (
	"context"
	"fmt"
)

func (c *Client) ListScheduledCommands(ctx context.Context, instanceID string) ([]ScheduledCommandData, error) {
	return listAll[ScheduledCommandData](ctx, c, fmt.Sprintf("/instances/%s/scheduled-commands", instanceID))
}

// GetScheduledCommand fetches a single scheduled command. include=instance is
// required to get the relationships block; see GetInstance.
func (c *Client) GetScheduledCommand(ctx context.Context, id string) (*ScheduledCommandData, error) {
	var doc Document[ScheduledCommandData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/scheduled-commands/%s?include=instance", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateScheduledCommand(ctx context.Context, instanceID string, req CreateScheduledCommandRequest) (*ScheduledCommandData, error) {
	var doc Document[ScheduledCommandData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/instances/%s/scheduled-commands", instanceID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateScheduledCommand(ctx context.Context, id string, req UpdateScheduledCommandRequest) (*ScheduledCommandData, error) {
	var doc Document[ScheduledCommandData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/scheduled-commands/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteScheduledCommand(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/scheduled-commands/%s", id), nil, nil)
}
