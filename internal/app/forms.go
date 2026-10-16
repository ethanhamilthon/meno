package app

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
	"meno/internal/ui"
)

type field struct {
	label  string
	opts   []string
	secret bool
	get    func() string
	set    func(string) error
}

func (a *App) formKey(k *tcell.EventKey, fs []field) {
	if len(fs) == 0 {
		return
	}
	if a.st.Field >= len(fs) {
		a.st.Field = len(fs) - 1
	}
	f := fs[a.st.Field]
	if a.st.Editing {
		a.editKey(k, f)
		return
	}
	switch {
	case k.Key() == tcell.KeyDown || k.Rune() == 'j':
		a.st.Field = min(a.st.Field+1, len(fs)-1)
	case k.Key() == tcell.KeyUp || k.Rune() == 'k':
		a.st.Field = max(a.st.Field-1, 0)
	case len(f.opts) > 0 && (k.Key() == tcell.KeyRight || k.Key() == tcell.KeyLeft || k.Key() == tcell.KeyEnter || k.Rune() == ' '):
		step := 1
		if k.Key() == tcell.KeyLeft {
			step = -1
		}
		a.cycleField(f, step)
	case k.Key() == tcell.KeyEnter:
		a.st.Editing = true
		a.st.Buffer = []rune(f.get())
	}
}

func (a *App) cycleField(f field, step int) {
	cur := f.get()
	idx := 0
	for i, o := range f.opts {
		if o == cur {
			idx = i
		}
	}
	n := len(f.opts)
	next := f.opts[((idx+step)%n+n)%n]
	if err := f.set(next); err != nil {
		a.st.Toast = err.Error()
	}
}

func (a *App) editKey(k *tcell.EventKey, f field) {
	switch k.Key() {
	case tcell.KeyEsc:
		a.st.Editing = false
		a.st.Buffer = nil
	case tcell.KeyEnter:
		if err := f.set(string(a.st.Buffer)); err != nil {
			a.st.Toast = err.Error()
		}
		a.st.Editing = false
		a.st.Buffer = nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.st.Buffer); n > 0 {
			a.st.Buffer = a.st.Buffer[:n-1]
		}
	case tcell.KeyRune:
		a.st.Buffer = append(a.st.Buffer, k.Rune())
	}
}

func drawForm(c ui.Canvas, b box, y int, fs []field, selField int, editing bool, buf string) {
	for i, f := range fs {
		isSel := i == selField
		bg := b.bg(isSel)
		b.fill(c, y+i, bg)
		mark := " "
		if isSel {
			mark = "▸"
		}
		b.put(c, 2, y+i, mark, ui.Blue, bg)
		b.put(c, 4, y+i, padTo(f.label, 12), ui.FgDim, bg)
		val := f.get()
		if isSel && editing {
			val = buf + "▏"
		} else if f.secret {
			val = llm.Mask(val)
		}
		if len(f.opts) > 0 {
			chips(c, b, y+i, 16, f.opts, f.get())
			continue
		}
		b.put(c, 16, y+i, val, ui.FgWhite, bg)
	}
}

func padTo(s string, n int) string { return ui.Pad(s, n) }
