package app

import (
	"context"
	"sort"

	"meno/internal/client"
	"meno/internal/llm"
	"meno/internal/proto"
)

// fetchModels asks the daemon to refresh each active provider. Only the daemon
// has credentials and makes provider HTTP requests.
func (a *App) fetchModels() {
	for _, p := range a.st.Providers {
		if !p.Active() || a.st.Loading[p.Name] {
			continue
		}
		name := p.Name
		a.st.Loading[name] = true
		a.spawn(func() {
			var out proto.ModelsResponse
			err := a.rpc(func(ctx context.Context, c *client.Client) error {
				var e error
				out, e = c.Models(ctx, proto.ModelsRequest{Provider: name, Refresh: true})
				return e
			})
			a.postEvent(modelsLoaded{provider: name, response: out, err: err})
		})
	}
}

func (a *App) onModelsLoaded(m modelsLoaded) {
	delete(a.st.Loading, m.provider)
	active := false
	for _, p := range a.st.Providers {
		if p.Name == m.provider && p.Active() {
			active = true
			break
		}
	}
	if !active {
		return
	}
	if m.err != nil {
		a.st.ModelErr[m.provider] = m.err.Error()
		return // Keep the last list when the daemon cannot be reached.
	}
	models := make([]llm.Model, 0, len(m.response.Models))
	for _, item := range m.response.Models {
		if item.Provider == m.provider {
			models = append(models, llm.Model{Provider: item.Provider, ID: item.ID})
		}
	}
	a.st.Models[m.provider] = models
	if a.st.Pricing == nil {
		a.st.Pricing = map[string]llm.ModelInfo{}
	}
	for key, info := range infoFromModels(m.response.Models) {
		a.st.Pricing[key] = info
	}
	if message := m.response.Errors[m.provider]; message != "" {
		a.st.ModelErr[m.provider] = message
	} else {
		delete(a.st.ModelErr, m.provider)
	}
}

// modelsOf returns the daemon's list without reconstructing catalog IDs.
func (a *App) modelsOf(p llm.Provider) []llm.Model {
	models := append([]llm.Model(nil), a.st.Models[p.Name]...)
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func (a *App) allModels() []llm.Model {
	var out []llm.Model
	for _, p := range a.st.Providers {
		if p.Active() {
			out = append(out, a.modelsOf(p)...)
		}
	}
	return out
}
