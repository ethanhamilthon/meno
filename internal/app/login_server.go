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

// probeFreePort is replaceable in tests to model a listener on the saved port.
var probeFreePort = func(m *cliproxy.Manager) bool { return m.PortFree() }

// initOAuth restores the provider if saved account files exist.
func (a *App) initOAuth() {
	a.oauth = cliproxy.New(a.paths.Home)
	a.oauth.Activate()
	if a.oauth.State().Accounts > 0 && a.oauth.Installed() {
		a.setManaged(true)
	}
}

// ensureOAuthServer installs and starts the helper. Call it outside the UI goroutine.
func (a *App) ensureOAuthServer(ctx context.Context, report func(string)) error {
	if err := a.oauth.Ensure(ctx, report); err != nil {
		return err
	}
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	running := a.oauth.Ready(probe)
	cancel()
	if running {
		a.postEvent(oauthHelperStatus{ready: true})
		return nil
	}
	if report != nil {
		report("starting the helper")
	}
	if a.daemons != nil && a.daemons.Running(llm.OAuthName) {
		a.daemons.Stop(llm.OAuthName)
	}
	a.chooseOAuthPort()
	if err := a.oauth.WriteConfig(); err != nil {
		return err
	}
	if a.daemons == nil {
		return errors.New("no process supervisor")
	}
	if err := a.daemons.Start(llm.OAuthName, a.oauth.Command(), a.paths.Home, os.Environ()); err != nil {
		return err
	}
	wait, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	if err := a.oauth.WaitReady(wait); err != nil {
		a.postEvent(oauthHelperStatus{ready: false})
		return errors.New("OAuth helper is not responding; it is restarting. Try again.")
	}
	a.postEvent(oauthHelperStatus{ready: true})
	a.refreshManagedProvider()
	return nil
}

// chooseOAuthPort moves the helper and rebuilds any managed provider.
func (a *App) chooseOAuthPort() bool {
	if probeFreePort(a.oauth) {
		return false
	}
	a.oauth.ChoosePort()
	a.refreshManagedProvider()
	return true
}

// startOAuth starts the installed helper and discovers accounts at launch.
func (a *App) startOAuth() {
	if a.oauth == nil {
		return
	}
	installed := a.oauth.Installed()
	st := a.oauth.State()
	if installed && (st.Accounts > 0 || !st.AccountsKnown) {
		safe.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			if err := a.ensureOAuthServer(ctx, nil); err != nil {
				a.postEvent(accountsLoaded{err: err})
				return
			}
			a.refreshAccounts()
		}, a.reportPanic)
	}
	if !a.oauthUpdates || !installed {
		return
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
		if a.st.Over != OverLoginFlow && !a.anyRunning() {
			a.oauth.Activate()
			a.refreshManagedProvider()
		}
	}
}

func (a *App) anyRunning() bool {
	for _, rt := range a.rtList {
		if rt.Running {
			return true
		}
	}
	return false
}

func (a *App) syncManagedProvider() {
	if a.hasManaged() {
		a.setManaged(true)
		a.syncProviderRun(a.managedProvider())
	}
}

func (a *App) refreshManagedProvider() {
	if a.hasManaged() {
		a.syncManagedProvider()
	}
}
