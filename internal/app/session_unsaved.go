package app

import (
	"context"
	"errors"

	"meno/internal/client"
	"meno/internal/proto"
)

// forgetEmpty tells the daemon to drop an empty session nobody used. The
// daemon keeps such a session in memory only, so this stores nothing and
// announces nothing; failures do not matter.
func (a *App) forgetEmpty(id string) {
	a.spawn(func() {
		_ = a.rpc(func(ctx context.Context, c *client.Client) error { return c.DeleteSession(ctx, id) })
	})
}

// sessionGone reports whether the daemon does not know the session: it was
// only in memory and the daemon restarted since.
func sessionGone(err error) bool {
	var re *client.RemoteError
	return errors.As(err, &re) && re.Code == proto.CodeNotFound
}

// replaceGone swaps a runtime whose session the daemon lost for a fresh
// session in the same directory and sends the message again, so a restart of
// the daemon never costs the user the text they typed.
func (a *App) replaceGone(old *Runtime, msg msgDraft) {
	s, err := a.createSession(old.Cwd)
	if err != nil {
		a.notifyFailure("not sent: " + err.Error() + " · esc to dismiss")
		a.restoreDraft([]msgDraft{msg})
		return
	}
	wasCurrent := a.st.Runtime == old
	rt := a.newRuntime(s, old.Cwd)
	rt.watch = a.watch(s.ID())
	if wasCurrent {
		a.st.Runtime = rt
	}
	a.dropRuntime(old)
	if wasCurrent {
		a.send(msg)
	}
}
