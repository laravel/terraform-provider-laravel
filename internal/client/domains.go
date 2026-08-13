package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDomains(ctx context.Context, environmentID string) ([]DomainData, error) {
	return listAll[DomainData](ctx, c, fmt.Sprintf("/environments/%s/domains", environmentID))
}

func (c *Client) GetDomain(ctx context.Context, id string) (*DomainData, error) {
	var doc Document[DomainData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/domains/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateDomain(ctx context.Context, environmentID string, req CreateDomainRequest) (*DomainData, error) {
	var doc Document[DomainData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/domains", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateDomain(ctx context.Context, id string, req UpdateDomainRequest) (*DomainData, error) {
	var doc Document[DomainData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/domains/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteDomain(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/domains/%s", id), nil, nil)
}
