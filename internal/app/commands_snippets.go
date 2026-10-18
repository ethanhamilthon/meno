package app

import (
	"strings"

	"meno/internal/config"
	"meno/internal/ui"
)

// saveSnippet implements :save_snippet <name>: the last visual or mouse
// selection becomes a global snippet.
func (a *App) saveSnippet(args []string) {
	name, ok := a.snippetArg(args, "save_snippet")
	if !ok {
		return
	}
	text := a.st.SelText
	if strings.TrimSpace(text) == "" {
		a.notifyErr("no selection: select text with v or the mouse first")
		return
	}
	if err := a.store.SaveSnippet(name, text); err != nil {
		a.notifyErr(err.Error())
		return
	}
	a.notify("saved snippet " + name)
}

// pasteSnippet implements :paste_snippet <name>: the text goes into the
// composer at the cursor and insert mode resumes there.
func (a *App) pasteSnippet(args []string) {
	name, ok := a.snippetArg(args, "paste_snippet")
	if !ok {
		return
	}
	if !a.pasteSnippetName(name) {
		return
	}
	a.notify("pasted snippet " + name)
}

// snippetArg reads the single name of a snippet command.
func (a *App) snippetArg(args []string, cmd string) (string, bool) {
	switch {
	case len(args) == 0:
		a.notifyErr("usage: :" + cmd + " <name>")
		return "", false
	case len(args) > 1:
		a.notifyErr("snippet names cannot contain spaces")
		return "", false
	}
	if err := config.CheckSnippetName(args[0]); err != nil {
		a.notifyErr(err.Error())
		return "", false
	}
	return args[0], true
}

// pasteSnippetName inserts a stored snippet at the composer cursor.
func (a *App) pasteSnippetName(name string) bool {
	sn, err := a.store.Snippet(name)
	if err != nil {
		a.notifyErr(err.Error())
		return false
	}
	a.pushUndo(a.snapshot())
	a.insertAtCursor([]rune(sn.Text))
	a.st.InLog, a.st.Mode, a.st.VimBuf = false, ui.ModeInsert, nil
	a.st.InsBase = a.snapshot()
	a.closeOverlay()
	return true
}
