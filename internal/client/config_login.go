package client

import (
	"context"
	"net/url"

	"meno/internal/proto"
)

// LoginStarts starts or polls an OAuth login and manages accounts in the daemon.
func (c *Client) LoginHelper(ctx context.Context) (proto.OAuthHelperResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/helper", nil)
	if e != nil {
		return proto.OAuthHelperResponse{}, e
	}
	return configDecode[proto.OAuthHelperResponse](raw)
}

func (c *Client) LoginServices(ctx context.Context) (proto.LoginServicesResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/services", nil)
	if e != nil {
		return proto.LoginServicesResponse{}, e
	}
	return configDecode[proto.LoginServicesResponse](raw)
}
func (c *Client) LoginStart(ctx context.Context, service string) (proto.LoginStartResponse, error) {
	raw, e := c.configCall(ctx, "POST", "/v1/login/start", proto.LoginStartRequest{Service: service})
	if e != nil {
		return proto.LoginStartResponse{}, e
	}
	return configDecode[proto.LoginStartResponse](raw)
}
func (c *Client) LoginStatus(ctx context.Context, state string) (proto.LoginStatusResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/status?state="+url.QueryEscape(state), nil)
	if e != nil {
		return proto.LoginStatusResponse{}, e
	}
	return configDecode[proto.LoginStatusResponse](raw)
}
func (c *Client) LoginCallback(ctx context.Context, redirect string) error {
	_, e := c.configCall(ctx, "POST", "/v1/login/callback", proto.LoginCallbackRequest{RedirectURL: redirect})
	return e
}
func (c *Client) LoginAccounts(ctx context.Context) (proto.AccountsResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/accounts", nil)
	if e != nil {
		return proto.AccountsResponse{}, e
	}
	return configDecode[proto.AccountsResponse](raw)
}
func (c *Client) SetLoginAccountEnabled(ctx context.Context, file string, enabled bool) error {
	_, e := c.configCall(ctx, "PUT", "/v1/login/accounts/"+pathID(file), proto.AccountEnabledRequest{Enabled: enabled})
	return e
}
func (c *Client) RemoveLoginAccount(ctx context.Context, file string) error {
	_, e := c.configCall(ctx, "DELETE", "/v1/login/accounts/"+pathID(file), nil)
	return e
}
func (c *Client) RestartLoginHelper(ctx context.Context) error {
	_, e := c.configCall(ctx, "POST", "/v1/login/restart", struct{}{})
	return e
}
