package app

// statusUsageText is the session line of the status bar, with icons instead of
// words: tokens in/out, cache hit of the last request/of the session, the
// context in use/window, and the spend. Everything in it is about the session:
// it does not change while the agent works.
func (a *App) statusUsageText() string { return a.statusUsage(1 << 30) }

// lastNotice is the text of the newest notice of the active session.
func (a *App) lastNotice() string {
	if ns := a.currentNotices(); len(ns) > 0 {
		return ns[len(ns)-1].Text
	}
	return ""
}
