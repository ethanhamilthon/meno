package tui

import (
	"strings"
	"testing"

	"meno/internal/agent"
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

func TestSettingsNumbersForThinking(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "thinking")
	a.onKey(key('-'))
	if a.st.Settings.ThinkLines != 4 || a.st.Settings.BashLines != 5 {
		t.Fatalf("thinking lines %d, bash lines %d", a.st.Settings.ThinkLines, a.st.Settings.BashLines)
	}
	a.draw()
	if text := screenText(s); !strings.Contains(text, "lines 4") {
		t.Fatalf("thinking lines not shown:\n%s", text)
	}
}

func TestThinkingModes(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	think := "one\n\ntwo\n\nthree\n\nfour\n\nfive"
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "answer", Replay: &llm.Msg{Thought: think}})
	a.st.Settings.Thinking = "all"
	if text := transcriptText(a); !strings.Contains(text, "one") || !strings.Contains(text, "five") || strings.Contains(text, "earlier") {
		t.Fatalf("all:\n%s", text)
	}
	a.st.Settings.Thinking, a.st.Settings.ThinkLines = "lines", 3
	text := transcriptText(a)
	if strings.Contains(text, "one") || !strings.Contains(text, "five") || !strings.Contains(text, "earlier") || !strings.Contains(text, "answer") {
		t.Fatalf("lines:\n%s", text)
	}
	a.st.Settings.Thinking = "off"
	if text := transcriptText(a); strings.Contains(text, "five") || strings.Contains(text, "∴") || !strings.Contains(text, "answer") {
		t.Fatalf("off:\n%s", text)
	}
}
