package client

import (
	"context"
	"fmt"
)

// AttachResend connects Resend to an environment, creating a sending key or
// reusing one from another environment.
func (c *Client) AttachResend(ctx context.Context, environmentID string, req AttachResendRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/resend", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// UpdateResend changes the sender of an environment's Resend integration. The
// sending key cannot be changed in place.
func (c *Client) UpdateResend(ctx context.Context, environmentID string, req UpdateResendRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/environments/%s/resend", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DetachResend(ctx context.Context, environmentID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/environments/%s/resend", environmentID), nil, nil)
}
