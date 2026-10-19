package app

import (
	"context"
	"errors"
	"time"

	"meno/internal/cliproxy"
	"meno/internal/safe"
)

// loginFlow is one OAuth login in progress.
type loginFlow struct {
	ID     int
	Svc    cliproxy.Service
	Steps  []string
	Login  *cliproxy.Login // nil until the helper has given the page
	Err    string
	cancel context.CancelFunc
}

// pollEvery is how often a login asks the helper whether the browser is done.
var pollEvery = 1500 * time.Millisecond

// startLogin begins a login to svc: it prepares the helper, opens the page in
// the browser and waits for the account to appear.
func (a *App) startLogin(svc cliproxy.Service) {
	a.cancelFlow()
	a.flowSeq++
	ctx, cancel := context.WithCancel(context.Background())
	id := a.flowSeq
	a.openOverlay(OverLoginFlow)
	a.st.Flow = loginFlow{ID: id, Svc: svc, cancel: cancel}
	every := pollEvery // read here, on the UI goroutine
	safe.Go(func() { a.runLogin(ctx, id, svc, every) }, a.reportPanic)
}

func (a *App) cancelFlow() {
	if a.st.Flow.cancel != nil {
		a.st.Flow.cancel()
		a.st.Flow.cancel = nil
	}
}

func (a *App) runLogin(ctx context.Context, id int, svc cliproxy.Service, every time.Duration) {
	step := func(s string) { a.postEvent(flowStep{id: id, text: s}) }
	if err := a.ensureOAuthServer(ctx, step); err != nil {
		a.postEvent(flowDone{id: id, err: err})
		return
	}
	step("asking " + svc.Label + " for the login page")
	var lg cliproxy.Login
	err := a.withHelper(ctx, func(ctx context.Context) error {
		var e error
		lg, e = a.oauth.StartLogin(ctx, svc.ID)
		return e
	})
	a.postEvent(flowLogin{id: id, login: lg, err: err})
	if err != nil {
		return
	}
	limit := lg.ExpiresIn
	if limit <= 0 {
		limit = 10 * time.Minute
	}
	deadline := time.Now().Add(limit)
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
		if time.Now().After(deadline) {
			a.postEvent(flowDone{id: id, err: errors.New("the login expired: start it again")})
			return
		}
		var status, msg string
		err := a.withHelper(ctx, func(ctx context.Context) error {
			var e error
			status, msg, e = a.oauth.LoginStatus(ctx, lg.State)
			return e
		})
		if errors.Is(err, cliproxy.ErrHelperDown) {
			err = errors.New(a.accountError(err))
		}
		switch {
		case err != nil:
			if failures++; failures >= 5 {
				a.postEvent(flowDone{id: id, err: err})
				return
			}
		case status == "ok":
			a.postEvent(flowDone{id: id})
			return
		case status == "error":
			if msg == "" {
				msg = "the login failed"
			}
			a.postEvent(flowDone{id: id, err: errors.New(msg)})
			return
		default:
			failures = 0
		}
	}
}

func (a *App) onFlowStep(m flowStep) {
	if m.id == a.st.Flow.ID {
		a.st.Flow.Steps = append(a.st.Flow.Steps, m.text)
	}
}

func (a *App) onFlowLogin(m flowLogin) {
	f := &a.st.Flow
	if m.id != f.ID {
		return
	}
	if m.err != nil {
		f.Err = m.err.Error()
		return
	}
	f.Login = &m.login
	a.scr.SetClipboard([]byte(m.login.URL))
	if err := a.openURL(m.login.URL); err != nil {
		f.Steps = append(f.Steps, "could not open a browser: "+err.Error())
		return
	}
	f.Steps = append(f.Steps, "opened the page in your browser; the address is also copied")
}

func (a *App) onFlowDone(m flowDone) {
	f := &a.st.Flow
	if m.id != f.ID {
		return
	}
	if m.err != nil {
		f.Err = m.err.Error()
		return
	}
	a.cancelFlow()
	a.notify("signed in to " + f.Svc.Label)
	a.openOverlay(OverLogin)
	a.refreshAccounts()
}
