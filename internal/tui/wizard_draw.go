package tui

import (
	"meno/internal/llm"
	"meno/internal/render"
)

func (a *App) drawWizard(c render.Canvas, w, h int) {
	steps := a.wizSteps()
	b := a.panelBox(w, h, 73, len(steps)+6)
	b.frame(c)
	title, name := "login /", "add provider"
	if a.st.Wiz.mcp {
		title, name = "mcp /", "new server"
	}
	b.put(c, 2, 0, title, render.FgDim, render.BgPopup)
	b.put(c, len([]rune(title))+3, 0, name, render.Blue, render.BgPopup)
	b.putRight(c, 0, "step "+itoa(a.st.Wiz.i+1)+"/"+itoa(len(steps)), render.FgDim, render.BgPopup)
	fields := a.wizFields()
	for i, label := range steps {
		y := 2 + i
		cur := i == a.st.Wiz.i
		mark, fg := " ", render.FgDim
		if cur {
			mark, fg = "▸", render.Blue
		}
		b.put(c, 2, y, mark, render.Blue, render.BgPopup)
		b.put(c, 4, y, padTo(label, 12), fg, render.BgPopup)
		for _, f := range fields {
			if f.label != label {
				continue
			}
			switch {
			case len(f.opts) > 0 && i <= a.st.Wiz.i:
				chips(c, b, y, 16, f.opts, f.get())
			case i < a.st.Wiz.i:
				val := f.get()
				if f.secret {
					val = llm.Mask(val)
				}
				b.put(c, 16, y, render.Truncate(val, max(1, b.w-18)), render.FgWhite, render.BgPopup)
			}
		}
	}
	hint := "enter next · esc cancel"
	if len(a.wizField().opts) > 0 {
		hint = "← → choose · enter next · esc cancel"
	}
	b.hint(c, hint)
}
