package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// ListApplications returns the organization's applications. filters maps
// "name", "region" and "slug" to a value each must contain; the API matches
// them as substrings and accepts no others. include=defaultEnvironment is
// asked for, as in GetApplication, so that each application reports its
// default environment relationship.
func (c *Client) ListApplications(ctx context.Context, filters map[string]string) ([]ApplicationData, error) {
	query := filterQuery(filters)
	if query == "" {
		query = "?include=defaultEnvironment"
	} else {
		query += "&include=defaultEnvironment"
	}
	return listAll[ApplicationData](ctx, c, "/applications"+query)
}

// GetApplication fetches a single application. include=defaultEnvironment is
// required: the API omits the relationships block unless it is asked for.
func (c *Client) GetApplication(ctx context.Context, id string) (*ApplicationData, error) {
	var doc Document[ApplicationData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/applications/%s?include=defaultEnvironment", id), nil, &doc); err != nil {
		return nil, err
	}
	app := doc.Data
	app.DefaultEnvironmentClusterID = includedEnvironmentClusterID(doc.Included, app.Relationships.DefaultEnvironment.RelatedID())
	return &app, nil
}

// includedEnvironmentClusterID returns the cluster_id of an environment in a
// document's included section: nil when it was not included, or when it runs
// on shared compute.
func includedEnvironmentClusterID(included []ResourceObject, id string) *string {
	if id == "" {
		return nil
	}
	for _, obj := range included {
		if obj.Type != "environments" || obj.ID != id {
			continue
		}
		var attrs struct {
			ClusterID *string `json:"cluster_id"`
		}
		if err := json.Unmarshal(obj.Attributes, &attrs); err != nil {
			return nil
		}
		return attrs.ClusterID
	}
	return nil
}

func (c *Client) CreateApplication(ctx context.Context, req CreateApplicationRequest) (*ApplicationData, error) {
	var doc Document[ApplicationData]
	if err := c.do(ctx, "POST", "/applications", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateApplication(ctx context.Context, id string, req UpdateApplicationRequest) (*ApplicationData, error) {
	var doc Document[ApplicationData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/applications/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/applications/%s", id), nil, nil)
}
