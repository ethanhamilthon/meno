package engine

import (
	"context"
	"encoding/json"
	"net/url"

	"meno/internal/api"
)

// modelResource lists the models of the enabled providers for `meno models`.
// It serves what the config service serves to the TUI: the catalog plus the
// models a provider reports on /models, with the per-provider errors. Without
// refresh=true the cached list is used; when that list is empty the providers
// are asked once, so a provider without explicit model settings still lists.
func (e *Engine) modelResource(ctx context.Context, query string) (api.ModelsResponse, error) {
	if e.resources == nil {
		return api.ModelsResponse{}, dapi(api.CodeNotFound, "resource is not implemented")
	}
	v, _ := url.ParseQuery(query)
	res, err := e.listModels(ctx, v)
	if err != nil {
		return res, err
	}
	if len(res.Models) == 0 && v.Get("refresh") != "true" {
		v.Set("refresh", "true")
		return e.listModels(ctx, v)
	}
	return res, nil
}

func (e *Engine) listModels(ctx context.Context, v url.Values) (api.ModelsResponse, error) {
	var out api.ModelsResponse
	r, err := e.resources.Resource(ctx, "GET", "/v1/models", v.Encode(), api.ResourceRequest{})
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(r.Data, &out); err != nil {
		return out, err
	}
	if out.Models == nil {
		out.Models = []api.ConfigModel{}
	}
	return out, nil
}
