package app

import (
	"context"
	"strings"
	"time"

	"meno/internal/client"
	"meno/internal/proto"
)

// oauthAccount is a signed-in account the daemon lists.
type oauthAccount struct {
	File, Provider, Service, Label, Plan, Note string
	Disabled                                   bool
	Failed                                     int
}

// loginService is something the user can sign in to.
type loginService struct{ ID, Label string }

// loginFlow is one OAuth login in progress.
type loginFlow struct {
	ID     int
	Svc    loginService
	Steps  []string
	Login  *loginInfo // nil until the helper has given the page
	Err    string
	cancel context.CancelFunc
	state  string
}

// loginInfo is the page the user signs in on.
type loginInfo struct {
	URL, UserCode string
	Device        bool
}

type accountsLoaded struct {
	list []oauthAccount
	err  error
}
type flowStep struct {
	id   int
	text string
}
type flowLogin struct {
	id    int
	login loginInfo
	state string
	err   error
}
type flowDone struct {
	id  int
	err error
}

// loginServices is what :login offers, in the order of the list. The daemon
// knows the same list; this one is only for drawing the rows.
var loginServices = []loginService{
	{"claude", "Claude"}, {"codex", "ChatGPT / Codex"}, {"antigravity", "Antigravity"},
	{"xai", "xAI"}, {"meta", "Meta"}, {"kimi", "Kimi"},
}

// refreshAccounts lists the OAuth accounts off the UI goroutine.
func (a *App) refreshAccounts() {
	a.spawn(func() {
		var out []oauthAccount
		err := a.rpc(func(ctx context.Context, c *client.Client) error {
			r, e := c.LoginAccounts(ctx)
			for _, x := range r.Accounts {
				out = append(out, oauthAccount{File: x.File, Provider: x.Provider, Service: x.Service, Label: x.Label, Plan: x.Plan, Note: x.Note, Disabled: x.Disabled, Failed: b2i(x.Failed)})
			}
			return e
		})
		a.postEvent(accountsLoaded{list: out, err: err})
	})
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// accountAction runs a change to an OAuth account off the UI goroutine and
// then lists the accounts again.
func (a *App) accountAction(what string, fn func(ctx context.Context, c *client.Client) error) {
	a.spawn(func() {
		if err := a.rpc(fn); err != nil {
			a.postEvent(errNotice{text: what + ": " + err.Error()})
			return
		}
		a.refreshAccounts()
		a.refreshConfig()
	})
}

func (a *App) onAccountsLoaded(m accountsLoaded) {
	if m.err != nil {
		a.st.AccountsErr = m.err.Error()
		return
	}
	a.st.AccountsErr, a.st.Accounts = "", m.list
	a.st.Sel = clampSel(a.st.Sel, len(a.loginRows()))
	a.afterProvidersChanged()
}

func (a *App) openLogin() {
	a.openOverlay(OverLogin)
	a.refreshAccounts()
}

// onLoginEvent applies the messages of the account list and of a login.
func (a *App) onLoginEvent(msg any) {
	switch m := msg.(type) {
	case accountsLoaded:
		a.onAccountsLoaded(m)
	case flowStep:
		if m.id == a.st.Flow.ID {
			a.st.Flow.Steps = append(a.st.Flow.Steps, m.text)
		}
	case flowLogin:
		if m.id != a.st.Flow.ID {
			return
		}
		if m.err != nil {
			a.st.Flow.Err = m.err.Error()
			return
		}
		l := m.login
		a.st.Flow.Login, a.st.Flow.state = &l, m.state
		if !l.Device {
			if err := a.openURL(l.URL); err != nil {
				a.notifyErr(err.Error())
			}
		}
	case flowDone:
		if m.id != a.st.Flow.ID {
			return
		}
		if m.err != nil {
			a.st.Flow.Err = m.err.Error()
			return
		}
		a.cancelFlow()
		a.closeOverlay()
		a.notify("signed in")
		a.refreshAccounts()
		a.refreshConfig()
	}
}

// pollEvery is how often a login asks the daemon whether the browser is done.
var pollEvery = 1500 * time.Millisecond

func (a *App) cancelFlow() {
	if a.st.Flow.cancel != nil {
		a.st.Flow.cancel()
		a.st.Flow.cancel = nil
	}
}

// startLogin begins a login to svc: the daemon prepares the helper and gives
// the page; the UI opens it and waits for the account to appear.
func (a *App) startLogin(svc loginService) {
	a.cancelFlow()
	a.flowSeq++
	ctx, cancel := context.WithCancel(context.Background())
	id := a.flowSeq
	a.openOverlay(OverLoginFlow)
	a.st.Flow = loginFlow{ID: id, Svc: svc, cancel: cancel}
	every := pollEvery
	a.spawn(func() { a.runLogin(ctx, id, svc, every) })
}

func (a *App) runLogin(ctx context.Context, id int, svc loginService, every time.Duration) {
	a.postEvent(flowStep{id, "asking " + svc.Label + " for the login page"})
	var st proto.LoginStartResponse
	err := a.rpcCtx(ctx, func(c context.Context, cl *client.Client) error {
		var e error
		st, e = cl.LoginStart(c, svc.ID)
		return e
	})
	if err != nil {
		a.postEvent(flowLogin{id: id, err: err})
		return
	}
	a.postEvent(flowLogin{id: id, login: loginInfo{URL: st.URL, UserCode: st.UserCode, Device: st.Device}, state: st.State})
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
		var s proto.LoginStatusResponse
		if e := a.rpcCtx(ctx, func(c context.Context, cl *client.Client) error {
			var x error
			s, x = cl.LoginStatus(c, st.State)
			return x
		}); e != nil {
			continue
		}
		switch strings.ToLower(s.Status) {
		case "ok", "done", "success", "complete", "completed":
			a.postEvent(flowDone{id: id})
			return
		case "error", "failed":
			a.postEvent(flowDone{id: id, err: errString(s.Error)})
			return
		}
	}
	a.postEvent(flowDone{id: id, err: errString("the login timed out")})
}

type errString string

func (e errString) Error() string { return string(e) }

// submitCallback hands the daemon the address the browser was sent to.
func (a *App) submitCallback(addr string) {
	if addr == "" {
		return
	}
	a.spawn(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error { return c.LoginCallback(ctx, addr) }); err != nil {
			a.postEvent(errNotice{text: "login: " + err.Error()})
		}
	})
}

// restartOAuthHelper asks the daemon to restart the login helper.
func (a *App) restartOAuthHelper() {
	a.spawn(func() {
		if err := a.rpc(func(ctx context.Context, c *client.Client) error { return c.RestartLoginHelper(ctx) }); err != nil {
			a.postEvent(errNotice{text: "login helper: " + err.Error()})
			return
		}
		a.postEvent(errNotice{text: "login helper restarted"})
		a.refreshAccounts()
	})
}
