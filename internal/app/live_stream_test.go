package app

import (
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

func transcriptText(a *App) (string, bool) {
	var b strings.Builder
	running := false
	for _, l := range a.transcriptWithSelection(80) {
		for _, s := range l.Segs {
			b.WriteString(s.Text)
		}
		running = running || l.BG == ui.BgJSRun
		b.WriteByte('\n')
	}
	return b.String(), running
}

func TestLiveStreamingToolCallThinkingAndText(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Settings.Thinking = "on"
	a.onAgent(agent.Ev{Kind: agent.EvThink, Thinking: "pondering"})
	a.onAgent(agent.Ev{Kind: agent.EvText, Text: "let me look"})
	a.onAgent(agent.Ev{Kind: agent.EvCode, CallID: "c1", Code: "const x = 1"})
	out, running := transcriptText(a)
	for _, want := range []string{"pondering", "let me look", "const x = 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if !running {
		t.Fatal("streaming code must use the live background")
	}
	a.onAgent(agent.Ev{Kind: agent.EvCode, CallID: "c1", Code: "const x = 1;\nconsole.log(x)"})
	if out, _ := transcriptText(a); !strings.Contains(out, "console.log(x)") {
		t.Fatal("code delta not shown")
	}
	a.onAgent(agent.Ev{Kind: agent.EvJSStart, CallID: "c1", Code: "const x = 1;\nconsole.log(x)"})
	if len(a.st.LiveCalls) != 1 || !a.st.LiveCalls[0].Running {
		t.Fatalf("%+v", a.st.LiveCalls)
	}
	// The finished call lands in the session first, then JSEnd arrives.
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "let me look", CallID: "c1", Code: "const x = 1;\nconsole.log(x)", Output: "1"})
	out, running = transcriptText(a)
	if strings.Count(out, "console.log(x)") != 1 || strings.Count(out, "let me look") != 1 || running {
		t.Fatalf("duplicated live and saved turn:\n%s", out)
	}
	a.onAgent(agent.Ev{Kind: agent.EvJSEnd, CallID: "c1"})
	if len(a.st.LiveCalls) != 0 || a.st.LiveText != "" || a.st.LiveThink != "" {
		t.Fatal("live state not cleared")
	}
}
