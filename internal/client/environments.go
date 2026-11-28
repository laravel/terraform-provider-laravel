package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// ListEnvironments returns an application's environments. filters maps
// "name", "slug" and "status" to a value each must contain; the API matches
// them as substrings and accepts no others.
func (c *Client) ListEnvironments(ctx context.Context, applicationID string, filters map[string]string) ([]EnvironmentData, error) {
	return listAll[EnvironmentData](ctx, c, fmt.Sprintf("/applications/%s/environments", applicationID)+filterQuery(filters))
}

// GetEnvironment fetches a single environment.
//
// include=application,branch is required: the API omits the relationships
// block entirely unless it is asked for, and neither the owning application
// id nor the branch is otherwise recoverable on import. `branch` is not an
// attribute at all -- it is a relationship whose identifier carries an id, so
// the human-readable name has to come from the included branch object. The
// attached database, cache and WebSocket application are likewise reported
// only as included relationships.
func (c *Client) GetEnvironment(ctx context.Context, id string) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/environments/%s?include=application,branch,database,cache,websocketApplication", id), nil, &doc); err != nil {
		return nil, err
	}

	env := doc.Data
	env.BranchName = resolveIncludedName(doc.Included, "branches", env.Relationships.Branch.RelatedID())
	return &env, nil
}

// resolveIncludedName looks up a related object in a document's `included`
// section and returns its `name` attribute. It returns "" when the id is
// empty, the object was not included, or it carries no name -- callers treat
// that as "not reported" and leave the existing value alone.
func resolveIncludedName(included []ResourceObject, typeName, id string) string {
	if id == "" {
		return ""
	}
	for _, obj := range included {
		if obj.Type != typeName || obj.ID != id {
			continue
		}
		var attrs struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(obj.Attributes, &attrs); err != nil {
			return ""
		}
		return attrs.Name
	}
	return ""
}

func (c *Client) CreateEnvironment(ctx context.Context, applicationID string, req CreateEnvironmentRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/applications/%s/environments", applicationID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, id string, req UpdateEnvironmentRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/environments/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// StartEnvironment starts a stopped environment by deploying it. The
// environment reports itself running only once that deployment succeeds.
// Like CreateDeployment, the deployment may come back queued.
func (c *Client) StartEnvironment(ctx context.Context, id string) (deployment *DeploymentData, queued *QueuedDeploymentData, err error) {
	var doc Document[json.RawMessage]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/start", id), nil, &doc); err != nil {
		return nil, nil, err
	}
	return decodeDeployment(doc.Data)
}

// StopEnvironment stops an environment, cancelling a deployment in progress.
// The API refuses it for an environment that has never been deployed.
func (c *Client) StopEnvironment(ctx context.Context, id string) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "POST", fmt.Sprintf("/environments/%s/stop", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/environments/%s", id), nil, nil)
}

// UpdateVanityDomainRequest is the body of PUT /environments/{id}/vanity-domain.
type UpdateVanityDomainRequest struct {
	Name string `json:"name"`
}

// SetVanityDomain renames the environment's vanity domain.
//
// This is a dedicated PUT route rather than a field on the environment PATCH
// body, which is why it is a separate call.
func (c *Client) SetVanityDomain(ctx context.Context, environmentID string, req UpdateVanityDomainRequest) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "PUT", fmt.Sprintf("/environments/%s/vanity-domain", environmentID), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}
