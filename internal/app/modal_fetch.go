package app

import (
	"context"
	"time"

	"meno/internal/llm"
)

// fetchModels loads /models for every configured provider in the background.
// Results arrive as modelsLoaded events so the UI never blocks on the network.
func (a *App) fetchModels() {
	for _, p := range a.st.Providers {
		if a.st.Loading[p.Name] {
			continue
		}
		a.st.Loading[p.Name] = true
		delete(a.st.ModelErr, p.Name)
		go func(p llm.Provider) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ms, err := llm.FetchModels(ctx, p)
			a.events <- modelsLoaded{provider: p.Name, models: ms, err: err}
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

// allModels returns the models of every provider, in provider order.
func (a *App) allModels() []llm.Model {
	var out []llm.Model
	for _, p := range a.st.Providers {
		out = append(out, a.st.Models[p.Name]...)
	}
	return out
}
