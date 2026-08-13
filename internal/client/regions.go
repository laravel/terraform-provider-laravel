package client

import "context"

func (c *Client) ListRegions(ctx context.Context) ([]RegionInfo, error) {
	var resp struct {
		Data []RegionInfo `json:"data"`
	}
	if err := c.do(ctx, "GET", "/meta/regions", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
