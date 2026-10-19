package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/cliproxy"
	"meno/internal/llm"
)

func TestLoginListOffersEightWaysToAdd(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Providers = nil
	rows := a.loginRows()
	if len(rows) != 8 {
		t.Fatalf("%d rows", len(rows))
	}
	if rows[0].kind != rowAddAPI || rows[0].proto != llm.ProtoResponses || rows[1].proto != llm.ProtoChat {
		t.Fatalf("the two API choices come first: %+v", rows[:2])
	}
	for i, s := range cliproxy.Services {
		if r := rows[2+i]; r.kind != rowAddOAuth || r.svc.ID != s.ID {
			t.Fatalf("row %d: %+v", 2+i, r)
		}
	}
	a.st.Providers = []llm.Provider{{Name: "mine", BaseURL: "http://x", Protocol: llm.ProtoChat}}
	a.st.Accounts = []cliproxy.Account{{File: "f", Label: "me@x"}}
	rows = a.loginRows()
	if len(rows) != 10 || rows[0].kind != rowAPI || rows[1].kind != rowOAuth {
		t.Fatalf("existing entries come before the choices: %+v", rows)
	}
}

func TestStartWithoutProviderLocksTheLoginList(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Providers = nil
	a.enforceProvider()
	if a.st.Over != OverLogin || !a.st.Locked {
		t.Fatalf("over=%v locked=%v", a.st.Over, a.st.Locked)
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverLogin {
		t.Fatal("esc must not close the list while it is locked")
	}
	a.runCommand("model")
	a.onKey(keyK(tcell.KeyEsc))
}

func TestLockedLoginOpensTheFormAndUnlocksWithAWorkingProvider(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Providers = nil
	a.enforceProvider()
	a.st.Sel = 1 // API key · Chat Completions
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Over != OverProviderEdit || a.st.EditIdx != -1 || a.st.EditProv.Protocol != llm.ProtoChat {
		t.Fatalf("over=%v edit=%+v", a.st.Over, a.st.EditProv)
	}
	fillField(a, 0, "mine")
	fillField(a, 1, "http://example.test/v1")
	if !a.st.Locked {
		t.Fatal("a provider without a key is not working yet")
	}
	fillField(a, 2, "secret")
	if a.st.Locked || a.needProvider() {
		t.Fatalf("a provider with a key unlocks: locked=%v", a.st.Locked)
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverLogin {
		t.Fatalf("esc goes back to the list, got %v", a.st.Over)
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverNone {
		t.Fatal("unlocked: esc closes the list")
	}
}

func TestSwitchingAProviderOffHidesItsModels(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Providers = []llm.Provider{{Name: "p", BaseURL: "http://127.0.0.1:1", Protocol: llm.ProtoChat}}
	a.st.Models["p"] = []llm.Model{{ID: "m", Provider: "p"}}
	a.st.Settings.Provider, a.st.Settings.Model = "p", "m"
	if len(a.allModels()) != 1 {
		t.Fatal("setup")
	}
	if _, ok := a.currentProvider(); !ok {
		t.Fatal("setup: provider selected")
	}
	a.openOverlay(OverLogin)
	a.onKey(key(' '))
	if !a.st.Providers[0].Disabled || len(a.allModels()) != 0 {
		t.Fatalf("off: %+v models %d", a.st.Providers[0], len(a.allModels()))
	}
	if _, ok := a.currentProvider(); ok {
		t.Fatal("a provider that is off must not serve requests")
	}
	saved, _ := a.store.LoadProviders()
	if len(saved) != 1 || !saved[0].Disabled {
		t.Fatalf("the switch must persist: %+v", saved)
	}
	a.onKey(key(' '))
	if a.st.Providers[0].Disabled {
		t.Fatal("on again")
	}
}

func TestRemovingAProviderNeedsASecondPress(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Providers = []llm.Provider{{Name: "p", BaseURL: "http://127.0.0.1:1", Protocol: llm.ProtoChat}}
	a.openOverlay(OverLogin)
	a.onKey(key('d'))
	if len(a.st.Providers) != 1 {
		t.Fatal("one press must not remove")
	}
	a.onKey(key('d'))
	if len(a.st.Providers) != 0 {
		t.Fatalf("second press removes: %+v", a.st.Providers)
	}
}

func TestOAuthLoginFlowAddsAnAccountAndTheProvider(t *testing.T) {
	a, scr, h, opened := loginApp(t)
	defer scr.Fini()
	a.st.Providers = nil
	a.enforceProvider()
	a.st.Sel = 2 // Claude
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Over != OverLoginFlow {
		t.Fatalf("over=%v", a.st.Over)
	}
	pump(t, a, func() bool {
		if a.st.Flow.Err != "" {
			t.Fatalf("flow error: %s steps %v", a.st.Flow.Err, a.st.Flow.Steps)
		}
		return len(a.st.Accounts) == 1
	})
	if a.st.Over != OverNone {
		t.Fatalf("an account unlocks the list and closes it, over=%v", a.st.Over)
	}
	if len(*opened) != 1 || (*opened)[0] != "https://claude.ai/oauth/authorize?x=1" {
		t.Fatalf("browser: %v", *opened)
	}
	var managed *llm.Provider
	for i := range a.st.Providers {
		if a.st.Providers[i].Managed() {
			managed = &a.st.Providers[i]
		}
	}
	st := a.oauth.State()
	if managed == nil || managed.Name != llm.OAuthName || managed.BaseURL != a.oauth.APIURL() || managed.APIKey != st.ClientKey || managed.Protocol != llm.ProtoChat || managed.Run == "" {
		t.Fatalf("managed provider: %+v", managed)
	}
	if !st.HasAccounts {
		t.Fatal("the state must remember that accounts exist")
	}
	if a.st.Locked {
		t.Fatal("an account unlocks")
	}
	saved, _ := a.store.LoadProviders()
	if len(saved) != 0 {
		t.Fatalf("the OAuth provider is derived and must not be saved: %+v", saved)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.accounts) != 1 {
		t.Fatal("account on the helper")
	}
}

func TestDeviceLoginShowsTheCode(t *testing.T) {
	a, scr, _, opened := loginApp(t)
	defer scr.Fini()
	a.startLogin(cliproxy.Services[3]) // xAI
	pump(t, a, func() bool { return a.st.Flow.Login != nil })
	if !a.st.Flow.Login.Device || a.st.Flow.Login.UserCode != "AB-12" || len(*opened) != 1 {
		t.Fatalf("%+v opened %v", a.st.Flow.Login, *opened)
	}
	a.draw()
	if out := screenText(scr); !strings.Contains(out, "AB-12") || !strings.Contains(out, "xAI") {
		t.Fatalf("the code must be on screen:\n%s", out)
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverLogin {
		t.Fatal("esc leaves the login")
	}
}

func TestAccountToggleAndLogoutGoThroughTheHelper(t *testing.T) {
	a, scr, h, _ := loginApp(t)
	defer scr.Fini()
	h.accounts = []map[string]any{{"name": "claude-1.json", "provider": "claude", "email": "me@example.com"}}
	a.openOverlay(OverLogin)
	a.refreshAccounts()
	pump(t, a, func() bool { return len(a.st.Accounts) == 1 })
	if !a.hasManaged() {
		t.Fatal("an enabled account makes the OAuth provider")
	}
	a.st.Sel = a.loginRowIndex(rowOAuth)
	a.onKey(key(' '))
	pump(t, a, func() bool { return len(a.st.Accounts) == 1 && a.st.Accounts[0].Disabled })
	if a.hasManaged() {
		t.Fatal("with every account off there is no OAuth provider")
	}
	a.onKey(key('d'))
	a.onKey(key('d'))
	pump(t, a, func() bool { return len(a.st.Accounts) == 0 })
}

func (a *App) loginRowIndex(kind rowKind) int {
	for i, r := range a.loginRows() {
		if r.kind == kind {
			return i
		}
	}
	return -1
}

func TestPastingTheFinalAddressReachesTheHelper(t *testing.T) {
	a, scr, h, _ := loginApp(t)
	defer scr.Fini()
	h.okAfter = 1 << 30 // the browser never returns by itself
	a.startLogin(cliproxy.Services[0])
	pump(t, a, func() bool { return a.st.Flow.Login != nil })
	a.onKey(key('p'))
	if !a.st.Editing {
		t.Fatal("p opens the field for the address")
	}
	addr := "http://localhost:54545/callback?code=c&state=st-1"
	for _, r := range addr {
		a.onKey(key(r))
	}
	a.onKey(keyK(tcell.KeyEnter))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		n := len(h.callback)
		h.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.callback) != 1 || h.callback[0] != addr || a.st.Editing {
		t.Fatalf("callback %v editing=%v", h.callback, a.st.Editing)
	}
	a.cancelFlow()
}
