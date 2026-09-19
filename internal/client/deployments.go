package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDeployments(ctx context.Context, environmentID string) ([]DeploymentData, error) {
	return listAll[DeploymentData](ctx, c, fmt.Sprintf("/environments/%s/deployments", environmentID))
}

func (c *Client) GetDeployment(ctx context.Context, id string) (*DeploymentData, error) {
	var doc Document[DeploymentData]
	// include=environment is required to recover the parent id on import.
	if err := c.do(ctx, "GET", fmt.Sprintf("/deployments/%s?include=environment", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateDeployment(ctx context.Context, environmentID string) (*DeploymentData, error) {
	return fetch[DeploymentData](ctx, c, "POST", fmt.Sprintf("/environments/%s/deployments", environmentID), nil)
}
