package client

import "context"

func (c *Client) GetOrganization(ctx context.Context) (*OrganizationData, error) {
	var doc Document[OrganizationData]
	if err := c.do(ctx, "GET", "/meta/organization", nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}
