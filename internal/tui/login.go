package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// /login manages API-key and custom providers.

func (a *App) openLogin() { a.openOverlay(OverLogin) }

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
		a.openProviderEdit(p, r.i)
	case rowAddAPI:
		a.openProviderEdit(a.newAPIDraft(r.proto), -1)
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
	}
}

// loginRemove deletes a provider; the second press confirms.
func (a *App) loginRemove(r loginRow) {
	var key, name string
	switch r.kind {
	case rowAPI:
		key, name = "api:"+a.st.Providers[r.i].Name, a.st.Providers[r.i].Name
	default:
		return
	}
	if a.st.LoginDel != key {
		a.st.LoginDel = key
		a.notify("press d again to remove " + name)
		return
	}
	a.st.LoginDel = ""
	a.deleteProvider(r.i)
	a.st.Sel = clampSel(a.st.Sel, len(a.loginRows()))
}
