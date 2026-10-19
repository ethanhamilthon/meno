package app

import (
	"context"
	"errors"
	"os"
	"time"

	"meno/internal/cliproxy"
	"meno/internal/llm"
	"meno/internal/safe"
)

// The OAuth side of :login. The accounts live in the local cliproxyapi helper
// (internal/cliproxy). meno shows them in the login list and offers all of
// them to the model as one provider, "oauth", that it rebuilds from the
// accounts and never saves.

// loginParts is the state of :login that lives outside State: the helper, the
// numbering of logins, and the hooks tests replace.
type loginParts struct {
	oauth        *cliproxy.Manager  // the local helper behind the OAuth logins
	oauthUpdates bool               // download the newest helper at start (off in tests)
	flowSeq      int                // numbers the logins, so a stale one is ignored
	openURL      func(string) error // opens a page in the browser (tests replace it)
}

// onLoginEvent routes the messages of the OAuth side to their handlers.
func (a *App) onLoginEvent(msg any) {
	switch m := msg.(type) {
	case accountsLoaded:
		a.onAccountsLoaded(m)
	case oauthRefreshed:
		a.onOAuthRefreshed(m)
	case flowStep:
		a.onFlowStep(m)
	case flowLogin:
		a.onFlowLogin(m)
	case flowDone:
		a.onFlowDone(m)
	}
}

// managedProvider is the provider for the models of the OAuth accounts.
func (a *App) managedProvider() llm.Provider {
	return llm.Provider{
		Name: llm.OAuthName, BaseURL: a.oauth.APIURL(), APIKey: a.oauth.APIKey(),
		Protocol: llm.ProtoChat, Run: a.oauth.Command(), Source: llm.SourceOAuth,
	}
}

// setManaged makes the provider list hold the OAuth provider exactly when want.
func (a *App) setManaged(want bool) {
	out := make([]llm.Provider, 0, len(a.st.Providers)+1)
	for _, p := range a.st.Providers {
		if !p.Managed() {
			out = append(out, p)
		}
	}
	if want {
		out = append(out, a.managedProvider())
	}
	a.st.Providers = out
}

func (a *App) hasManaged() bool {
	for _, p := range a.st.Providers {
		if p.Managed() {
			return true
		}
	}
	return false
}

// initOAuth runs at start: with accounts from an earlier run the OAuth
// provider exists at once, so the models are there before the helper answers.
func (a *App) initOAuth() {
	a.oauth = cliproxy.New(a.paths.Home)
	a.oauth.Activate() // a helper downloaded by the last run starts now
	if st := a.oauth.State(); st.HasAccounts && a.oauth.Installed() {
		a.setManaged(true)
	}
}

// enabledAccounts counts the accounts that are switched on.
func (a *App) enabledAccounts() int {
	n := 0
	for _, acc := range a.st.Accounts {
		if !acc.Disabled {
			n++
		}
	}
	return n
}

// refreshAccounts asks the helper for its accounts, off the UI goroutine.
func (a *App) refreshAccounts() {
	if a.oauth == nil || !a.oauth.Installed() {
		return
	}
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		list, err := a.oauth.Accounts(ctx)
		a.postEvent(accountsLoaded{list: list, err: err})
	}, a.reportPanic)
}

func (a *App) onAccountsLoaded(m accountsLoaded) {
	if m.err != nil {
		a.st.AccountsErr = m.err.Error()
		return
	}
	a.st.AccountsErr = ""
	a.st.Accounts = m.list
	n := a.enabledAccounts()
	a.oauth.Update(func(s *cliproxy.State) { s.HasAccounts = n > 0 })
	a.setManaged(n > 0)
	if n > 0 {
		a.syncProviderRun(a.managedProvider())
	} else if a.daemons != nil {
		a.daemons.Stop(llm.OAuthName)
	}
	a.st.Sel = clampSel(a.st.Sel, len(a.loginRows()))
	a.fetchModels()
	a.afterProvidersChanged()
}

// ensureOAuthServer installs the helper when it is missing and starts it when
// it does not run. It blocks, so call it off the UI goroutine.
func (a *App) ensureOAuthServer(ctx context.Context, report func(string)) error {
	if err := a.oauth.Ensure(ctx, report); err != nil {
		return err
	}
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	running := a.oauth.Ready(probe)
	cancel()
	if running {
		return nil
	}
	report("starting the helper")
	if !a.oauth.PortFree() {
		a.oauth.ChoosePort() // another program holds the port
	}
	if err := a.oauth.WriteConfig(); err != nil {
		return err
	}
	if a.daemons == nil {
		return errors.New("no process supervisor")
	}
	if err := a.daemons.Start(llm.OAuthName, a.oauth.Command(), a.paths.Home, os.Environ()); err != nil {
		return err
	}
	wait, cancelWait := context.WithTimeout(ctx, 30*time.Second)
	defer cancelWait()
	if err := a.oauth.WaitReady(wait); err != nil {
		return errors.New("the helper did not start, see " + a.daemons.LogPath(llm.OAuthName))
	}
	return nil
}

// startOAuth runs at launch when accounts exist: it lists them as soon as the
// helper answers and, if updates are on, looks for a newer helper.
func (a *App) startOAuth() {
	if a.oauth == nil {
		return
	}
	if a.hasManaged() {
		safe.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			if a.oauth.WaitReady(ctx) == nil {
				a.refreshAccounts()
			}
		}, a.reportPanic)
	}
	if !a.oauthUpdates || !a.oauth.Installed() {
		return // the helper is installed by the first OAuth login, then kept current
	}
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		go func() {
			select {
			case <-a.done:
				cancel()
			case <-ctx.Done():
			}
		}()
		out, err := a.oauth.Refresh(ctx, time.Now())
		a.postEvent(oauthRefreshed{out: out, err: err})
	}, a.reportPanic)
}

// EnableOAuthUpdates lets meno download the newest cliproxyapi at start. Tests
// leave it off, so they never touch the network.
func (a *App) EnableOAuthUpdates() { a.oauthUpdates = true }

func (a *App) onOAuthRefreshed(m oauthRefreshed) {
	switch {
	case m.err != nil:
		if !a.oauth.Installed() {
			a.notifyErr("cliproxyapi: " + m.err.Error())
		}
	case m.out.Major != "":
		a.notify("cliproxyapi " + m.out.Major + " is out; meno keeps " + a.oauth.State().Version + " until it supports the new major version")
	case m.out.Downloaded != "":
		// Switch now when nothing is using the helper; otherwise at the next start.
		if a.st.Over != OverLoginFlow && !a.anyRunning() {
			a.oauth.Activate()
			if a.hasManaged() {
				a.setManaged(true)
				a.syncProviderRun(a.managedProvider())
			}
		}
	}
}

// anyRunning reports whether any loaded session has a turn in progress.
func (a *App) anyRunning() bool {
	for _, rt := range a.rtList {
		if rt.Running {
			return true
		}
	}
	return false
}
