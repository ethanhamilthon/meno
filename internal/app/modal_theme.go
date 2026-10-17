package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// themeIdx are the themes matching the search query.
func (a *App) themeIdx() []ui.Theme {
	var out []ui.Theme
	for _, t := range ui.Themes() {
		if containsFold(t.Name, string(a.st.Query)) {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) openTheme() {
	a.openOverlay(OverTheme)
	for i, t := range a.themeIdx() {
		if t.Name == ui.CurrentTheme() {
			a.st.Sel = i
		}
	}
}

// previewTheme shows the highlighted theme right away.
func (a *App) previewTheme() {
	if list := a.themeIdx(); len(list) > 0 {
		ui.SetTheme(list[clampSel(a.st.Sel, len(list))].Name)
	}
}

// cancelTheme puts back the theme that was saved before the preview.
func (a *App) cancelTheme() {
	if !ui.SetTheme(a.st.Settings.Theme) {
		ui.SetTheme(ui.DefaultTheme)
	}
}

func (a *App) themeKey(k *tcell.EventKey) {
	list := a.themeIdx()
	switch {
	case a.navKey(k, len(list), false):
	case k.Key() == tcell.KeyEnter:
		if len(list) > 0 {
			a.st.Settings.Theme = list[clampSel(a.st.Sel, len(list))].Name
			ui.SetTheme(a.st.Settings.Theme)
			a.persistSettings()
			a.notify("theme: " + a.st.Settings.Theme)
		}
		a.closeOverlay()
		return
	default:
		a.queryKey(k)
	}
	a.previewTheme()
}

func (a *App) drawTheme(c ui.Canvas, w, h int) {
	list := a.themeIdx()
	items := make([]item, 0, len(list))
	for _, t := range list {
		kind := "light"
		if t.Dark {
			kind = "dark"
		}
		g, gfg := "○", ui.FgDim
		if t.Name == a.st.Settings.Theme {
			g, gfg = "●", t.Blue
		}
		items = append(items, item{glyph: g, glyphFG: gfg, label: t.Name, right: kind})
	}
	a.searchList(c, w, h, 52, len(ui.Themes()), "theme", "search themes", items, "↑/↓ preview · enter apply · esc cancel")
}
