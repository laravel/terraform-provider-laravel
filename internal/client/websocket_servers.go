package client

import (
	"context"
	"fmt"
)

func (c *Client) ListWebsocketServers(ctx context.Context) ([]WebsocketServerData, error) {
	return listAll[WebsocketServerData](ctx, c, "/websocket-servers")
}

func (c *Client) GetWebsocketServer(ctx context.Context, id string) (*WebsocketServerData, error) {
	var doc Document[WebsocketServerData]
	if err := c.do(ctx, "GET", fmt.Sprintf("/websocket-servers/%s", id), nil, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) CreateWebsocketServer(ctx context.Context, req CreateWebsocketServerRequest) (*WebsocketServerData, error) {
	var doc Document[WebsocketServerData]
	if err := c.do(ctx, "POST", "/websocket-servers", req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) UpdateWebsocketServer(ctx context.Context, id string, req UpdateWebsocketServerRequest) (*WebsocketServerData, error) {
	var doc Document[WebsocketServerData]
	if err := c.do(ctx, "PATCH", fmt.Sprintf("/websocket-servers/%s", id), req, &doc); err != nil {
		return nil, err
	}
	return &doc.Data, nil
}

func (c *Client) DeleteWebsocketServer(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/websocket-servers/%s", id), nil, nil)
}

// ListWebsocketServerTypes returns the WebSocket server types with the
// regions and connection tiers each supports.
func (c *Client) ListWebsocketServerTypes(ctx context.Context) ([]WebsocketServerType, error) {
	var resp struct {
		Data []WebsocketServerType `json:"data"`
	}
	if err := c.do(ctx, "GET", "/websocket-servers/types", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
