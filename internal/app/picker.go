package app

import (
	"context"
	"os"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"meno/internal/attach"
	"meno/internal/objtag"
	"meno/internal/pathsearch"
	"meno/internal/safe"
	"meno/internal/ui"
)

// pickState is the @ picker: a query typed after @ (still plain text until the
// user accepts something) and the results of an asynchronous search.
type pickState struct {
	Start int // rune offset of the @
	Query string
	Items []pathsearch.Entry
	// ResultsFor is the query the current Items answer; while it lags behind
	// Query, the list is shown but cannot be accepted.
	ResultsFor string
	Base       string // directory the results are relative to
	DirPrefix  string // the directory part the user typed, "" when none
	gen        int
	cancel     context.CancelFunc
	dismissed  int // start offset the user closed with Esc, -1 none
}

type pickResults struct {
	sid   string
	gen   int
	query string
	items []pathsearch.Entry
	base  string
	dir   string
}

// pickToken finds the @ word around the cursor: it must start at the beginning
// of the input or after whitespace and contain no whitespace up to the cursor.
func (a *App) pickToken() (start, end int, query string, ok bool) {
	in := a.st.Input
	c := min(max(0, a.st.Cursor), len(in))
	s := c
	for s > 0 && !unicode.IsSpace(in[s-1]) && !isObjRune(in[s-1]) {
		s--
	}
	if s >= len(in) || in[s] != '@' || s >= c {
		return 0, 0, "", false
	}
	e := c
	for e < len(in) && !unicode.IsSpace(in[e]) && !isObjRune(in[e]) {
		e++
	}
	return s, e, string(in[s+1 : c]), true
}

// pickSync opens, updates or closes the picker after the input changed.
func (a *App) pickSync() {
	if a.st.Mode != ui.ModeInput || a.st.Ask != nil {
		a.pickClose()
		return
	}
	start, _, query, ok := a.pickToken()
	p := a.st.Pick
	if !ok {
		a.pickClose()
		if p != nil {
			a.st.Pick = nil
		}
		return
	}
	if p != nil && p.dismissed == start {
		return
	}
	if p != nil && p.Start == start && p.Query == query {
		return
	}
	if p == nil {
		p = &pickState{dismissed: -1}
		a.st.Pick = p
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.Start, p.Query, p.dismissed = start, query, -1
	p.gen++
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	cwd, sid, gen, query := a.st.Cwd, a.st.Session.ID(), p.gen, query
	home, _ := os.UserHomeDir()
	a.running.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		base, fuzzy, dir := pathsearch.Resolve(cwd, home, query)
		items := pathsearch.Search(ctx, base, fuzzy, 50)
		if ctx.Err() == nil {
			a.postEvent(pickResults{sid, gen, query, items, base, dir})
		}
	}()
}

func (a *App) onPickResults(m pickResults) {
	rt := a.rts[m.sid]
	if rt == nil || rt.Pick == nil || rt.Pick.gen != m.gen {
		return // stale: a newer query replaced it
	}
	rt.Pick.Items, rt.Pick.Base, rt.Pick.DirPrefix, rt.Pick.ResultsFor = m.items, m.base, m.dir, m.query
	if rt == a.st.Runtime {
		a.st.Sel = 0
	}
}

func (a *App) pickClose() {
	if p := a.st.Pick; p != nil && p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if a.st.Pick != nil {
		a.st.Pick.Items = nil
	}
}

// pickOpen reports whether the picker has something to show.
func (a *App) pickOpen() bool {
	p := a.st.Pick
	return p != nil && p.dismissed != p.Start && len(p.Items) > 0 && a.st.Mode == ui.ModeInput
}

// pickItems adapts the results for the completion popup.
func (a *App) pickItems() []acItem {
	var out []acItem
	for _, e := range a.st.Pick.Items {
		desc := "file"
		switch {
		case e.Dir && e.Symlink:
			desc = "symlink to folder"
		case e.Dir:
			desc = "folder"
		case e.Symlink:
			desc = "symlink"
		default:
			if _, ok := attach.PathFromText(e.Path, ""); ok {
				desc = "image"
			}
		}
		out = append(out, acItem{kind: '@', name: e.Rel, desc: desc})
	}
	return out
}

// pickKey handles keys while the picker is open. Enter creates one object; Tab
// enters a folder (the path stays text) or accepts a file.
func (a *App) pickKey(k *tcell.EventKey) bool {
	p := a.st.Pick
	n := len(p.Items)
	switch k.Key() {
	case tcell.KeyDown:
		a.st.Sel = clampSel(a.st.Sel+1, n)
	case tcell.KeyUp:
		a.st.Sel = clampSel(a.st.Sel-1, n)
	case tcell.KeyEsc:
		p.dismissed = p.Start // close the list, keep the typed text
		a.pickClose()
	case tcell.KeyTab:
		if p.ResultsFor != p.Query {
			return true // still searching: do not act on the old list
		}
		e := p.Items[clampSel(a.st.Sel, n)]
		if e.Dir {
			a.pickEnter(e)
		} else {
			a.pickAccept(e)
		}
	case tcell.KeyEnter:
		if k.Modifiers()&(tcell.ModShift|tcell.ModAlt|tcell.ModCtrl) != 0 {
			return false
		}
		if p.ResultsFor != p.Query {
			return true
		}
		a.pickAccept(p.Items[clampSel(a.st.Sel, n)])
	default:
		return false
	}
	return true
}

// pickAccept replaces the @ word with one object, or attaches an image file.
// A folder reference and entering a folder are separate actions: this is the reference.
func (a *App) pickAccept(e pathsearch.Entry) {
	start, end, _, ok := a.pickToken()
	if !ok {
		return
	}
	if !e.Dir {
		if p, ok := attach.PathFromText(e.Path, userHome()); ok {
			in := a.st.Input
			a.st.Input = append(append([]rune{}, in[:start]...), in[end:]...)
			a.st.Cursor, a.st.WantSet, a.st.Sel = start, false, 0
			a.st.Pick = nil
			a.attachFile(p, false)
			return
		}
	}
	kind := objtag.File
	if e.Dir {
		kind = objtag.Folder
	}
	r, ok := objectRune(inputObject{Kind: kind, Ref: e.Path})
	if !ok {
		a.notifyErr("too many distinct references in this session of meno; restart to pick more")
		return
	}
	in := a.st.Input
	out := append(append([]rune{}, in[:start]...), r)
	cursor := len(out)
	if end >= len(in) || !unicode.IsSpace(in[end]) {
		out = append(out, ' ')
		cursor++
	} else {
		cursor++
	}
	out = append(out, in[end:]...)
	a.st.Input, a.st.Cursor, a.st.WantSet, a.st.Sel = out, min(cursor, len(out)), false, 0
	a.st.Pick = nil
}

// pickEnter continues the search inside a folder: the query becomes its path.
func (a *App) pickEnter(e pathsearch.Entry) {
	start, end, _, ok := a.pickToken()
	if !ok {
		return
	}
	rel := e.Rel
	if p := a.st.Pick; p != nil {
		rel = p.DirPrefix + e.Rel
	}
	text := []rune("@" + strings.TrimSuffix(rel, "/") + "/")
	in := a.st.Input
	out := append(append(append([]rune{}, in[:start]...), text...), in[end:]...)
	a.st.Input, a.st.Cursor, a.st.WantSet, a.st.Sel = out, start+len(text), false, 0
	a.pickSync()
}
