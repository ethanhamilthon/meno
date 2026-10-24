package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

func TestLoginOffersOnlyConfiguredProvidersAndAPIProtocols(t *testing.T) {
	a, _ := visualFixture(t, 90, 30)
	a.st.Providers = []llm.Provider{{Name: "custom", BaseURL: "http://127.0.0.1:8317/v1", Protocol: llm.ProtoResponses}, {Name: "disabled", Disabled: true}}
	rows := a.loginRows()
	if len(rows) != 4 || rows[0].kind != rowAPI || rows[1].kind != rowAPI || rows[2].proto != llm.ProtoResponses || rows[3].proto != llm.ProtoChat {
		t.Fatalf("rows: %+v", rows)
	}
	if a.needProvider() {
		t.Fatal("local custom provider should remain usable without a key")
	}
	a.openLogin()
	if a.st.Over != OverLogin {
		t.Fatal("/login should open provider list")
	}
	a.loginRemove(rows[2])
	if len(a.st.Providers) != 2 || a.st.LoginDel != "" {
		t.Fatal("add row must not remove a provider")
	}
}

func TestLoginFirstRunLockAndAPIProviderUnlock(t *testing.T) {
	a, _ := visualFixture(t, 90, 30)
	a.st.Providers = nil
	a.enforceProvider()
	if !a.st.Locked || a.st.Over != OverLogin {
		t.Fatal("empty configuration must open locked /login")
	}
	a.onOverlayKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.st.Over != OverLogin {
		t.Fatal("escape closed first-run login")
	}
	a.st.Providers = []llm.Provider{{Name: "api", BaseURL: "https://example.test/v1", APIKey: "key", Protocol: llm.ProtoResponses}}
	a.afterProvidersChanged()
	if a.st.Locked || a.st.Over != OverNone {
		t.Fatal("API provider did not unlock login")
	}
}
