package app

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"meno/internal/cliproxy"
)

func TestAccountsErrorShowsWithoutManagedProvider(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	a.oauth.Update(func(s *cliproxy.State) { s.Accounts, s.AccountsKnown = 1, true })
	a.st.AccountsErr = "OAuth helper is not responding"
	rows := a.loginRows()
	if rows[len(rows)-1].kind != rowHelperRestart {
		t.Fatal("helper restart row is not visible")
	}
	a.openOverlay(OverLogin)
	a.draw()
	if !strings.Contains(screenText(scr), "helper not responding") {
		t.Fatal("helper error is not visible in :login")
	}
}

func TestAccountNoteAppearsInLoginRow(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	a.st.Accounts = []cliproxy.Account{{File: "disabled.json", Label: "person", Note: "invalid grant"}}
	row := a.loginItem(loginRow{kind: rowOAuth, i: 0})
	if !strings.Contains(row.detail, "needs a new login") {
		t.Fatalf("detail %q", row.detail)
	}
}

func TestRestartRowStartsHelperAndRefreshesAccounts(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	var started atomic.Int32
	a.helperStart = func(context.Context) error { started.Add(1); return nil }
	a.helperReadyFn = func(context.Context) bool { return false }
	a.st.AccountsErr = "OAuth helper is not responding"
	a.openOverlay(OverLogin)
	a.loginEnter(loginRow{kind: rowHelperRestart})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && started.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	if got := started.Load(); got != 1 {
		t.Fatalf("helper starts=%d", got)
	}
}

func TestWithHelperStartsAndCallsOnce(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	var starts, calls int
	a.helperReadyFn = func(context.Context) bool { return false }
	a.helperStart = func(context.Context) error { starts++; return nil }
	err := a.withHelper(context.Background(), func(context.Context) error { calls++; return nil })
	if err != nil || starts != 1 || calls != 1 {
		t.Fatalf("err=%v starts=%d calls=%d", err, starts, calls)
	}
}

func TestWithHelperRetriesOnceAfterHelperDown(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	var starts, calls int
	a.helperReadyFn = func(context.Context) bool { return true }
	a.helperStart = func(context.Context) error { starts++; return nil }
	err := a.withHelper(context.Background(), func(context.Context) error {
		calls++
		if calls == 1 {
			return cliproxy.ErrHelperDown
		}
		return nil
	})
	if err != nil || starts != 1 || calls != 2 {
		t.Fatalf("err=%v starts=%d calls=%d", err, starts, calls)
	}
}

func TestWithHelperDoesNotRetryOtherErrors(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	var calls int
	a.helperReadyFn = func(context.Context) bool { return true }
	err := a.withHelper(context.Background(), func(context.Context) error { calls++; return errors.New("rejected") })
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestOAuthExitSchedulesDelayedRestart(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	a.oauth.Update(func(s *cliproxy.State) { s.Accounts = 1; s.AccountsKnown = true })
	if !a.helperStopped("oauth", errors.New("exit"), "oauth.log") {
		t.Fatal("oauth exit was not handled")
	}
	if len(a.oauthRestarts) != 1 || time.Since(a.oauthRestarts[0]) > time.Second {
		t.Fatalf("restart not scheduled promptly: %v", a.oauthRestarts)
	}
}

func TestOAuthRestartBackoffStopsAfterFiveAttempts(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	a.oauth.Update(func(s *cliproxy.State) { s.Accounts = 1 })
	for i := 0; i < 5; i++ {
		a.restartOAuthAfterExit("oauth.log")
	}
	if len(a.oauthRestarts) != 5 {
		t.Fatalf("scheduled %d restarts", len(a.oauthRestarts))
	}
	a.restartOAuthAfterExit("oauth.log")
	if len(a.oauthRestarts) != 5 {
		t.Fatalf("restart limit ignored: %d", len(a.oauthRestarts))
	}
}

func TestChooseOAuthPortRefreshesManagedProvider(t *testing.T) {
	a, scr, _, _ := loginApp(t)
	defer scr.Fini()
	a.oauth.Update(func(s *cliproxy.State) { s.Accounts = 1; s.HasAccounts = true })
	a.setManaged(true)
	oldURL := a.st.Providers[len(a.st.Providers)-1].BaseURL
	if !a.chooseOAuthPort() {
		t.Fatal("the fake server must occupy its configured port")
	}
	provider := a.st.Providers[len(a.st.Providers)-1]
	if provider.BaseURL == oldURL || provider.BaseURL != a.oauth.APIURL() {
		t.Fatalf("provider URL %q; helper URL %q", provider.BaseURL, a.oauth.APIURL())
	}
}
