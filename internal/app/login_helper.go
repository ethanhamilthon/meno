package app

import (
	"context"
	"errors"
	"time"

	"meno/internal/cliproxy"
	"meno/internal/safe"
)

func (a *App) accountError(err error) string {
	if errors.Is(err, cliproxy.ErrHelperDown) {
		return "OAuth helper is not responding"
	}
	return err.Error()
}

// withHelper ensures readiness, calls fn, and retries once after a transport failure.
func (a *App) withHelper(ctx context.Context, fn func(context.Context) error) error {
	ready := a.oauth.Ready(ctx)
	if a.helperReadyFn != nil {
		ready = a.helperReadyFn(ctx)
	}
	if !ready {
		if err := a.ensureHelper(ctx); err != nil {
			return err
		}
	}
	err := fn(ctx)
	if !errors.Is(err, cliproxy.ErrHelperDown) {
		return err
	}
	if startErr := a.ensureHelper(ctx); startErr != nil {
		return startErr
	}
	return fn(ctx)
}

func (a *App) ensureHelper(ctx context.Context) error {
	if a.helperStart != nil {
		return a.helperStart(ctx)
	}
	return a.ensureOAuthServer(ctx, nil)
}

// On helper failure, use bounded delayed restarts and retain the log path.
func (a *App) restartOAuthAfterExit(logPath string) {
	a.oauthRestartMu.Lock()
	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)
	kept := a.oauthRestarts[:0]
	for _, at := range a.oauthRestarts {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	a.oauthRestarts = kept
	if len(kept) >= 5 {
		a.oauthRestartMu.Unlock()
		a.postEvent(errNotice{text: "OAuth helper is not restarting; see " + logPath})
		return
	}
	delays := []time.Duration{2 * time.Second, 5 * time.Second, 15 * time.Second, 60 * time.Second, 60 * time.Second}
	delay := delays[len(kept)]
	a.oauthRestarts = append(a.oauthRestarts, now)
	a.oauthRestartMu.Unlock()
	safe.Go(func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-a.done:
			return
		case <-timer.C:
		}
		if a.closing || a.oauth.State().Accounts == 0 || !a.oauth.Installed() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		if err := a.ensureOAuthServer(ctx, nil); err != nil {
			a.postEvent(errNotice{text: "OAuth helper is not responding; see " + logPath})
		}
	}, a.reportPanic)
}

func (a *App) restartOAuthHelper() {
	safe.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		if a.daemons != nil {
			a.daemons.Stop("oauth")
		}
		if err := a.ensureOAuthServer(ctx, nil); err != nil {
			log := ""
			if a.daemons != nil {
				log = "; see " + a.daemons.LogPath("oauth")
			}
			a.postEvent(errNotice{text: "OAuth helper is not responding" + log})
			return
		}
		a.refreshAccounts()
	}, a.reportPanic)
}

func (a *App) helperStopped(name string, err error, log string) bool {
	if name == "oauth" {
		a.postEvent(oauthHelperStatus{ready: false})
	}
	if name != "oauth" || a.oauth == nil || a.oauth.State().Accounts == 0 {
		return false
	}
	a.restartOAuthAfterExit(log)
	return true
}
