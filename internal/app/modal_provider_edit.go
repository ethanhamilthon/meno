package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
	"meno/internal/ui"
)

func (a *App) providerEditKey(k *tcell.EventKey) {
	fs := a.providerFields()
	a.formKey(k, fs, a.saveProviderEdit)
	if isKey(k, 'd') && !a.st.Editing && a.st.EditIdx >= 0 {
		a.deleteProvider(a.st.EditIdx)
		a.openOverlay(OverLogin)
		a.st.Sel = clampSel(0, len(a.loginRows()))
	}
}

// saveProviderEdit stores the provider as soon as it is valid; until then the
// form says what is missing. Every later change is saved at once.
func (a *App) saveProviderEdit() {
	p := a.st.EditProv
	if err := llm.Validate(p); err != nil {
		a.st.FormNote = err.Error()
		return
	}
	if i := a.providerNamed(p.Name); i >= 0 && i != a.st.EditIdx {
		a.st.FormNote = "a provider named " + p.Name + " exists"
		return
	}
	a.st.FormNote = ""
	if a.st.EditIdx >= 0 && a.st.EditIdx < len(a.st.Providers) {
		a.st.Providers[a.st.EditIdx] = p
	} else {
		a.st.Providers = append(a.st.Providers, p)
		a.st.EditIdx = len(a.st.Providers) - 1
	}
	a.persistProviders()
	a.afterProvidersChanged()
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
	b.put(c, 2, 0, "login /", ui.FgDim, ui.BgPopup)
	b.put(c, 14, 0, a.st.EditProv.Name, ui.Blue, ui.BgPopup)
	status := "● connected"
	fg := ui.Green
	if a.st.EditProv.APIKey == "" {
		status, fg = "○ no key", ui.FgDim
	}
	b.putRight(c, 0, status, fg, ui.BgPopup)
	drawForm(c, b, 2, a.providerFields(), a.st.Field, a.st.Editing, &a.st.Edit)
	drawFormNote(c, b, a.st.FormNote)
	b.hint(c, "j/k move · enter edit · d delete · esc back")
}
