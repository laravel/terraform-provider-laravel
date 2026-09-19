package client

import (
	"context"
	"fmt"
)

func (c *Client) ListWebsocketApplications(ctx context.Context, serverID string) ([]WebsocketApplicationData, error) {
	return listAll[WebsocketApplicationData](ctx, c, fmt.Sprintf("/websocket-servers/%s/applications", serverID))
}

// Only list and create are nested under the server. Every per-item route is
// flat -- the nested forms are not registered on the API: the GET 404s and the
// PATCH/DELETE fall through to the web catch-all and answer 302 to the app
// root. Taking the id alone also lets `terraform import` work, since an
// imported application has no server_id in state yet.
func (c *Client) GetWebsocketApplication(ctx context.Context, id string) (*WebsocketApplicationData, error) {
	return fetch[WebsocketApplicationData](ctx, c, "GET", fmt.Sprintf("/websocket-applications/%s", id), nil)
}

func (c *Client) CreateWebsocketApplication(ctx context.Context, serverID string, req CreateWebsocketApplicationRequest) (*WebsocketApplicationData, error) {
	return fetch[WebsocketApplicationData](ctx, c, "POST", fmt.Sprintf("/websocket-servers/%s/applications", serverID), req)
}

func (c *Client) UpdateWebsocketApplication(ctx context.Context, id string, req UpdateWebsocketApplicationRequest) (*WebsocketApplicationData, error) {
	return fetch[WebsocketApplicationData](ctx, c, "PATCH", fmt.Sprintf("/websocket-applications/%s", id), req)
}

func (c *Client) DeleteWebsocketApplication(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/websocket-applications/%s", id), nil, nil)
}
