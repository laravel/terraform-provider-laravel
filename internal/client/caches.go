package client

import (
	"context"
	"fmt"
)

func (c *Client) ListCaches(ctx context.Context) ([]CacheData, error) {
	return listAll[CacheData](ctx, c, "/caches")
}

func (c *Client) GetCache(ctx context.Context, id string) (*CacheData, error) {
	var doc Document[CacheData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/caches/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateCache(ctx context.Context, req CreateCacheRequest) (*CacheData, error) {
	var doc Document[CacheData]
	if err := c.do(ctx, "POST", "/caches", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateCache(ctx context.Context, id string, req UpdateCacheRequest) (*CacheData, error) {
	var doc Document[CacheData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/caches/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteCache(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/caches/%s", id), nil, nil)
}
