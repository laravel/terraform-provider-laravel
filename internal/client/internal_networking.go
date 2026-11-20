package client

import (
	"context"
	"fmt"
)

// Internal networking lets other environments on the same dedicated cluster
// reach an environment over the private network. It is a Private Cloud
// feature: the organization needs the internal networking entitlement, and
// the environment must run on a dedicated cluster.

// EnvironmentInternalNetworking is the internal_networking object of an
// environment response. Hostname and AccessLevel are null while it is
// disabled; EnvironmentIDs is empty unless AccessLevel is "specific".
type EnvironmentInternalNetworking struct {
	Enabled        bool     `json:"enabled"`
	Hostname       *string  `json:"hostname"`
	AccessLevel    *string  `json:"access_level"`
	EnvironmentIDs []string `json:"environment_ids"`
}

// InternalNetworkingRequest sets who may reach the environment: "all"
// environments on its cluster, or the "specific" ones in EnvironmentIDs.
type InternalNetworkingRequest struct {
	AccessLevel    string   `json:"access_level"`
	EnvironmentIDs []string `json:"environment_ids,omitempty"`
}

// EnableInternalNetworking turns internal networking on. The API also accepts
// it for an environment that already has it, and then changes the access.
func (c *Client) EnableInternalNetworking(ctx context.Context, environmentID string, req InternalNetworkingRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/internal-networking", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// UpdateInternalNetworking changes the access of an environment that has
// internal networking; the API refuses it with a 422 otherwise.
func (c *Client) UpdateInternalNetworking(ctx context.Context, environmentID string, req InternalNetworkingRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/environments/%s/internal-networking", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DisableInternalNetworking(ctx context.Context, environmentID string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/environments/%s/internal-networking", environmentID), nil, nil)
}
