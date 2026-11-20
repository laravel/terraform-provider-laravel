package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) ListDeployments(ctx context.Context, environmentID string) ([]DeploymentData, error) {
	return listAll[DeploymentData](ctx, c, fmt.Sprintf("/environments/%s/deployments", environmentID))
}

func (c *Client) GetDeployment(ctx context.Context, id string) (*DeploymentData, error) {
	var doc Document[DeploymentData]
	// include=environment is required to recover the parent id on import.
	if err := c.do(ctx, "GET", fmt.Sprintf("/deployments/%s?include=environment", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// CreateDeployment requests a deployment of the environment. While the
// environment's resources are still provisioning, Cloud does not start one: it
// answers 202 with a queued deployment instead, returned here as queued with a
// nil deployment. GET /deployments/{id} does not know a queued deployment's id.
func (c *Client) CreateDeployment(ctx context.Context, environmentID string) (deployment *DeploymentData, queued *QueuedDeploymentData, err error) {
	var doc Document[json.RawMessage]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/deployments", environmentID), nil, &doc); err != nil {
		return nil, nil, err
	}
	return decodeDeployment(doc.Data)
}

// decodeDeployment decodes a response that holds either a deployment or a
// queued deployment.
func decodeDeployment(data json.RawMessage) (deployment *DeploymentData, queued *QueuedDeploymentData, err error) {
	var head ResourceIdentifier
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, nil, fmt.Errorf("decoding deployment: %w", err)
	}
	if head.Type == QueuedDeploymentType {
		queued = &QueuedDeploymentData{}
		err = json.Unmarshal(data, queued)
	} else {
		deployment = &DeploymentData{}
		err = json.Unmarshal(data, deployment)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("decoding deployment: %w", err)
	}
	return deployment, queued, nil
}

// GetQueuedDeployment reads a queued deployment. Once Cloud has promoted it,
// its deployment relationship is the deployment it started.
func (c *Client) GetQueuedDeployment(ctx context.Context, id string) (*QueuedDeploymentData, error) {
	var doc Document[QueuedDeploymentData]
	// Relationships are only reported when included.
	if err := c.do(ctx, "GET", fmt.Sprintf("/queued-deployments/%s?include=environment,deployment", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}
