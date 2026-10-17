package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

func TestLogTextRefreshesWhenContentChangesWithoutNewRows(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "first"})
	before := a.logText()
	a.st.Session.Replace([]agent.Turn{{Role: llm.RoleAssistant, Text: "other"}})
	after := a.logText()
	if len(before) != len(after) {
		t.Fatal("test requires an unchanged row count")
	}
	var text strings.Builder
	for _, line := range after {
		text.WriteString(string(line))
	}
	if !strings.Contains(text.String(), "other") || strings.Contains(text.String(), "first") {
		t.Fatalf("stale timeline text: %s", text.String())
	}
}

func TestLiveTextAfterEarlierAssistantTurn(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "earlier answer"})
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "next question"})
	a.onAgent(agent.Ev{Kind: agent.EvText, Text: "new answer", Turns: 2})
	text, _ := transcriptText(a)
	if !strings.Contains(text, "new answer") {
		t.Fatalf("live text hidden by earlier response: %s", text)
	}
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "new answer"})
	text, _ = transcriptText(a)
	if strings.Count(text, "new answer") != 1 {
		t.Fatalf("saved response duplicated: %s", text)
	}
}

func TestVisualTimelineUsesDocumentSelection(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "select this"})
	a.st.Mode, a.st.InLog = ui.ModeVisual, true
	a.st.AnchorLog, a.st.VKind = true, 'V'
	a.draw()
	if !strings.Contains(screenText(s), "select this") {
		t.Fatal("timeline disappeared in visual mode")
	}
	if strings.Contains(a.visualHint(), "r run") {
		t.Fatal("obsolete turn-selection hint")
	}
}
