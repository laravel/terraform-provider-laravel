package client

import (
	"context"
	"fmt"
)

func (c *Client) ListDomains(ctx context.Context, environmentID string) ([]DomainData, error) {
	return listAll[DomainData](ctx, c, fmt.Sprintf("/environments/%s/domains", environmentID))
}

// GetDomain fetches a single domain. include=environment is required to get
// the relationships block; see GetInstance.
func (c *Client) GetDomain(ctx context.Context, id string) (*DomainData, error) {
	return fetch[DomainData](ctx, c, "GET", fmt.Sprintf("/domains/%s?include=environment", id), nil)
}

func (c *Client) CreateDomain(ctx context.Context, environmentID string, req CreateDomainRequest) (*DomainData, error) {
	return fetch[DomainData](ctx, c, "POST", fmt.Sprintf("/environments/%s/domains", environmentID), req)
}

func (c *Client) UpdateDomain(ctx context.Context, id string, req UpdateDomainRequest) (*DomainData, error) {
	return fetch[DomainData](ctx, c, "PATCH", fmt.Sprintf("/domains/%s", id), req)
}

func (c *Client) DeleteDomain(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/domains/%s", id), nil, nil)
}
