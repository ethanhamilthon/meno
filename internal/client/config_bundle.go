package client

import (
	"context"

	"meno/internal/api"
)

// Config reads the daemon configuration in one request.
func (c *Client) Config(ctx context.Context) (api.ConfigBundle, error) {
	raw, err := c.configCall(ctx, "GET", "/v1/config", nil)
	if err != nil {
		return api.ConfigBundle{}, err
	}
	return configDecode[api.ConfigBundle](raw)
}
