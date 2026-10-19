package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/cliproxy"
	"meno/internal/llm"
)

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
	if a.oauth.State().Accounts != 1 || a.oauth.State().HasAccounts {
		t.Fatalf("account state should retain disabled account: %+v", a.oauth.State())
	}
	if a.st.AccountsErr != "" {
		t.Fatalf("account action failed: %s", a.st.AccountsErr)
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
