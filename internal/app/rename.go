package app

import "meno/internal/agent"

// renameSession handles ":rename <name>": it sets the title of the current
// session. Until then the title is the start of the first user message.
func (a *App) renameSession(arg string) {
	if arg == "" {
		a.notify("usage: :rename <name> · now: " + a.st.Session.DisplayTitle())
		return
	}
	title, err := agent.NormalizeTitle(arg)
	if err != nil {
		a.notifyErr(err.Error())
		return
	}
	if err := agent.RenameSession(a.paths.Home, a.st.Session.ID, title); err != nil {
		a.notifyErr("rename failed: " + err.Error())
		return
	}
	a.st.Session.Title = title
	a.notify("session renamed")
}
