package app

import "time"

// notice is a UI-only message shown in the timeline (not sent to the model):
// events like "session resumed" and errors like "stop the current turn first".
type notice struct {
	Session string // session id it belongs to
	After   int    // number of session turns before it
	Text    string
	Err     bool
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
	if text == "" {
		return
	}
	n := notice{Text: text, Err: isErr, At: noticeNow()}
	if a.st.Session != nil {
		n.Session = a.st.Session.ID()
		n.After = a.st.Session.Len()
	}
	a.st.Notices = append(a.st.Notices, n)
	if len(a.st.Notices) > maxNotices {
		a.st.Notices = a.st.Notices[len(a.st.Notices)-maxNotices:]
	}
	a.st.Scroll = 0
}

func (a *App) notify(text string)    { a.addNotice(text, false) }
func (a *App) notifyErr(text string) { a.commandFailed = true; a.addNotice(text, true) }

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
		if now.Sub(n.At) < ttl {
			kept = append(kept, n)
		}
	}
	removed := len(kept) != len(a.st.Notices)
	a.st.Notices = kept
	return removed
}
