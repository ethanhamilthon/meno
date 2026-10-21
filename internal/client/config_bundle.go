package client

import (
	"context"

	"meno/internal/proto"
)

// Config reads the daemon configuration in one request.
func (c *Client) Config(ctx context.Context) (proto.ConfigBundle, error) {
	raw, err := c.configCall(ctx, "GET", "/v1/config", nil)
	if err != nil {
		return proto.ConfigBundle{}, err
	}
	return configDecode[proto.ConfigBundle](raw)
}
