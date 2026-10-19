package app

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

// openSnippets shows the global snippets: paste, rename or delete them.
func (a *App) openSnippets() {
	if err := a.reloadSnippets(); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.st.SnippetDel = ""
	a.openOverlay(OverSnippets)
}

func (a *App) reloadSnippets() error {
	list, err := a.store.LoadSnippets()
	if err != nil {
		return err
	}
	a.st.Snippets = list
	return nil
}

func (a *App) snippetsKey(k *tcell.EventKey) {
	if a.st.Editing {
		a.snippetRenameKey(k)
		return
	}
	n := len(a.st.Snippets)
	sel := clampSel(a.st.Sel, n)
	switch {
	case a.navKey(k, n, true):
		a.st.SnippetDel = ""
	case k.Key() == tcell.KeyEnter && n > 0:
		a.pasteSnippetName(a.st.Snippets[sel].Name)
	case isKey(k, 'r') && n > 0:
		a.beginEdit(a.st.Snippets[sel].Name, false, false)
		a.st.SnippetDel = ""
	case isKey(k, 'd') && n > 0:
		name := a.st.Snippets[sel].Name
		if a.st.SnippetDel != name {
			a.st.SnippetDel = name
			a.notify("press d again to delete " + name)
			return
		}
		a.deleteSnippet(name)
	}
}

func (a *App) snippetRenameKey(k *tcell.EventKey) {
	switch {
	case k.Key() == tcell.KeyEsc:
		a.endEdit()
	case k.Key() == tcell.KeyEnter:
		a.commitSnippetRename()
	default:
		a.st.Edit.Handle(k)
	}
}

func (a *App) commitSnippetRename() {
	sel := clampSel(a.st.Sel, len(a.st.Snippets))
	if len(a.st.Snippets) == 0 {
		a.endEdit()
		return
	}
	old := a.st.Snippets[sel].Name
	name := strings.TrimSpace(a.st.Edit.String())
	if err := a.store.RenameSnippet(old, name); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.endEdit()
	if err := a.reloadSnippets(); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.selectSnippet(name)
	a.notify("renamed " + old + " to " + name)
}

func (a *App) deleteSnippet(name string) {
	if err := a.store.DeleteSnippet(name); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.st.SnippetDel = ""
	if err := a.reloadSnippets(); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.st.Sel = clampSel(a.st.Sel, len(a.st.Snippets))
	a.notify("deleted snippet " + name)
}

// selectSnippet moves the list cursor to the snippet called name.
func (a *App) selectSnippet(name string) {
	for i, sn := range a.st.Snippets {
		if sn.Name == name {
			a.st.Sel = i
			return
		}
	}
	a.st.Sel = clampSel(a.st.Sel, len(a.st.Snippets))
}

func (a *App) drawSnippets(c ui.Canvas, w, h int) {
	items := make([]item, len(a.st.Snippets))
	for i, sn := range a.st.Snippets {
		items[i] = item{label: sn.Name, right: snippetLines(sn.Text), detail: oneLine(sn.Text)}
	}
	hint := "j/k select · enter paste · r rename · d delete · esc close"
	switch {
	case a.st.Editing:
		hint = "type a new name · enter rename · esc cancel"
	case a.st.SnippetDel != "":
		hint = "press d again to delete · esc cancel"
	}
	empty := "select text, then :save_snippet <name>"
	a.searchListEmpty(c, w, h, 73, len(items), "snippets", "saved text", items, hint, empty)
}

func snippetLines(text string) string {
	n := strings.Count(text, "\n") + 1
	if n == 1 {
		return strconv.Itoa(len([]rune(text))) + " chars"
	}
	return strconv.Itoa(n) + " lines"
}
