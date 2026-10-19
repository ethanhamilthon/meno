package app

import (
	"context"
	"sync"
	"time"

	"meno/internal/cliproxy"
	"meno/internal/llm"
)

// The OAuth side of :login. The helper owns OAuth accounts; meno builds one
// temporary provider from enabled accounts and never saves it.
type loginParts struct {
	oauth          *cliproxy.Manager
	oauthUpdates   bool
	oauthRestartMu sync.Mutex
	oauthRestarts  []time.Time
	flowSeq        int
	openURL        func(string) error
	helperReadyFn  func(context.Context) bool
	helperStart    func(context.Context) error
	helperChecked  bool
	helperRunning  bool
}

func (a *App) onLoginEvent(msg any) {
	switch m := msg.(type) {
	case accountsLoaded:
		a.onAccountsLoaded(m)
	case oauthPortChanged:
		a.refreshManagedProvider()
	case oauthHelperStatus:
		a.helperChecked, a.helperRunning = true, m.ready
	case oauthRefreshed:
		a.onOAuthRefreshed(m)
	case flowStep:
		a.onFlowStep(m)
	case flowLogin:
		a.onFlowLogin(m)
	case flowDone:
		a.onFlowDone(m)
	}
}

func (a *App) managedProvider() llm.Provider {
	return llm.Provider{Name: llm.OAuthName, BaseURL: a.oauth.APIURL(), APIKey: a.oauth.APIKey(), Protocol: llm.ProtoChat, Run: a.oauth.Command(), Source: llm.SourceOAuth}
}

func (a *App) setManaged(want bool) {
	out := make([]llm.Provider, 0, len(a.st.Providers)+1)
	for _, p := range a.st.Providers {
		if !p.Managed() {
			out = append(out, p)
		}
	}
	if want {
		out = append(out, a.managedProvider())
	}
	a.st.Providers = out
}

func (a *App) hasManaged() bool {
	for _, p := range a.st.Providers {
		if p.Managed() {
			return true
		}
	}
	return false
}
