package app

// lastNotice is the text of the newest notice of the active session.
func (a *App) lastNotice() string {
	if ns := a.currentNotices(); len(ns) > 0 {
		return ns[len(ns)-1].Text
	}
	return ""
}
