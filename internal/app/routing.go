package app

import (
	"context"
	"os"
	"strings"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/routing"
)

type effortRouteConfig struct {
	settings  config.Settings
	client    func(llm.Provider) llm.Client
	providers []llm.Provider
	modes     []string
	cwd       string
	allowed   []string
	sticky    *routing.Sticky
}

// routeEffort picks one request level without changing the session selection.
func routeEffort(ctx context.Context, msg agent.Message, provider, model, manual string, cfg effortRouteConfig) (string, string, *routing.Sticky) {
	if cfg.settings.EffortRouter != "auto" {
		return manual, "", nil
	}
	sticky := &routing.Sticky{Current: manual}
	if cfg.sticky != nil {
		*sticky = *cfg.sticky
	}
	routerProviderName := cfg.settings.EffortRouterProvider
	if routerProviderName == "" {
		routerProviderName = provider
	}
	routerModel := cfg.settings.EffortRouterModel
	if routerModel == "" {
		routerModel = cfg.settings.SubagentModel
	}
	if routerModel == "" {
		routerModel = model
	}
	var routerProvider llm.Provider
	for _, p := range cfg.providers {
		if p.Name == routerProviderName && p.Active() {
			routerProvider = p
			break
		}
	}
	if routerProvider.Name == "" || routerModel == "" {
		return manual, routerFailureNotice(manual), &routing.Sticky{Current: manual}
	}
	files := countProjectEntries(cfg.cwd)
	router := routing.ModelRouter{Client: cfg.client(routerProvider), Provider: routerProvider, Model: routerModel}
	verdict, err := router.Route(ctx, routing.Request{
		Context: routing.Context{Text: msg.Text, Modes: append([]string(nil), cfg.modes...), Attachments: len(msg.Attachments), Files: files},
		Current: sticky.Current,
		Allowed: append([]string(nil), cfg.allowed...),
	})
	if err != nil || !validRoutedEffort(verdict.Level, cfg.allowed) {
		return manual, routerFailureNotice(manual), &routing.Sticky{Current: manual}
	}
	old := sticky.Current
	level, why := sticky.Decide(verdict)
	if why != "" && level != old {
		return level, "effort routing: " + old + " → " + level + " (" + why + ")", sticky
	}
	return level, "", sticky
}

func routerFailureNotice(manual string) string {
	return "effort routing: using " + manual + " (the router did not answer)"
}

func validRoutedEffort(level string, allowed []string) bool {
	if level == "" {
		return true
	}
	for _, option := range allowed {
		if level == option {
			return true
		}
	}
	return false
}

// countProjectEntries is a shallow directory read. Dot-directories are skipped.
func countProjectEntries(cwd string) int {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		count++
	}
	return count
}
