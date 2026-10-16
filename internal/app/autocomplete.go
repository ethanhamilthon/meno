package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

const acMax = 5

// acPrefix returns the partial prompt name after a leading '#' in the composer,
// and whether the composer holds a single '#word' token.
func (a *App) acPrefix() (string, bool) {
	text := string(a.st.Input)
	if !strings.HasPrefix(text, "#") || strings.ContainsAny(text, " \n") {
		return "", false
	}
	return text[1:], true
}

func (a *App) acMatches() []int {
	q, ok := a.acPrefix()
	if !ok {
		return nil
	}
	return filterIdx(len(a.st.Prompts), func(i int) bool {
		return strings.HasPrefix(a.st.Prompts[i].Name, q)
	})
}

func (a *App) acActive() bool {
	return a.st.Mode == ui.ModeInsert && len(a.acMatches()) > 0
}

// acKey handles keys while the autocomplete popup is open. It returns true if consumed.
func (a *App) acKey(k *tcell.EventKey) bool {
	m := a.acMatches()
	n := min(len(m), acMax)
	switch k.Key() {
	case tcell.KeyDown:
		a.st.Sel = clampSel(a.st.Sel+1, n)
		return true
	case tcell.KeyUp:
		a.st.Sel = clampSel(a.st.Sel-1, n)
		return true
	case tcell.KeyTab, tcell.KeyEnter:
		if a.st.Sel < n {
			a.st.Input = []rune("#" + a.st.Prompts[m[a.st.Sel]].Name + " ")
			a.st.Sel = 0
		}
		return true
	}
	return false
}

func (a *App) acHeight() int {
	if !a.acActive() {
		return 0
	}
	return min(len(a.acMatches()), acMax)
}

func (a *App) drawAutocomplete(c ui.Canvas, x, y, w int) {
	m := a.acMatches()
	n := min(len(m), acMax)
	sel := clampSel(a.st.Sel, n)
	for i := 0; i < n; i++ {
		p := a.st.Prompts[m[i]]
		bg, tagFG, mark := ui.BgPopup, ui.FgSoft, " "
		if i == sel {
			bg, tagFG, mark = ui.BgPopupSel, ui.Blue, "▸"
		}
		c.Fill(x, y+i, w, 1, bg)
		c.Text(x+2, y+i, 2, mark, tcell.StyleDefault.Foreground(ui.Blue).Background(bg))
		c.Text(x+4, y+i, 22, "#"+p.Name, tcell.StyleDefault.Foreground(tagFG).Background(bg))
		c.Text(x+26, y+i, w-28, p.Template, tcell.StyleDefault.Foreground(ui.FgDim).Background(bg))
	}
}
