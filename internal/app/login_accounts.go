package app

import (
	"context"
	"time"

	"meno/internal/cliproxy"
	"meno/internal/llm"
	"meno/internal/safe"
)

func (a *App) enabledAccounts() int {
	n := 0
	for _, acc := range a.st.Accounts {
		if !acc.Disabled {
			n++
		}
	}
	return n
}

// refreshAccounts repairs the helper when needed, then lists accounts off the UI goroutine.
func (a *App) refreshAccounts() {
	if a.oauth == nil || !a.oauth.Installed() {
		return
	}
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		var list []cliproxy.Account
		err := a.withHelper(ctx, func(ctx context.Context) error {
			var e error
			list, e = a.oauth.Accounts(ctx)
			return e
		})
		a.postEvent(accountsLoaded{list: list, err: err})
	}, a.reportPanic)
}

func (a *App) onAccountsLoaded(m accountsLoaded) {
	if m.err != nil {
		a.st.AccountsErr = a.accountError(m.err)
		return
	}
	a.st.AccountsErr, a.st.Accounts = "", m.list
	enabled := a.enabledAccounts() > 0
	a.oauth.Update(func(s *cliproxy.State) {
		s.Accounts, s.AccountsKnown, s.HasAccounts = len(m.list), true, enabled
	})
	a.setManaged(enabled)
	if len(m.list) == 0 && a.daemons != nil {
		a.daemons.Stop(llm.OAuthName)
	}
	if len(m.list) > 0 {
		a.syncManagedProvider()
	}
	a.st.Sel = clampSel(a.st.Sel, len(a.loginRows()))
	a.fetchModels()
	a.afterProvidersChanged()
}
