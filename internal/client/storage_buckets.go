package client

import (
	"context"
	"fmt"
)

func (c *Client) ListStorageBuckets(ctx context.Context) ([]StorageBucketData, error) {
	return listAll[StorageBucketData](ctx, c, "/buckets")
}

func (c *Client) GetStorageBucket(ctx context.Context, id string) (*StorageBucketData, error) {
	var doc Document[StorageBucketData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/buckets/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateStorageBucket(ctx context.Context, req CreateStorageBucketRequest) (*StorageBucketData, error) {
	var doc Document[StorageBucketData]
	if err := c.do(ctx, "POST", "/buckets", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateStorageBucket(ctx context.Context, id string, req UpdateStorageBucketRequest) (*StorageBucketData, error) {
	var doc Document[StorageBucketData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/buckets/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteStorageBucket(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/buckets/%s", id), nil, nil)
}
