package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Edge network rules live in one ruleset per type (firewall, rate_limit,
// cache) of a dedicated edge network. The ruleset is a hyphenated slug in
// URLs ("rate-limit") but an underscored type in rule bodies ("rate_limit").

func edgeNetworkRulesPath(edgeNetworkID, ruleset string) string {
	return fmt.Sprintf("/edge-networks/%s/rulesets/%s/rules", edgeNetworkID, strings.ReplaceAll(ruleset, "_", "-"))
}

func (c *Client) GetEdgeNetworkRule(ctx context.Context, edgeNetworkID, ruleset, id string) (*EdgeNetworkRuleData, error) {
	var doc Document[EdgeNetworkRuleData]
	if err := c.do(ctx, "GET", edgeNetworkRulesPath(edgeNetworkID, ruleset)+"/"+id, nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateEdgeNetworkRule(ctx context.Context, edgeNetworkID, ruleset string, req CreateEdgeNetworkRuleRequest) (*EdgeNetworkRuleData, error) {
	var doc Document[EdgeNetworkRuleData]
	if err := c.do(ctx, "POST", edgeNetworkRulesPath(edgeNetworkID, ruleset), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

// UpdateEdgeNetworkRule changes only the fields the request sets.
func (c *Client) UpdateEdgeNetworkRule(ctx context.Context, edgeNetworkID, ruleset, id string, req UpdateEdgeNetworkRuleRequest) (*EdgeNetworkRuleData, error) {
	var doc Document[EdgeNetworkRuleData]
	if err := c.do(ctx, "PATCH", edgeNetworkRulesPath(edgeNetworkID, ruleset)+"/"+id, req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteEdgeNetworkRule(ctx context.Context, edgeNetworkID, ruleset, id string) error {
	return c.do(ctx, "DELETE", edgeNetworkRulesPath(edgeNetworkID, ruleset)+"/"+id, nil, nil)
}

// ListEdgeNetworkRules returns a ruleset's rules in the order they are
// evaluated.
func (c *Client) ListEdgeNetworkRules(ctx context.Context, edgeNetworkID, ruleset string) ([]EdgeNetworkRuleData, error) {
	return listAll[EdgeNetworkRuleData](ctx, c, edgeNetworkRulesPath(edgeNetworkID, ruleset))
}

// IsRulesetBusy returns true if the API refused a rule change because another
// change to the same ruleset was in progress. The API waits five seconds for
// the ruleset before it answers 409; the same request succeeds once the other
// change finishes.
func IsRulesetBusy(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}

type EdgeNetworkRuleData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type"`
	Attributes EdgeNetworkRuleAttributes `json:"attributes"`
}

type EdgeNetworkRuleAttributes struct {
	// Type is the ruleset, such as "firewall" or "rate_limit".
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IsActive    bool    `json:"is_active"`
	// Settings are the rule's action and conditions; their keys depend on
	// the ruleset. The API returns unset keys as null and fills in some
	// defaults, such as a rate limit's counting_key.
	Settings  json.RawMessage `json:"settings"`
	CreatedAt *string         `json:"created_at"`
}

// EdgeNetworkRulePosition places a rule among the ruleset's rules: exactly one
// of Before or After (another rule's ID) or Index (1 is first).
type EdgeNetworkRulePosition struct {
	Before *string `json:"before,omitempty"`
	After  *string `json:"after,omitempty"`
	Index  *int64  `json:"index,omitempty"`
}

// CreateEdgeNetworkRuleRequest adds a rule, last unless Position is set. A new
// rule is always active.
type CreateEdgeNetworkRuleRequest struct {
	Name        string                   `json:"name"`
	Description *string                  `json:"description,omitempty"`
	Settings    json.RawMessage          `json:"settings"`
	Position    *EdgeNetworkRulePosition `json:"position,omitempty"`
}

// UpdateEdgeNetworkRuleRequest changes the fields it sets. Description is
// always sent, so null clears it; Settings replace the current ones entirely.
type UpdateEdgeNetworkRuleRequest struct {
	Name        *string                  `json:"name,omitempty"`
	Description *string                  `json:"description"`
	IsActive    *bool                    `json:"is_active,omitempty"`
	Settings    json.RawMessage          `json:"settings,omitempty"`
	Position    *EdgeNetworkRulePosition `json:"position,omitempty"`
}
