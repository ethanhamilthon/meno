package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestNoticesGoToTimelineNotInput(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.draw()
	a.runCommand("nope")
	a.draw()
	if out := screenText(s); !strings.Contains(out, "unknown command :nope") || !strings.Contains(out, "v0.1") {
		t.Fatalf("notice must sit over the welcome screen:\n%s", out)
	}
	if strings.Contains(visualRow(s, 27), "unknown command") {
		t.Fatal("notice leaked into the composer")
	}
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "hi"})
	a.notifyErr("first")
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "answer"})
	a.notify("second")
	out, _ := transcriptText(a)
	i, j, k := strings.Index(out, "hi"), strings.Index(out, "first"), strings.Index(out, "answer")
	if !(i < j && j < k && k < strings.Index(out, "second")) {
		t.Fatalf("order:\n%s", out)
	}
}
