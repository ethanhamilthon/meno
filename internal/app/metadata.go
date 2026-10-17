package app

import (
	"context"
	"fmt"
	"meno/internal/llm"
	"meno/internal/store"
	"time"
)

type metadataLoaded struct {
	pricing map[string]llm.ModelInfo
	err     error
}

func (a *App) postEvent(e any) {
	select {
	case <-a.done:
		return
	default:
	}
	select {
	case a.events <- e:
	case <-a.done:
	}
}
func (a *App) fetchMetadata() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		go func() {
			select {
			case <-a.done:
				cancel()
			case <-ctx.Done():
			}
		}()
		m, err := llm.FetchMetadata(ctx)
		if err == nil {
			_ = store.SaveJSON(a.paths.File("model-catalog.json"), m)
		}
		a.postEvent(metadataLoaded{m, err})
	}()
}
func (a *App) loadMetadataCache() {
	var m map[string]llm.ModelInfo
	if err := store.LoadJSON(a.paths.File("model-catalog.json"), &m); err == nil {
		a.st.Pricing = m
	}
}
func (a *App) modelInfo() (llm.ModelInfo, bool) {
	p, ok := a.currentProvider()
	if !ok {
		return llm.ModelInfo{}, false
	}
	return llm.LookupMetadata(a.st.Pricing, p, a.st.Settings.Model)
}

// sessionCacheHit is the share of input tokens served from the cache over all
// requests of the session, in percent.
func (a *App) sessionCacheHit() float64 {
	u := a.st.Usage
	if u.In <= 0 {
		return 0
	}
	return 100 * float64(min(max(0, u.Cached), u.In)) / float64(u.In)
}

// statusUsageText deliberately uses icons rather than prose labels. Token totals
// and spend are session-wide, and so is the cache hit rate.
func (a *App) statusUsageText() string {
	cache := a.sessionCacheHit()
	context := compactTokens(a.st.Context) + "/?"
	if info, ok := a.modelInfo(); ok && info.Context > 0 {
		context = compactTokens(a.st.Context) + "/" + compactTokens(info.Context)
	}
	cost := fmt.Sprintf("$%.4f", a.st.Spent)
	if a.st.Unpriced {
		cost += "+?"
	}
	return fmt.Sprintf("↓ %s · ↑ %s · ↺ %.0f%% · ◫ %s · %s", compactTokens(a.st.Usage.In), compactTokens(a.st.Usage.Out), cache, context, cost)
}
