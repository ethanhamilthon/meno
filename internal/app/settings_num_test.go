package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
)

func settingIdx(t *testing.T, key string) int {
	for i, r := range settingRows {
		if r.key == key {
			return i
		}
	}
	t.Fatalf("no setting row %s", key)
	return 0
}

func TestEffortRoutingSettingsUI(t *testing.T) {
	a, screen := newTestApp(t)
	defer screen.Fini()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "effort_router")
	a.draw()
	if text := screenText(screen); !strings.Contains(text, "effort routing") || !strings.Contains(text, "off") || !strings.Contains(text, "auto") {
		t.Fatalf("routing setting missing: %s", text)
	}
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Settings.EffortRouter != "auto" {
		t.Fatalf("routing choice %q", a.st.Settings.EffortRouter)
	}
	a.st.Sel = settingIdx(t, "effort_router_model")
	a.onKey(keyK(tcell.KeyEnter))
	typeRunes(a, "router-small")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Settings.EffortRouterModel != "router-small" {
		t.Fatalf("router model %q", a.st.Settings.EffortRouterModel)
	}
	saved, err := config.NewStore(a.paths).LoadSettings()
	if err != nil || saved.EffortRouter != "auto" || saved.EffortRouterModel != "router-small" {
		t.Fatalf("settings not saved: %+v %v", saved, err)
	}
}

func TestSettingsNumbersForThinkingAndTimeout(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "js_timeout")
	a.draw()
	if text := screenText(s); !strings.Contains(text, "30 sec") || !strings.Contains(text, "off") || !strings.Contains(text, "model") {
		t.Fatalf("timeout choices missing:\n%s", text)
	}
	a.onKey(key('+'))
	if a.st.Settings.JSTimeoutSec != 35 {
		t.Fatalf("timeout %d", a.st.Settings.JSTimeoutSec)
	}
	a.st.Sel = settingIdx(t, "thinking")
	a.onKey(key('-'))
	if a.st.Settings.ThinkLines != 4 || a.st.Settings.JSLines != 5 {
		t.Fatalf("thinking lines %d, js lines %d", a.st.Settings.ThinkLines, a.st.Settings.JSLines)
	}
	a.draw()
	if text := screenText(s); !strings.Contains(text, "lines 4") {
		t.Fatalf("thinking lines not shown:\n%s", text)
	}
	a.st.Sel = settingIdx(t, "js_timeout")
	a.draw()
	if text := screenText(s); !strings.Contains(text, "35 sec") {
		t.Fatalf("timeout seconds not shown:\n%s", text)
	}
	// Cycling reaches off and model; the VM follows the setting.
	a.st.Sel = settingIdx(t, "js_timeout")
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Settings.JSTimeout != "off" {
		t.Fatalf("cycle: %q", a.st.Settings.JSTimeout)
	}
	a.onKey(keyK(tcell.KeyRight))
	if !a.st.Settings.ModelTimeout() {
		t.Fatal("model mode not reachable")
	}
	// A number key on a setting without a number does nothing.
	a.st.Sel = settingIdx(t, "js_timeout")
	a.onKey(key('+'))
	if a.st.Settings.JSTimeoutSec != 40 {
		t.Fatalf("+ still adjusts the stored seconds: %d", a.st.Settings.JSTimeoutSec)
	}
}

func TestSettingsWebSearchBackendAndKey(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "websearch")
	a.draw()
	if text := screenText(s); !strings.Contains(text, "Web") || !strings.Contains(text, "auto") {
		t.Fatalf("Web tab missing:\n%s", text)
	}
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Settings.WebSearch != "brave" {
		t.Fatalf("backend %q", a.st.Settings.WebSearch)
	}
	a.st.Sel = settingIdx(t, "websearch_key")
	a.onKey(keyK(tcell.KeyEnter))
	typeRunes(a, "brave-key")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Settings.WebSearchKey != "brave-key" {
		t.Fatalf("key %q", a.st.Settings.WebSearchKey)
	}
	saved, err := a.store.LoadSettings()
	if err != nil || saved.WebSearchKey != "brave-key" || saved.WebSearch != "brave" {
		t.Fatalf("not persisted: %+v %v", saved, err)
	}
	a.draw()
	if text := screenText(s); strings.Contains(text, "brave-key") || !strings.Contains(text, "••") {
		t.Fatalf("key not masked:\n%s", text)
	}
}

func TestThinkingModes(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	think := "one\n\ntwo\n\nthree\n\nfour\n\nfive"
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "answer", Replay: &llm.Msg{Thought: think}})
	a.st.Settings.Thinking = "all"
	if text, _ := transcriptText(a); !strings.Contains(text, "one") || !strings.Contains(text, "five") || strings.Contains(text, "earlier") {
		t.Fatalf("all:\n%s", text)
	}
	a.st.Settings.Thinking, a.st.Settings.ThinkLines = "lines", 3
	text, _ := transcriptText(a)
	if strings.Contains(text, "one") || !strings.Contains(text, "five") || !strings.Contains(text, "earlier") || !strings.Contains(text, "answer") {
		t.Fatalf("lines:\n%s", text)
	}
	a.st.Settings.Thinking = "off"
	if text, _ := transcriptText(a); strings.Contains(text, "five") || strings.Contains(text, "∴") || !strings.Contains(text, "answer") {
		t.Fatalf("off:\n%s", text)
	}
}
