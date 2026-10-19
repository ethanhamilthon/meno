package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/cliproxy"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/safe"
	"meno/internal/ui"
)

// :login is the one place for providers: accounts that signed in with OAuth,
// providers added with an API key, and the choices to add more of either.

type rowKind int

const (
	rowAPI      rowKind = iota // a provider added by hand: i indexes Providers
	rowOAuth                   // an OAuth account: i indexes Accounts
	rowAddAPI                  // add a provider with an API key; proto says which protocol
	rowAddOAuth                // log in to svc
)

type loginRow struct {
	kind  rowKind
	i     int
	proto string
	svc   cliproxy.Service
}

// loginRows lists what the user has, then everything that can be added.
func (a *App) loginRows() []loginRow {
	var rows []loginRow
	for i, p := range a.st.Providers {
		if !p.Managed() {
			rows = append(rows, loginRow{kind: rowAPI, i: i})
		}
	}
	for i := range a.st.Accounts {
		rows = append(rows, loginRow{kind: rowOAuth, i: i})
	}
	rows = append(rows, loginRow{kind: rowAddAPI, proto: llm.ProtoResponses}, loginRow{kind: rowAddAPI, proto: llm.ProtoChat})
	for _, s := range cliproxy.Services {
		rows = append(rows, loginRow{kind: rowAddOAuth, svc: s})
	}
	return rows
}

func firstAddRow(rows []loginRow) int {
	for i, r := range rows {
		if r.kind == rowAddAPI || r.kind == rowAddOAuth {
			return i
		}
	}
	return 0
}

// loginRowOfProvider is the row of the provider at index i of Providers.
func (a *App) loginRowOfProvider(i int) int {
	for n, r := range a.loginRows() {
		if r.kind == rowAPI && r.i == i {
			return n
		}
	}
	return 0
}

func protoLabel(proto string) string {
	if proto == llm.ProtoResponses {
		return "Responses"
	}
	return "Chat Completions"
}

func (a *App) loginItems() []item {
	var out []item
	for _, r := range a.loginRows() {
		out = append(out, a.loginItem(r))
	}
	return out
}

func (a *App) loginItem(r loginRow) item {
	switch r.kind {
	case rowAPI:
		p := a.st.Providers[r.i]
		g, fg, state := "●", ui.Green, "on"
		switch {
		case p.Disabled:
			g, fg, state = "○", ui.FgDim, "off"
		case !usable(p):
			g, fg = "!", ui.Yellow
			state = "needs a key"
		case a.st.ModelErr[p.Name] != "":
			g, fg = "!", ui.Yellow
			state = "models: " + a.st.ModelErr[p.Name]
		}
		detail := "API key · " + protoLabel(p.Protocol) + " · " + state + " · space on/off · d remove"
		if t := fileTag(p.FromFile(), config.ProviderFileName); t != "" {
			detail = t
		}
		return item{glyph: g, glyphFG: fg, label: p.Name, right: p.BaseURL, detail: detail}
	case rowOAuth:
		acc := a.st.Accounts[r.i]
		g, fg, state := "●", ui.Green, "signed in"
		switch {
		case acc.Disabled:
			g, fg, state = "○", ui.FgDim, "off"
		case acc.Failed > 0:
			g, fg, state = "!", ui.Yellow, itoa(acc.Failed)+" failed requests"
		}
		right := acc.Service
		if acc.Plan != "" {
			right += " · " + acc.Plan
		}
		return item{glyph: g, glyphFG: fg, label: acc.Label, right: right, detail: "OAuth · " + state + " · space on/off · d log out"}
	case rowAddAPI:
		d := "OpenAI and other APIs that speak the Responses protocol"
		if r.proto == llm.ProtoChat {
			d = "OpenRouter, local servers and other chat-completions APIs"
		}
		return item{glyph: "+", glyphFG: ui.Blue, label: "API key · " + protoLabel(r.proto), accent: true, detail: d}
	default:
		d := "sign in with your browser"
		if r.svc.ID == "xai" || r.svc.ID == "meta" || r.svc.ID == "kimi" {
			d = "sign in with a code you confirm in your browser"
		}
		return item{glyph: "+", glyphFG: ui.Blue, label: "Log in · " + r.svc.Label, accent: true, detail: d}
	}
}

func (a *App) drawLogin(c ui.Canvas, w, h int) {
	rows := a.loginRows()
	sub := ""
	switch {
	case a.st.AccountsErr != "" && a.hasManaged():
		sub = "accounts unavailable"
	case len(a.st.Accounts) > 0:
		sub = itoa(len(a.st.Accounts)) + " signed in"
	}
	title := "login"
	if a.st.Locked {
		title = "login · add a provider to start"
	}
	hint := "j/k select · enter add or edit · space on/off · d remove · esc close"
	if a.st.Locked {
		hint = "j/k select · enter add · ctrl+c quits"
	}
	sel := clampSel(a.st.Sel, len(rows))
	listBox(c, w, h, 73, titleRow(title, sub), a.loginItems(), sel, hint)
}

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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			a.postEvent(errNotice{text: what + ": " + err.Error()})
			return
		}
		a.refreshAccounts()
	}, a.reportPanic)
}
