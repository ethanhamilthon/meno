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
		a.st.Editing, a.st.Buffer = true, []rune(a.st.Snippets[sel].Name)
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
	switch k.Key() {
	case tcell.KeyEsc:
		a.st.Editing, a.st.Buffer = false, nil
	case tcell.KeyEnter:
		a.commitSnippetRename()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n := len(a.st.Buffer); n > 0 {
			a.st.Buffer = a.st.Buffer[:n-1]
		}
	case tcell.KeyRune:
		if k.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 {
			a.st.Buffer = append(a.st.Buffer, k.Rune())
		}
	}
}

func (a *App) commitSnippetRename() {
	sel := clampSel(a.st.Sel, len(a.st.Snippets))
	if len(a.st.Snippets) == 0 {
		a.st.Editing, a.st.Buffer = false, nil
		return
	}
	old := a.st.Snippets[sel].Name
	name := strings.TrimSpace(string(a.st.Buffer))
	if err := a.store.RenameSnippet(old, name); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.st.Editing, a.st.Buffer = false, nil
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
	sel := clampSel(a.st.Sel, len(a.st.Snippets))
	items := make([]item, len(a.st.Snippets))
	for i, sn := range a.st.Snippets {
		label := sn.Name
		if a.st.Editing && i == sel {
			label = string(a.st.Buffer) + "▏"
		}
		items[i] = item{label: label, right: snippetLines(sn.Text), detail: oneLine(sn.Text)}
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
