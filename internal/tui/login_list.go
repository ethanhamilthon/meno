package tui

import (
	"meno/internal/llm"
	"meno/internal/render"
)

type rowKind int

const (
	rowAPI    rowKind = iota // a provider added by hand: i indexes Providers
	rowAddAPI                // add a provider with an API key; proto says which protocol
)

type loginRow struct {
	kind  rowKind
	i     int
	proto string
}

// loginRows lists what the user has, then everything that can be added.
func (a *App) loginRows() []loginRow {
	var rows []loginRow
	for i := range a.st.Providers {
		rows = append(rows, loginRow{kind: rowAPI, i: i})
	}
	rows = append(rows, loginRow{kind: rowAddAPI, proto: llm.ProtoResponses}, loginRow{kind: rowAddAPI, proto: llm.ProtoChat})
	return rows
}

func firstAddRow(rows []loginRow) int {
	for i, r := range rows {
		if r.kind == rowAddAPI {
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
	case rowAddAPI:
		d := "OpenAI and other APIs that speak the Responses protocol"
		if r.proto == llm.ProtoChat {
			d = "OpenRouter, local servers and other chat-completions APIs"
		}
		return item{glyph: "+", glyphFG: render.Blue, label: "API key · " + protoLabel(r.proto), accent: true, detail: d}
	}
	return item{}
}

func (a *App) drawLogin(c render.Canvas, w, h int) {
	rows := a.loginRows()
	title := "login"
	if a.st.Locked {
		title = "login · add a provider to start"
	}
	hint := "j/k select · enter add or edit · space on/off · d remove · esc close"
	if a.st.Locked {
		hint = "j/k select · enter add · ctrl+c quits"
	}
	sel := clampSel(a.st.Sel, len(rows))
	listBox(c, w, h, 73, titleRow(title, ""), a.loginItems(), sel, hint)
}
