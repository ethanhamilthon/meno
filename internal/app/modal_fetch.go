package app

import (
	"context"
	"sort"
	"time"

	"meno/internal/llm"
	"meno/internal/safe"
)

// fetchModels loads /models for every configured provider in the background.
// Results arrive as modelsLoaded events so the UI never blocks on the network.
func (a *App) fetchModels() {
	for _, p := range a.st.Providers {
		if !p.Active() || a.st.Loading[p.Name] {
			continue
		}
		a.st.Loading[p.Name] = true
		delete(a.st.ModelErr, p.Name)
		go func(p llm.Provider) {
			defer safe.Recover(a.reportPanic)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ms, err := llm.FetchModels(ctx, p)
			a.postEvent(modelsLoaded{provider: p.Name, models: ms, err: err})
		}(p)
	}
}

func (a *App) onModelsLoaded(m modelsLoaded) {
	delete(a.st.Loading, m.provider)
	if m.err != nil {
		a.st.ModelErr[m.provider] = m.err.Error()
		a.st.Models[m.provider] = nil
		return
	}
	a.st.Models[m.provider] = m.models
	delete(a.st.ModelErr, m.provider)
}

// modelsOf merges the cached catalog entries of a provider with live discovery.
func (a *App) modelsOf(p llm.Provider) []llm.Model {
	models := llm.CatalogModels(a.st.Pricing, p)
	seen := make(map[string]bool, len(models))
	for _, m := range models {
		seen[m.ID] = true
	}
	for _, m := range a.st.Models[p.Name] {
		if !seen[m.ID] {
			models = append(models, m)
			seen[m.ID] = true
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

// allModels is modelsOf for every provider, in provider order.
func (a *App) allModels() []llm.Model {
	var out []llm.Model
	for _, p := range a.st.Providers {
		if !p.Active() {
			continue
		}
		out = append(out, a.modelsOf(p)...)
	}
	return out
}
