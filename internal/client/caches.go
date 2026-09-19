package client

import (
	"context"
	"fmt"
)

func (c *Client) ListCaches(ctx context.Context) ([]CacheData, error) {
	return listAll[CacheData](ctx, c, "/caches")
}

func (c *Client) GetCache(ctx context.Context, id string) (*CacheData, error) {
	return fetch[CacheData](ctx, c, "GET", fmt.Sprintf("/caches/%s", id), nil)
}

func (c *Client) CreateCache(ctx context.Context, req CreateCacheRequest) (*CacheData, error) {
	return fetch[CacheData](ctx, c, "POST", "/caches", req)
}

func (c *Client) UpdateCache(ctx context.Context, id string, req UpdateCacheRequest) (*CacheData, error) {
	return fetch[CacheData](ctx, c, "PATCH", fmt.Sprintf("/caches/%s", id), req)
}

func (c *Client) DeleteCache(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/caches/%s", id), nil, nil)
}
