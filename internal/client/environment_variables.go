package client

import (
	"context"
	"fmt"
)

func (c *Client) SetEnvironmentVariables(ctx context.Context, environmentID string, req AddEnvironmentVariablesRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/variables", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

type DeleteEnvironmentVariablesRequest struct {
	Keys []string `json:"keys"`
}

func (c *Client) DeleteEnvironmentVariables(ctx context.Context, environmentID string, req DeleteEnvironmentVariablesRequest) error {
	return c.do(ctx, "POST", fmt.Sprintf("/environments/%s/variables/delete", environmentID), req, nil)
}
