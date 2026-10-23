package configsvc

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"meno/internal/api"
	"meno/internal/config"
	"meno/internal/llm"
)

func (s *Service) providerResource(ctx context.Context, method, path, rawQuery string, req api.ResourceRequest) (any, error) {
	parts := splitResource(strings.TrimPrefix(path, "/providers"))
	if len(parts) == 0 {
		if method == "GET" {
			return s.providersResponse()
		}
		if method == "POST" {
			request, err := decode[api.ProviderRequest](req.Data)
			if err != nil {
				return nil, bad(err)
			}
			p := request.Provider
			p.Source = ""
			ps, err := s.store.LoadProviders()
			if err != nil {
				return nil, err
			}
			configured, e := s.providers()
			if e != nil {
				return nil, e
			}
			if providerExists(configured, p.Name) {
				return nil, conflict("provider already exists: " + p.Name)
			}

			if err = llm.Validate(providerValue(p)); err != nil {
				return nil, bad(err)
			}
			ps = append(ps, providerValue(p))
			if err = s.store.SaveProviders(config.OwnProviders(ps)); err != nil {
				return nil, err
			}
			s.mu.Lock()
			delete(s.knownModels, p.Name)
			delete(s.modelErrors, p.Name)
			s.mu.Unlock()
			s.syncProvider(ps[len(ps)-1])
			s.changed("providers_changed")
			return api.ProviderResponse{Provider: providerWire(ps[len(ps)-1])}, nil
		}
		return nil, apiErr(api.CodeNotFound, "unsupported provider route")
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, bad(err)
	}
	ps, err := s.providers()
	if err != nil {
		return nil, err
	}
	idx := providerIndex(ps, name)
	if idx < 0 {
		return nil, notFound("no provider named " + name)
	}
	p := ps[idx]
	if p.Managed() {
		return nil, apiErr(api.CodeForbidden, "provider is managed and read only")
	}
	if len(parts) == 1 {
		switch method {
		case "GET":
			return api.ProviderResponse{Provider: providerWire(p)}, nil
		case "PUT":
			request, e := decode[api.ProviderRequest](req.Data)
			if e != nil {
				return nil, bad(e)
			}
			v := request.Provider
			v.Source = ""
			if v.Name != name {
				return nil, bad(fmt.Errorf("provider name cannot change"))
			}
			own, e := s.store.LoadProviders()
			if e != nil {
				return nil, e
			}
			i := providerIndex(own, name)
			if i < 0 {
				return nil, notFound("no provider named " + name)
			}
			if v.ClearAPIKey {
				v.APIKey = ""
			} else if v.APIKey == "" {
				v.APIKey = own[i].APIKey
			}
			if e = llm.Validate(providerValue(v)); e != nil {
				return nil, bad(e)
			}
			own[i] = providerValue(v)
			if e = s.store.SaveProviders(own); e != nil {
				return nil, e
			}
			s.mu.Lock()
			delete(s.knownModels, name)
			delete(s.modelErrors, name)
			s.mu.Unlock()
			s.syncProvider(own[i])
			s.changed("providers_changed")
			return api.ProviderResponse{Provider: providerWire(own[i])}, nil
		case "DELETE":
			own, e := s.store.LoadProviders()
			if e != nil {
				return nil, e
			}
			i := providerIndex(own, name)
			if i < 0 {
				return nil, notFound("no provider named " + name)
			}
			own = append(own[:i], own[i+1:]...)
			if e = s.store.SaveProviders(own); e != nil {
				return nil, e
			}
			s.daemons.Stop(name)
			s.changed("providers_changed")
			return api.AckResponse{OK: true}, nil
		}
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "enabled":
			if method != "PUT" && method != "PATCH" {
				break
			}
			r, e := decode[api.ProviderEnabledRequest](req.Data)
			if e != nil {
				return nil, bad(e)
			}
			own, e := s.store.LoadProviders()
			if e != nil {
				return nil, e
			}
			i := providerIndex(own, name)
			if i < 0 {
				return nil, notFound("no provider named " + name)
			}
			own[i].Disabled = !r.Enabled
			if e = s.store.SaveProviders(own); e != nil {
				return nil, e
			}
			s.mu.Lock()
			delete(s.knownModels, name)
			delete(s.modelErrors, name)
			s.mu.Unlock()
			s.syncProvider(own[i])
			s.changed("providers_changed")
			return api.ProviderResponse{Provider: providerWire(own[i])}, nil
		case "models":
			if method != "PUT" {
				break
			}
			r, e := decode[api.ProviderModelsRequest](req.Data)
			if e != nil {
				return nil, bad(e)
			}
			own, e := s.store.LoadProviders()
			if e != nil {
				return nil, e
			}
			i := providerIndex(own, name)
			if i < 0 {
				return nil, notFound("no provider named " + name)
			}
			own[i].Models = modelSettingsValue(r.Models)
			if e = llm.ValidateModels(own[i].Models); e != nil {
				return nil, bad(e)
			}
			if e = s.store.SaveProviders(own); e != nil {
				return nil, e
			}
			s.mu.Lock()
			delete(s.knownModels, name)
			delete(s.modelErrors, name)
			s.mu.Unlock()
			s.changed("providers_changed")
			return api.ProviderResponse{Provider: providerWire(own[i])}, nil
		}
	}
	return nil, apiErr(api.CodeNotFound, "unsupported provider route")
}

func providerExists(ps []llm.Provider, name string) bool { return providerIndex(ps, name) >= 0 }
