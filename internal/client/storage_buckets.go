package client

import (
	"context"
	"fmt"
)

func (c *Client) ListStorageBuckets(ctx context.Context) ([]StorageBucketData, error) {
	return listAll[StorageBucketData](ctx, c, "/buckets")
}

func (c *Client) GetStorageBucket(ctx context.Context, id string) (*StorageBucketData, error) {
	return fetch[StorageBucketData](ctx, c, "GET", fmt.Sprintf("/buckets/%s", id), nil)
}

func (c *Client) CreateStorageBucket(ctx context.Context, req CreateStorageBucketRequest) (*StorageBucketData, error) {
	return fetch[StorageBucketData](ctx, c, "POST", "/buckets", req)
}

func (c *Client) UpdateStorageBucket(ctx context.Context, id string, req UpdateStorageBucketRequest) (*StorageBucketData, error) {
	return fetch[StorageBucketData](ctx, c, "PATCH", fmt.Sprintf("/buckets/%s", id), req)
}

func (c *Client) DeleteStorageBucket(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/buckets/%s", id), nil, nil)
}
