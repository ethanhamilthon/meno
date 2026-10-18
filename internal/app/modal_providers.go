package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/ui"
)

func (a *App) providersItems() []item {
	var out []item
	for _, p := range a.st.Providers {
		dot := ui.FgDim
		g := "○"
		if p.APIKey != "" {
			g = "●"
			dot = ui.Green
		}
		if p.Name == "local" {
			g, dot = "●", ui.Green
		}
		out = append(out, item{glyph: g, glyphFG: dot, label: p.Name, accent: false, right: p.BaseURL, detail: fileTag(p.FromFile(), config.ProviderFileName)})
	}
	out = append(out, item{glyph: "+", glyphFG: ui.Blue, label: "new provider", accent: true})
	return out
}

func (a *App) providersKey(k *tcell.EventKey) {
	items := a.providersItems()
	switch {
	case a.navKey(k, len(items), true):
	case k.Key() == tcell.KeyEnter && a.st.Sel < len(a.st.Providers) && a.st.Providers[a.st.Sel].FromFile():
		a.fileEntryNotice(config.ProviderFileName)
	case k.Key() == tcell.KeyEnter:
		if a.st.Sel == len(a.st.Providers) {
			a.st.EditProv = llm.Provider{Protocol: llm.ProtoChat}
			a.st.EditIdx = -1
		} else {
			a.st.EditProv = a.st.Providers[a.st.Sel]
			a.st.EditIdx = a.st.Sel
		}
		a.openOverlay(OverProviderEdit)
	case isKey(k, 'n'):
		a.st.EditProv = llm.Provider{Protocol: llm.ProtoChat}
		a.st.EditIdx = -1
		a.openOverlay(OverProviderEdit)
	case isKey(k, 'd') && a.st.Sel < len(a.st.Providers) && a.st.Providers[a.st.Sel].FromFile():
		a.fileEntryNotice(config.ProviderFileName)
	case isKey(k, 'd') && a.st.Sel < len(a.st.Providers):
		a.daemons.Stop(a.st.Providers[a.st.Sel].Name)
		a.st.Providers = append(a.st.Providers[:a.st.Sel], a.st.Providers[a.st.Sel+1:]...)
		a.persistProviders()
		a.st.Sel = clampSel(a.st.Sel, len(a.st.Providers))
	}
}

func (a *App) drawProviders(c ui.Canvas, w, h int) {
	items := a.providersItems()
	header := titleRow("providers", "")
	listBox(c, w, h, 73, header, items, a.st.Sel, "j/k select · enter edit · n new · d delete · esc close")
}

func (a *App) providerEditKey(k *tcell.EventKey) {
	fs := a.providerFields()
	a.formKey(k, fs)
	if k.Key() == tcell.KeyCtrlS {
		a.saveProviderEdit()
	}
	if isKey(k, 'd') && !a.st.Editing && a.st.EditIdx >= 0 {
		a.daemons.Stop(a.st.Providers[a.st.EditIdx].Name)
		a.st.Providers = append(a.st.Providers[:a.st.EditIdx], a.st.Providers[a.st.EditIdx+1:]...)
		a.persistProviders()
		a.openOverlay(OverProviders)
	}
}

func (a *App) saveProviderEdit() {
	p := a.st.EditProv
	if err := llm.Validate(p); err != nil {
		a.notifyErr(err.Error())
		return
	}
	if a.st.EditIdx >= 0 && a.st.EditIdx < len(a.st.Providers) {
		if old := a.st.Providers[a.st.EditIdx].Name; old != p.Name {
			a.daemons.Stop(old) // renamed: the daemon is keyed by the name
		}
		a.st.Providers[a.st.EditIdx] = p
	} else {
		a.st.Providers = append(a.st.Providers, p)
	}
	a.persistProviders()
	a.syncProviderRun(p)
	a.notify("provider saved")
}

func (a *App) providerFields() []field {
	p := &a.st.EditProv
	return []field{
		{label: "name", get: func() string { return p.Name }, set: func(s string) error { p.Name = s; return nil }},
		{label: "base url", get: func() string { return p.BaseURL }, set: func(s string) error { p.BaseURL = s; return nil }},
		{label: "api key", secret: true, get: func() string { return p.APIKey }, set: func(s string) error { p.APIKey = s; return nil }},
		{label: "run", get: func() string { return p.Run }, set: func(s string) error { p.Run = strings.TrimSpace(s); return nil }},
		{label: "protocol", opts: []string{llm.ProtoResponses, llm.ProtoChat}, get: func() string { return p.Protocol }, set: func(s string) error { p.Protocol = s; return nil }},
	}
}

func (a *App) drawProviderEdit(c ui.Canvas, w, h int) {
	b := centered(w, h, 73, 11)
	b.frame(c)
	b.put(c, 2, 0, "providers /", ui.FgDim, ui.BgPopup)
	b.put(c, 14, 0, a.st.EditProv.Name, ui.Blue, ui.BgPopup)
	status := "● connected"
	fg := ui.Green
	if a.st.EditProv.APIKey == "" {
		status, fg = "○ no key", ui.FgDim
	}
	b.putRight(c, 0, status, fg, ui.BgPopup)
	drawForm(c, b, 2, a.providerFields(), a.st.Field, a.st.Editing, string(a.st.Buffer))
	b.hint(c, "j/k move · enter edit · ctrl+s save · d delete · esc back")
}
