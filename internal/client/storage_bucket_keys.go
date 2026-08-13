package client

import (
	"context"
	"fmt"
)

func (c *Client) ListStorageBucketKeys(ctx context.Context, bucketID string) ([]StorageBucketKeyData, error) {
	return listAll[StorageBucketKeyData](ctx, c, fmt.Sprintf("/buckets/%s/keys", bucketID))
}

// Only list and create are nested under the bucket. Every per-item route is
// flat, at /bucket-keys/{id} -- the nested forms are not registered on the API:
// the GET 404s and the PATCH/DELETE fall through to the web catch-all and answer
// 302 to the app root. This is the same shape as the websocket application
// routes below.
func (c *Client) GetStorageBucketKey(ctx context.Context, id string) (*StorageBucketKeyData, error) {
	var doc Document[StorageBucketKeyData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/bucket-keys/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateStorageBucketKey(ctx context.Context, bucketID string, req CreateStorageBucketKeyRequest) (*StorageBucketKeyData, error) {
	var doc Document[StorageBucketKeyData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/buckets/%s/keys", bucketID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateStorageBucketKey(ctx context.Context, id string, req UpdateStorageBucketKeyRequest) (*StorageBucketKeyData, error) {
	var doc Document[StorageBucketKeyData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/bucket-keys/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteStorageBucketKey(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/bucket-keys/%s", id), nil, nil)
}
