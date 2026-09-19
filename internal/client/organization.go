package client

import "context"

func (c *Client) GetOrganization(ctx context.Context) (*OrganizationData, error) {
	return fetch[OrganizationData](ctx, c, "GET", "/meta/organization", nil)
}
