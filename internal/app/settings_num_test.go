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

func TestSettingsNumbersForThinkingAndTimeout(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "bash_timeout")
	a.draw()
	if text := screenText(s); !strings.Contains(text, "120 sec") || !strings.Contains(text, "off") || !strings.Contains(text, "model") {
		t.Fatalf("timeout choices missing:\n%s", text)
	}
	a.onKey(key('+'))
	if a.st.Settings.BashTimeout != "150" {
		t.Fatalf("timeout %s", a.st.Settings.BashTimeout)
	}
	a.st.Sel = settingIdx(t, "thinking")
	a.onKey(key('-'))
	if a.st.Settings.ThinkLines != 4 || a.st.Settings.BashLines != 5 {
		t.Fatalf("thinking lines %d, bash lines %d", a.st.Settings.ThinkLines, a.st.Settings.BashLines)
	}
	a.draw()
	if text := screenText(s); !strings.Contains(text, "lines 4") {
		t.Fatalf("thinking lines not shown:\n%s", text)
	}
	a.st.Sel = settingIdx(t, "bash_timeout")
	a.draw()
	if text := screenText(s); !strings.Contains(text, "150 sec") {
		t.Fatalf("timeout seconds not shown:\n%s", text)
	}
	// Cycling reaches off, model, and seconds; the numeric value is retained.
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Settings.BashTimeout != "off" {
		t.Fatalf("cycle: %q", a.st.Settings.BashTimeout)
	}
	a.onKey(keyK(tcell.KeyRight))
	if !a.st.Settings.BashModelTimeout() {
		t.Fatal("model mode not reachable")
	}
	if got := config.Choices("bash_timeout"); len(got) != 3 || got[0] != "seconds" || got[1] != "off" || got[2] != "model" {
		t.Fatalf("timeout choices: %v", got)
	}
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Settings.BashTimeout != "150" {
		t.Fatalf("seconds mode lost value: %s", a.st.Settings.BashTimeout)
	}
	// A number key on a setting without numeric seconds does nothing.
	a.onKey(keyK(tcell.KeyRight))
	a.onKey(key('+'))
	if a.st.Settings.BashTimeout != "off" {
		t.Fatalf("+ changed a non-numeric timeout mode: %s", a.st.Settings.BashTimeout)
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
