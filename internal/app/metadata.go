package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"meno/internal/llm"
	"meno/internal/safe"
	"meno/internal/ui"
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
		defer safe.Recover(a.reportPanic)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		go func() {
			defer safe.Recover(a.reportPanic)
			select {
			case <-a.done:
				cancel()
			case <-ctx.Done():
			}
		}()
		m, err := llm.FetchMetadata(ctx)
		if err == nil {
			_ = a.store.SaveCatalog(m)
		}
		a.postEvent(metadataLoaded{m, err})
	}()
}
func (a *App) loadMetadataCache() {
	if m, err := a.store.LoadCatalog(); err == nil && len(m) > 0 {
		a.st.Pricing = m
	}
}
func modelOf(a *App) string {
	_, m, _ := a.selection()
	return m
}

func (a *App) modelInfo() (llm.ModelInfo, bool) {
	p, ok := a.currentProvider()
	if !ok {
		return llm.ModelInfo{}, false
	}
	return p.Lookup(a.st.Pricing, modelOf(a))
}

// statusUsageText is the session line of the status bar, with icons instead of
// words: tokens in/out, cache hit of the last request/of the session, the
// context in use/window, and the spend. Everything in it is about the session:
// it does not change while the agent works.
func (a *App) statusUsageText() string { return a.statusUsage(1 << 30) }

// statusUsage is the same line cut to max columns: fields leave from the right
// (spend first, then the context, then the cache) until it fits.
func (a *App) statusUsage(max int) string {
	context := compactTokens(a.st.Context) + "/?"
	if info, ok := a.modelInfo(); ok && info.Context > 0 {
		context = compactTokens(a.st.Context) + "/" + compactTokens(info.Context)
	}
	cost := fmtSpend(a.st.Spent)
	if a.st.Unpriced {
		cost += "+?"
	}
	fields := []string{
		compactTokens(a.st.Usage.In) + "/" + compactTokens(a.st.Usage.Out),
		"↺" + cacheShare(a.st.LastUsage) + "/" + cacheShare(a.st.Usage),
		"◫ " + context,
		cost,
	}
	for len(fields) > 1 && ui.TextWidth(strings.Join(fields, " ")) > max {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

// cacheShare is the share of input tokens served from the cache, "?" when the
// provider does not report it.
func cacheShare(u llm.Usage) string {
	if !u.CacheReported {
		return "?"
	}
	if u.In <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(min(max(0, u.Cached), u.In))/float64(u.In))
}

// fmtSpend prints a price with at most four digits: $0.001, $0.012, $1.234,
// $12.34, $123.4, without trailing zeros ($1.25, not $1.250). A price too small to show is "<$0.001"; no spend is "$0".
func fmtSpend(v float64) string {
	switch {
	case v <= 0:
		return "$0"
	case v < 0.0005:
		return "<$0.001"
	}
	intDigits := 1
	for x := v; x >= 10; x /= 10 {
		intDigits++
	}
	s := fmt.Sprintf("%.*f", max(0, 4-intDigits), v)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".") // $1.250 is $1.25
	}
	return "$" + s
}
