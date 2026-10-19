package app

import (
	"testing"

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
	if len(rows) != 11 || rows[0].kind != rowAPI || rows[1].kind != rowOAuth || rows[10].kind != rowHelperRestart {
		t.Fatalf("existing entries and the helper action come before or after the choices: %+v", rows)
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
