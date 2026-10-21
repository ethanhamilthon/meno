package app

// setConnLost records whether the stream of a session is broken. A fresh
// snapshot proves the connection works again.
func (a *App) setConnLost(sid string, lost bool) {
	if a.connLost == nil {
		a.connLost = map[string]bool{}
	}
	if lost {
		a.connLost[sid] = true
	} else {
		delete(a.connLost, sid)
	}
}

// offline reports that the current session has no working stream. What it
// shows may be stale, so actions that depend on it are held back.
func (a *App) offline() bool {
	return a.st.Runtime != nil && a.connLost[a.st.Runtime.ID()]
}

// connLabel is the persistent status text of the connection ("" when live).
func (a *App) connLabel() string {
	if a.offline() {
		return "offline · state may be stale"
	}
	return ""
}
