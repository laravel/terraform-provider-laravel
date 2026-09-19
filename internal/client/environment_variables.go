package client

import (
	"context"
	"fmt"
)

func (c *Client) SetEnvironmentVariables(ctx context.Context, environmentID string, req AddEnvironmentVariablesRequest) (*EnvironmentData, error) {
	return fetch[EnvironmentData](ctx, c, "POST", fmt.Sprintf("/environments/%s/variables", environmentID), req)
}

type DeleteEnvironmentVariablesRequest struct {
	Keys []string `json:"keys"`
}

func (c *Client) DeleteEnvironmentVariables(ctx context.Context, environmentID string, req DeleteEnvironmentVariablesRequest) error {
	return c.do(ctx, "POST", fmt.Sprintf("/environments/%s/variables/delete", environmentID), req, nil)
}
