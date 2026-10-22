package client

import (
	"context"
	"net/url"

	"meno/internal/api"
)

// LoginStarts starts or polls an OAuth login and manages accounts in the daemon.
func (c *Client) LoginHelper(ctx context.Context) (api.OAuthHelperResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/helper", nil)
	if e != nil {
		return api.OAuthHelperResponse{}, e
	}
	return configDecode[api.OAuthHelperResponse](raw)
}

func (c *Client) LoginServices(ctx context.Context) (api.LoginServicesResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/services", nil)
	if e != nil {
		return api.LoginServicesResponse{}, e
	}
	return configDecode[api.LoginServicesResponse](raw)
}
func (c *Client) LoginStart(ctx context.Context, service string) (api.LoginStartResponse, error) {
	raw, e := c.configCall(ctx, "POST", "/v1/login/start", api.LoginStartRequest{Service: service})
	if e != nil {
		return api.LoginStartResponse{}, e
	}
	return configDecode[api.LoginStartResponse](raw)
}
func (c *Client) LoginStatus(ctx context.Context, state string) (api.LoginStatusResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/status?state="+url.QueryEscape(state), nil)
	if e != nil {
		return api.LoginStatusResponse{}, e
	}
	return configDecode[api.LoginStatusResponse](raw)
}
func (c *Client) LoginCallback(ctx context.Context, redirect string) error {
	_, e := c.configCall(ctx, "POST", "/v1/login/callback", api.LoginCallbackRequest{RedirectURL: redirect})
	return e
}
func (c *Client) LoginAccounts(ctx context.Context) (api.AccountsResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/login/accounts", nil)
	if e != nil {
		return api.AccountsResponse{}, e
	}
	return configDecode[api.AccountsResponse](raw)
}
func (c *Client) SetLoginAccountEnabled(ctx context.Context, file string, enabled bool) error {
	_, e := c.configCall(ctx, "PUT", "/v1/login/accounts/"+pathID(file), api.AccountEnabledRequest{Enabled: enabled})
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
