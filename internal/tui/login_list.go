package tui

import (
	"strings"

	"meno/internal/llm"
	"meno/internal/render"
)

type rowKind int

const (
	rowAPI           rowKind = iota // a provider added by hand: i indexes Providers
	rowOAuth                        // an OAuth account: i indexes Accounts
	rowAddAPI                       // add a provider with an API key; proto says which protocol
	rowAddOAuth                     // log in to svc
	rowHelperRestart                // restart the local OAuth helper
)

type loginRow struct {
	kind  rowKind
	i     int
	proto string
	svc   loginService
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
	for _, s := range loginServices {
		rows = append(rows, loginRow{kind: rowAddOAuth, svc: s})
	}
	if len(a.st.Accounts) > 0 || a.st.AccountsErr != "" {
		rows = append(rows, loginRow{kind: rowHelperRestart})
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
		g, fg, state := "●", render.Green, "on"
		switch {
		case p.Disabled:
			g, fg, state = "○", render.FgDim, "off"
		case !usable(p):
			g, fg = "!", render.Yellow
			state = "needs a key"
		case a.st.ModelErr[p.Name] != "":
			g, fg = "!", render.Yellow
			state = "models: " + a.st.ModelErr[p.Name]
		}
		detail := "API key · " + protoLabel(p.Protocol) + " · " + state + " · space on/off · d remove"
		return item{glyph: g, glyphFG: fg, label: p.Name, right: p.BaseURL, detail: detail}
	case rowOAuth:
		acc := a.st.Accounts[r.i]
		g, fg, state := "●", render.Green, "signed in"
		switch {
		case acc.Disabled:
			g, fg, state = "○", render.FgDim, "off"
		case acc.Failed > 0:
			g, fg, state = "!", render.Yellow, itoa(acc.Failed)+" failed requests"
		}
		right := acc.Service
		if acc.Plan != "" {
			right += " · " + acc.Plan
		}
		if acc.Note != "" {
			state += " · needs a new login"
		}
		return item{glyph: g, glyphFG: fg, label: acc.Label, right: right, detail: "OAuth · " + state + " · space on/off · d log out"}
	case rowHelperRestart:
		status := "restart it when logins stop working"
		if a.st.AccountsErr != "" {
			status = "helper is not responding · " + a.st.AccountsErr
		}
		return item{glyph: "↻", glyphFG: render.Blue, label: "Login helper: restart", accent: true, detail: status}
	case rowAddAPI:
		d := "OpenAI and other APIs that speak the Responses protocol"
		if r.proto == llm.ProtoChat {
			d = "OpenRouter, local servers and other chat-completions APIs"
		}
		return item{glyph: "+", glyphFG: render.Blue, label: "API key · " + protoLabel(r.proto), accent: true, detail: d}
	default:
		d := "sign in with your browser"
		if r.svc.ID == "xai" || r.svc.ID == "meta" || r.svc.ID == "kimi" {
			d = "sign in with a code you confirm in your browser"
		}
		return item{glyph: "+", glyphFG: render.Blue, label: "Log in · " + r.svc.Label, accent: true, detail: d}
	}
}

func (a *App) drawLogin(c render.Canvas, w, h int) {
	rows := a.loginRows()
	sub := ""
	switch {
	case a.st.AccountsErr != "":
		if strings.Contains(strings.ToLower(a.st.AccountsErr), "not responding") || strings.Contains(strings.ToLower(a.st.AccountsErr), "connection refused") {
			sub = "helper not responding"
		} else {
			sub = "accounts unavailable"
		}
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
