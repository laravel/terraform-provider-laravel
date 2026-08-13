package client

import "context"

func (c *Client) ListInstanceSizes(ctx context.Context) (*InstanceSizesResponse, error) {
	var resp InstanceSizesResponse
	if err := c.do(ctx, "GET", "/instances/sizes", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
