package app

import (
	"fmt"
	"strings"

	"meno/internal/llm"
	"meno/internal/proto"
	"meno/internal/ui"
)

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

// infoFromModels turns the daemon's model list into the catalog the status
// bar prices with.
func infoFromModels(ms []proto.ConfigModel) map[string]llm.ModelInfo {
	out := map[string]llm.ModelInfo{}
	for _, m := range ms {
		info := llm.ModelInfo{Vision: m.Images}
		if m.ContextWindow != nil {
			info.Context = *m.ContextWindow
		}
		if m.Prices != nil {
			info.Input, info.Output, info.CacheRead, info.CacheWrite = m.Prices.Input, m.Prices.Output, m.Prices.CacheRead, m.Prices.CacheWrite
		} else {
			info.MissingPricing = true
		}
		out[m.Provider+"/"+m.ID] = info
		out["custom:"+strings.ToLower(m.Provider)+"/"+m.ID] = info
		if _, ok := out[m.ID]; !ok {
			out[m.ID] = info
		}
	}
	return out
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
