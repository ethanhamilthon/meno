package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/safe"
)

// :login is the one place for providers: accounts that signed in with OAuth,
// providers added with an API key, and the choices to add more of either.

func (a *App) loginKey(k *tcell.EventKey) {
	rows := a.loginRows()
	row := rows[clampSel(a.st.Sel, len(rows))]
	switch {
	case a.navKey(k, len(rows), true):
		a.st.LoginDel = ""
	case k.Key() == tcell.KeyEnter:
		a.loginEnter(row)
	case k.Key() == tcell.KeyRune && k.Rune() == ' ':
		a.loginToggle(row)
	case isKey(k, 'd'):
		a.loginRemove(row)
	}
}

func (a *App) loginEnter(r loginRow) {
	switch r.kind {
	case rowAPI:
		p := a.st.Providers[r.i]
		if p.FromFile() {
			a.fileEntryNotice(config.ProviderFileName)
			return
		}
		a.openProviderEdit(p, r.i)
	case rowOAuth:
		a.notify("space turns the account on or off · d logs it out")
	case rowAddAPI:
		a.openProviderEdit(a.newAPIDraft(r.proto), -1)
	case rowAddOAuth:
		a.startLogin(r.svc)
	case rowHelperRestart:
		a.restartOAuthHelper()
	}
}

// openProviderEdit shows the form of one provider; idx -1 is a new one.
func (a *App) openProviderEdit(p llm.Provider, idx int) {
	a.st.EditProv, a.st.EditIdx = p, idx
	a.openOverlay(OverProviderEdit)
	if err := llm.Validate(p); err != nil {
		a.st.FormNote = err.Error()
	}
}

func (a *App) loginToggle(r loginRow) {
	switch r.kind {
	case rowAPI:
		a.toggleProvider(r.i)
	case rowOAuth:
		acc := a.st.Accounts[r.i]
		a.accountAction("account", func(ctx context.Context) error { return a.oauth.SetDisabled(ctx, acc.File, !acc.Disabled) })
	}
}

// loginRemove deletes a provider or logs an account out; the second press confirms.
func (a *App) loginRemove(r loginRow) {
	var key, name string
	switch r.kind {
	case rowAPI:
		key, name = "api:"+a.st.Providers[r.i].Name, a.st.Providers[r.i].Name
	case rowOAuth:
		key, name = "acc:"+a.st.Accounts[r.i].File, a.st.Accounts[r.i].Label
	default:
		return
	}
	if a.st.LoginDel != key {
		a.st.LoginDel = key
		a.notify("press d again to remove " + name)
		return
	}
	a.st.LoginDel = ""
	if r.kind == rowAPI {
		a.deleteProvider(r.i)
		a.st.Sel = clampSel(a.st.Sel, len(a.loginRows()))
		return
	}
	file := a.st.Accounts[r.i].File
	a.accountAction("log out", func(ctx context.Context) error { return a.oauth.Logout(ctx, file) })
}

// accountAction runs a change to an OAuth account off the UI goroutine and
// then reloads the accounts.
func (a *App) accountAction(what string, fn func(ctx context.Context) error) {
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		err := a.withHelper(ctx, func(ctx context.Context) error { return fn(ctx) })
		if err != nil {
			a.postEvent(errNotice{text: what + ": " + a.accountError(err)})
			return
		}
		a.refreshAccounts()
	}, a.reportPanic)
}
