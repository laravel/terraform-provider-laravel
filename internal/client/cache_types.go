package client

import "context"

func (c *Client) ListCacheTypes(ctx context.Context) ([]CacheTypeInfo, error) {
	var resp CacheTypesResponse
	if err := c.do(ctx, "GET", "/caches/types", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
