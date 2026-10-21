package client

import (
	"context"
	"encoding/json"

	"meno/internal/proto"
)

func (c *Client) configCall(ctx context.Context, method, path string, data any) (json.RawMessage, error) {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	v, err := c.Resource(ctx, method, path, "", proto.ResourceRequest{Data: raw})
	if err != nil {
		return nil, err
	}
	return v.Data, nil
}
func configDecode[T any](raw json.RawMessage) (T, error) {
	var out T
	err := json.Unmarshal(raw, &out)
	return out, err
}

// Settings reads and writes daemon settings.
func (c *Client) Settings(ctx context.Context) (proto.SettingsResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/settings", nil)
	if e != nil {
		return proto.SettingsResponse{}, e
	}
	return configDecode[proto.SettingsResponse](raw)
}
func (c *Client) PutSettings(ctx context.Context, v proto.ConfigSettings) (proto.SettingsResponse, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/settings", proto.SettingsRequest{Settings: v})
	if e != nil {
		return proto.SettingsResponse{}, e
	}
	return configDecode[proto.SettingsResponse](raw)
}

// PatchSettings changes only the named fields. A field whose stored value no
// longer equals the one in expect is rejected as a conflict.
func (c *Client) PatchSettings(ctx context.Context, fields, expect map[string]any) (proto.SettingsResponse, error) {
	enc := func(in map[string]any) (map[string]json.RawMessage, error) {
		out := map[string]json.RawMessage{}
		for k, v := range in {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			out[k] = b
		}
		return out, nil
	}
	f, err := enc(fields)
	if err != nil {
		return proto.SettingsResponse{}, err
	}
	x, err := enc(expect)
	if err != nil {
		return proto.SettingsResponse{}, err
	}
	raw, e := c.configCall(ctx, "PATCH", "/v1/settings", proto.SettingsPatch{Fields: f, Expect: x})
	if e != nil {
		return proto.SettingsResponse{}, e
	}
	return configDecode[proto.SettingsResponse](raw)
}

// Providers reads all configured providers, including read-only overlays.
func (c *Client) Providers(ctx context.Context) (proto.ProvidersResponse, error) {
	raw, e := c.configCall(ctx, "GET", "/v1/providers", nil)
	if e != nil {
		return proto.ProvidersResponse{}, e
	}
	return configDecode[proto.ProvidersResponse](raw)
}
func (c *Client) CreateProvider(ctx context.Context, v proto.ConfigProvider) (proto.ProviderResponse, error) {
	raw, e := c.configCall(ctx, "POST", "/v1/providers", proto.ProviderRequest{Provider: v})
	if e != nil {
		return proto.ProviderResponse{}, e
	}
	return configDecode[proto.ProviderResponse](raw)
}
func (c *Client) UpdateProvider(ctx context.Context, name string, v proto.ConfigProvider) (proto.ProviderResponse, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/providers/"+pathID(name), proto.ProviderRequest{Provider: v})
	if e != nil {
		return proto.ProviderResponse{}, e
	}
	return configDecode[proto.ProviderResponse](raw)
}
func (c *Client) DeleteProvider(ctx context.Context, name string) error {
	_, e := c.configCall(ctx, "DELETE", "/v1/providers/"+pathID(name), nil)
	return e
}
func (c *Client) SetProviderEnabled(ctx context.Context, name string, enabled bool) (proto.ProviderResponse, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/providers/"+pathID(name)+"/enabled", proto.ProviderEnabledRequest{Enabled: enabled})
	if e != nil {
		return proto.ProviderResponse{}, e
	}
	return configDecode[proto.ProviderResponse](raw)
}
func (c *Client) SetProviderModels(ctx context.Context, name string, models map[string]proto.ConfigModelSettings) (proto.ProviderResponse, error) {
	raw, e := c.configCall(ctx, "PUT", "/v1/providers/"+pathID(name)+"/models", proto.ProviderModelsRequest{Models: models})
	if e != nil {
		return proto.ProviderResponse{}, e
	}
	return configDecode[proto.ProviderResponse](raw)
}
