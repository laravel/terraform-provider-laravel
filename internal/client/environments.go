package client

import (
	"context"
	"fmt"
)

func (c *Client) ListEnvironments(ctx context.Context, applicationID string) ([]EnvironmentData, error) {
	return listAll[EnvironmentData](ctx, c, fmt.Sprintf("/applications/%s/environments", applicationID))
}

func (c *Client) GetEnvironment(ctx context.Context, id string) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/environments/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateEnvironment(ctx context.Context, applicationID string, req CreateEnvironmentRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/applications/%s/environments", applicationID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, id string, req UpdateEnvironmentRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/environments/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/environments/%s", id), nil, nil)
}

// UpdateVanityDomainRequest is the body of PUT /environments/{id}/vanity-domain.
type UpdateVanityDomainRequest struct {
	Name string `json:"name"`
}

// SetVanityDomain renames the environment's vanity domain.
//
// This is a dedicated PUT route rather than a field on the environment PATCH
// body, which is why it is a separate call.
func (c *Client) SetVanityDomain(ctx context.Context, environmentID string, req UpdateVanityDomainRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "PUT", fmt.Sprintf("/environments/%s/vanity-domain", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}
