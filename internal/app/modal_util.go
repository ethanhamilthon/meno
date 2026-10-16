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

func (a *App) searchList(c ui.Canvas, w, h, bw, total int, items []item, hint string) {
	header := searchHeader(
		func() string { return string(a.st.Query) },
		func() string { return itoa(len(items)) + " / " + itoa(total) },
	)
	listBox(c, w, h, bw, header, items, clampSel(a.st.Sel, len(items)), hint)
}
