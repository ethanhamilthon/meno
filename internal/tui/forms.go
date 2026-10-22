package tui

import (
	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
	"meno/internal/render"
)

type field struct {
	label  string
	opts   []string
	secret bool
	multi  bool // Alt+Enter or Ctrl+J starts a new line
	get    func() string
	set    func(string) error
}

// formKey drives a list of fields. commit runs after every change that sticks,
// so a form needs no save key: what is entered is saved at once.
func (a *App) formKey(k *tcell.EventKey, fs []field, commit func()) {
	if len(fs) == 0 {
		return
	}
	if a.st.Field >= len(fs) {
		a.st.Field = len(fs) - 1
	}
	f := fs[a.st.Field]
	if a.st.Editing {
		a.editKey(k, f, commit)
		return
	}
	switch {
	case k.Key() == tcell.KeyDown || isKey(k, 'j'):
		a.st.Field = min(a.st.Field+1, len(fs)-1)
	case k.Key() == tcell.KeyUp || isKey(k, 'k'):
		a.st.Field = max(a.st.Field-1, 0)
	case len(f.opts) > 0 && (k.Key() == tcell.KeyRight || k.Key() == tcell.KeyLeft || k.Key() == tcell.KeyEnter || k.Rune() == ' '):
		step := 1
		if k.Key() == tcell.KeyLeft {
			step = -1
		}
		a.cycleField(f, step)
		commit()
	case k.Key() == tcell.KeyEnter:
		a.beginEdit(f.get(), f.secret, f.multi)
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
		a.notifyErr(err.Error())
	}
}

// beginEdit starts editing a text with the cursor at its end.
func (a *App) beginEdit(text string, secret, multi bool) {
	a.st.Editing = true
	a.st.Edit = textEdit{Secret: secret, Multi: multi}
	a.st.Edit.Set(text)
}

func (a *App) endEdit() {
	a.st.Editing = false
	a.st.Edit.Reset()
}

// editKey edits one field: Enter keeps the text, Esc drops it.
func (a *App) editKey(k *tcell.EventKey, f field, commit func()) {
	switch {
	case k.Key() == tcell.KeyEsc:
		a.endEdit()
	case k.Key() == tcell.KeyEnter && !a.st.Edit.Handle(k):
		if err := f.set(a.st.Edit.String()); err != nil {
			a.notifyErr(err.Error())
			return
		}
		a.endEdit()
		commit()
	default:
		a.st.Edit.Handle(k)
	}
}

func drawForm(c render.Canvas, b box, y int, fs []field, selField int, editing bool, ed *textEdit) {
	for i, f := range fs {
		isSel := i == selField
		bg := b.bg(isSel)
		b.fill(c, y+i, bg)
		mark := " "
		if isSel {
			mark = "▸"
		}
		b.put(c, 2, y+i, mark, render.Blue, bg)
		b.put(c, 4, y+i, padTo(f.label, 12), render.FgDim, bg)
		if len(f.opts) > 0 {
			chips(c, b, y+i, 16, f.opts, f.get())
			continue
		}
		if isSel && editing {
			ed.Draw(c, b.x+16, b.y+y+i, max(1, b.w-18), 1, render.Style{FG: render.FgWhite, BG: bg}.TCell(), true)
			continue
		}
		val := f.get()
		if f.secret {
			val = llm.Mask(val)
		}
		b.put(c, 16, y+i, val, render.FgWhite, bg)
	}
}

// drawFormNote shows why a form is not saved yet, on the row above the hint.
func drawFormNote(c render.Canvas, b box, note string) {
	if note != "" {
		b.put(c, 2, b.h-2, render.Truncate("not saved: "+note, max(1, b.w-6)), render.Yellow, render.BgPopup)
	}
}

func padTo(s string, n int) string { return render.Pad(s, n) }
