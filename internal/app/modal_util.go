package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func filterIdx(n int, match func(i int) bool) []int {
	var out []int
	for i := 0; i < n; i++ {
		if match(i) {
			out = append(out, i)
		}
	}
	return out
}

func containsFold(s, q string) bool {
	return q == "" || strings.Contains(strings.ToLower(s), strings.ToLower(q))
}

func (a *App) queryKey(k *tcell.EventKey) {
	switch k.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.st.Query); n > 0 {
			a.st.Query = a.st.Query[:n-1]
		}
	case tcell.KeyRune:
		a.st.Query = append(a.st.Query, k.Rune())
	default:
		return
	}
	a.st.Sel = 0
}

func (a *App) searchList(c ui.Canvas, w, h, bw, total int, title, placeholder string, items []item, hint string) {
	a.searchListEmpty(c, w, h, bw, total, title, placeholder, items, hint, "")
}

// searchListEmpty is searchList with its own text for the case of no items.
func (a *App) searchListEmpty(c ui.Canvas, w, h, bw, total int, title, placeholder string, items []item, hint, empty string) {
	sel := clampSel(a.st.Sel, len(items))
	bh := 6 + itemsHeight(items, sel)
	if len(items) == 0 {
		bh++
	}
	b := centered(w, h, bw, bh)
	b.frame(c)
	titledSearch(c, b, title, itoa(len(items))+" / "+itoa(total), string(a.st.Query), placeholder)
	if len(items) == 0 {
		if empty == "" {
			empty = "nothing here"
		}
		b.put(c, 4, 4, ui.Truncate(empty, max(0, b.w-8)), ui.FgDim, ui.BgPopup)
	}
	drawItems(c, b, 4, items, sel)
	b.hint(c, hint)
}
