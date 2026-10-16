package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/ui"
)

func (a *App) promptsKey(k *tcell.EventKey) {
	n := len(a.st.Prompts)
	switch {
	case a.navKey(k, n+1, true):
	case k.Key() == tcell.KeyEnter:
		a.openPromptEdit(a.st.Sel)
	case k.Rune() == 'n':
		a.openPromptEdit(-1)
	case k.Rune() == 'd' && a.st.Sel < n:
		a.st.Prompts = append(a.st.Prompts[:a.st.Sel], a.st.Prompts[a.st.Sel+1:]...)
		a.persistPrompts()
		a.st.Sel = clampSel(a.st.Sel, len(a.st.Prompts)+1)
	}
}

func (a *App) openPromptEdit(i int) {
	if i < 0 || i >= len(a.st.Prompts) {
		a.st.EditPrompt = config.Prompt{Scope: "project"}
		a.st.EditIdx = -1
	} else {
		a.st.EditPrompt = a.st.Prompts[i]
		a.st.EditIdx = i
	}
	a.openOverlay(OverPromptEdit)
}

func (a *App) drawPrompts(c ui.Canvas, w, h int) {
	var items []item
	for _, p := range a.st.Prompts {
		items = append(items, item{label: "#" + p.Name, right: p.Scope, detail: p.Template})
	}
	items = append(items, item{glyph: "+", glyphFG: ui.Blue, label: "new prompt", accent: true})
	listBox(c, w, h, 620, titleRow("prompts", "type # in input to autocomplete"), items, a.st.Sel,
		"j/k select · enter edit · n new · d delete · esc close")
}

func (a *App) promptFields() []field {
	p := &a.st.EditPrompt
	return []field{
		{label: "name", get: func() string { return p.Name }, set: func(v string) error { p.Name = v; return nil }},
		{label: "scope", opts: []string{"project", "global"}, get: func() string { return p.Scope }, set: func(v string) error { p.Scope = v; return nil }},
		{label: "template", get: func() string { return p.Template }, set: func(v string) error { p.Template = v; return nil }},
	}
}

func (a *App) promptEditKey(k *tcell.EventKey) {
	a.formKey(k, a.promptFields())
	switch {
	case k.Key() == tcell.KeyCtrlS:
		a.savePromptEdit()
	case k.Rune() == 'd' && !a.st.Editing && a.st.EditIdx >= 0:
		a.st.Prompts = append(a.st.Prompts[:a.st.EditIdx], a.st.Prompts[a.st.EditIdx+1:]...)
		a.persistPrompts()
		a.openOverlay(OverPrompts)
	}
}

func (a *App) savePromptEdit() {
	p := a.st.EditPrompt
	if err := config.ValidatePrompt(p); err != nil {
		a.st.Toast = err.Error()
		return
	}
	for j, o := range a.st.Prompts {
		if j != a.st.EditIdx && o.Name == p.Name {
			a.st.Toast = "prompt #" + p.Name + " already exists"
			return
		}
	}
	if a.st.EditIdx >= 0 && a.st.EditIdx < len(a.st.Prompts) {
		a.st.Prompts[a.st.EditIdx] = p
	} else {
		a.st.Prompts = append(a.st.Prompts, p)
		a.st.EditIdx = len(a.st.Prompts) - 1
	}
	a.persistPrompts()
	a.st.Toast = "prompt saved"
}

func (a *App) drawPromptEdit(c ui.Canvas, w, h int) {
	fs := a.promptFields()
	b := centered(w, h, 620, len(fs)+6)
	b.frame(c)
	b.put(c, 2, 0, "prompts /", ui.FgDim, ui.BgPopup)
	name := a.st.EditPrompt.Name
	if name == "" {
		name = "new prompt"
	}
	b.put(c, 13, 0, name, ui.Blue, ui.BgPopup)
	drawForm(c, b, 2, fs, a.st.Field, a.st.Editing, string(a.st.Buffer))
	b.hint(c, "j/k move · enter edit · ctrl+s save · d delete · esc back")
}
