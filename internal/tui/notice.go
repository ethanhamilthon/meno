package tui

import "time"

// notice is a UI-only message shown in the timeline (not sent to the model):
// events like "session resumed" and errors like "stop the current turn first".
type notice struct {
	Session string // session id it belongs to
	After   int    // number of session turns before it
	Text    string
	Err     bool
	Sticky  bool // a failure the user must see: it stays until dismissed with Esc
	At      time.Time
}

const maxNotices = 200

// Notices fade out: they are hints about what just happened, not history.
const (
	noticeTTL    = 5 * time.Second
	noticeErrTTL = 10 * time.Second
)

// noticeNow is replaced in tests.
var noticeNow = time.Now

func (a *App) addNotice(text string, isErr bool) {
	a.noticeFor(a.st.Runtime, text, isErr, false)
}

// noticeFor records background failures without changing active command or scroll state.
func (a *App) noticeFor(rt *Runtime, text string, isErr, sticky bool) {
	if text == "" {
		return
	}
	n := notice{Text: text, Err: isErr, Sticky: sticky, At: noticeNow()}
	if rt != nil && rt.Session != nil {
		n.Session = rt.ID()
		n.After = rt.Session.Len()
	}
	a.st.Notices = append(a.st.Notices, n)
	if len(a.st.Notices) > maxNotices {
		a.st.Notices = a.st.Notices[len(a.st.Notices)-maxNotices:]
	}
	if rt != nil {
		rt.Scroll = 0
	}
	if rt == a.st.Runtime && isErr {
		a.commandFailed = true
	}
}

func (a *App) notify(text string)    { a.addNotice(text, false) }
func (a *App) notifyErr(text string) { a.commandFailed = true; a.addNotice(text, true) }

// notifyFailure is an error that is not allowed to fade: a message that was not
// sent stays on screen until the user closes it.
func (a *App) notifyFailure(text string) {
	a.notifyErr(text)
	if n := len(a.st.Notices); n > 0 {
		a.st.Notices[n-1].Sticky = true
	}
}

// dismissNotices removes the sticky failures of the current session and says
// whether there were any.
func (a *App) dismissNotices() bool {
	if a.st.Session == nil {
		return false
	}
	kept := a.st.Notices[:0]
	for _, n := range a.st.Notices {
		if !(n.Sticky && n.Session == a.st.Session.ID()) {
			kept = append(kept, n)
		}
	}
	gone := len(kept) != len(a.st.Notices)
	a.st.Notices = kept
	return gone
}

// currentNotices are the notices of the active session.
func (a *App) currentNotices() []notice {
	if a.st.Session == nil {
		return nil
	}
	var out []notice
	for _, n := range a.st.Notices {
		if n.Session == a.st.Session.ID() {
			out = append(out, n)
		}
	}
	return out
}

// timelineEmpty reports whether there is nothing but (maybe) notices to show,
// i.e. the welcome screen should be visible.
func (a *App) timelineEmpty() bool {
	return a.st.Session == nil || a.st.Session.Len() == 0 && a.st.LiveText == "" && a.st.LiveThink == "" && len(a.st.LiveCalls) == 0
}

// pruneNotices drops expired notices and reports whether anything was removed.
func (a *App) pruneNotices() bool {
	now := noticeNow()
	kept := a.st.Notices[:0]
	for _, n := range a.st.Notices {
		ttl := noticeTTL
		if n.Err {
			ttl = noticeErrTTL
		}
		if now.Sub(n.At) < ttl || n.Sticky {
			kept = append(kept, n)
		}
	}
	removed := len(kept) != len(a.st.Notices)
	a.st.Notices = kept
	return removed
}
