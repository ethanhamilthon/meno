package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

func (a *App) loginFlowKey(k *tcell.EventKey) {
	f := &a.st.Flow
	if a.st.Editing { // pasting the address the browser ended on
		switch {
		case k.Key() == tcell.KeyEsc:
			a.endEdit()
		case k.Key() == tcell.KeyEnter && f.Login != nil:
			a.submitCallback(a.st.Edit.String())
			a.endEdit()
		default:
			a.st.Edit.Handle(k)
		}
		return
	}
	switch {
	case f.Err != "" && (k.Key() == tcell.KeyEnter || isKey(k, 'r')):
		a.startLogin(f.Svc)
	case f.Login != nil && isKey(k, 'o'):
		if err := a.openURL(f.Login.URL); err != nil {
			a.notifyErr(err.Error())
		}
	case f.Login != nil && isKey(k, 'y'):
		a.scr.SetClipboard([]byte(f.Login.URL))
		a.notify("address copied")
	case f.Login != nil && !f.Login.Device && isKey(k, 'p'):
		a.beginEdit("", false, false)
		a.st.Edit.Placeholder = "paste the address of the page the browser ended on"
	}
}

func (a *App) drawLoginFlow(c render.Canvas, w, h int) {
	f := &a.st.Flow
	url := ""
	if f.Login != nil {
		url = f.Login.URL
	}
	urlRows := wrapRunes(url, max(8, modalWidth(w)-8), 4)
	bh := 8 + len(urlRows) + min(len(f.Steps), 3)
	b := centered(w, h, 73, bh)
	b.frame(c)
	titleRow("login · "+f.Svc.Label, "")(c, b)
	y := 2
	for _, s := range f.Steps[max(0, len(f.Steps)-3):] {
		b.put(c, 2, y, render.Truncate(s, max(1, b.w-6)), render.FgDim, render.BgPopup)
		y++
	}
	hint := "esc cancel"
	switch {
	case f.Err != "":
		b.put(c, 2, y+1, render.Truncate(f.Err, max(1, b.w-6)), render.Red, render.BgPopup)
		hint = "enter try again · esc back"
	case f.Login != nil && f.Login.Device:
		b.put(c, 2, y+1, "Open the page and confirm this code:", render.FgWhite, render.BgPopup)
		b.put(c, 2, y+2, f.Login.UserCode, render.Green, render.BgPopup)
		y += 3
		for i, r := range urlRows {
			b.put(c, 2, y+i, r, render.FgDim, render.BgPopup)
		}
		hint = "o open again · y copy address · esc cancel"
	case f.Login != nil:
		b.put(c, 2, y+1, "Sign in in your browser. Waiting for it to finish.", render.FgWhite, render.BgPopup)
		y += 2
		for i, r := range urlRows {
			b.put(c, 2, y+i, r, render.FgDim, render.BgPopup)
		}
		hint = "o open again · y copy address · p paste the final address · esc cancel"
	}
	if a.st.Editing {
		b.fill(c, b.h-3, render.BgPopupIn)
		a.st.Edit.Draw(c, b.x+4, b.y+b.h-3, max(1, b.w-8), 1, render.Style{FG: render.FgWhite, BG: render.BgPopupIn}.TCell(), true)
		hint = "enter send · esc back"
	}
	b.hint(c, hint)
}

// wrapRunes cuts s into rows of at most width cells, at most maxRows of them;
// the last row ends in … when text is left.
func wrapRunes(s string, width, maxRows int) []string {
	var rows []string
	rs := []rune(s)
	for len(rs) > 0 && len(rows) < maxRows {
		n := min(width, len(rs))
		rows = append(rows, string(rs[:n]))
		rs = rs[n:]
	}
	if len(rs) > 0 && len(rows) > 0 {
		last := []rune(rows[len(rows)-1])
		rows[len(rows)-1] = string(last[:len(last)-1]) + "…"
	}
	return rows
}
