package client

import (
	"context"
	"fmt"
)

func (c *Client) ListWebsocketServers(ctx context.Context) ([]WebsocketServerData, error) {
	return listAll[WebsocketServerData](ctx, c, "/websocket-servers")
}

func (c *Client) GetWebsocketServer(ctx context.Context, id string) (*WebsocketServerData, error) {
	return fetch[WebsocketServerData](ctx, c, "GET", fmt.Sprintf("/websocket-servers/%s", id), nil)
}

func (c *Client) CreateWebsocketServer(ctx context.Context, req CreateWebsocketServerRequest) (*WebsocketServerData, error) {
	return fetch[WebsocketServerData](ctx, c, "POST", "/websocket-servers", req)
}

func (c *Client) UpdateWebsocketServer(ctx context.Context, id string, req UpdateWebsocketServerRequest) (*WebsocketServerData, error) {
	return fetch[WebsocketServerData](ctx, c, "PATCH", fmt.Sprintf("/websocket-servers/%s", id), req)
}

func (c *Client) DeleteWebsocketServer(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/websocket-servers/%s", id), nil, nil)
}
