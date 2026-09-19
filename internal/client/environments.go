package client

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) ListEnvironments(ctx context.Context, applicationID string) ([]EnvironmentData, error) {
	return listAll[EnvironmentData](ctx, c, fmt.Sprintf("/applications/%s/environments", applicationID))
}

// GetEnvironment fetches a single environment.
//
// include=application,branch is required: the API omits the relationships
// block entirely unless it is asked for, and neither the owning application
// id nor the branch is otherwise recoverable on import. `branch` is not an
// attribute at all -- it is a relationship whose identifier carries an id, so
// the human-readable name has to come from the included branch object.
func (c *Client) GetEnvironment(ctx context.Context, id string) (*EnvironmentData, error) {
	var doc Document[EnvironmentData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/environments/%s?include=application,branch", id), nil, &doc); err != nil {
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
	return fetch[EnvironmentData](ctx, c, "POST", fmt.Sprintf("/applications/%s/environments", applicationID), req)
}

func (c *Client) UpdateEnvironment(ctx context.Context, id string, req UpdateEnvironmentRequest) (*EnvironmentData, error) {
	return fetch[EnvironmentData](ctx, c, "PATCH", fmt.Sprintf("/environments/%s", id), req)
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
	return fetch[EnvironmentData](ctx, c, "PUT", fmt.Sprintf("/environments/%s/vanity-domain", environmentID), req)
}
