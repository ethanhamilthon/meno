package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
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

func TestSettingsViewRowCycles(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.openOverlay(OverSettings)
	a.st.Sel = settingIdx(t, "view")
	before := a.st.Settings.View
	a.onKey(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	a.draw()
	if a.st.Settings.View == before {
		t.Fatalf("view did not change: %q", a.st.Settings.View)
	}
}

func TestViewModes(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	think := "one\n\ntwo\n\nthree"
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: "answer", Replay: &llm.Msg{Thought: think}})
	a.st.Settings.View = "full"
	if text := transcriptText(a); !strings.Contains(text, "one") || !strings.Contains(text, "three") || !strings.Contains(text, "answer") {
		t.Fatalf("full:\n%s", text)
	}
	for _, view := range []string{"bash_only", "compact"} {
		a.st.Settings.View = view
		if text := transcriptText(a); strings.Contains(text, "three") || strings.Contains(text, "∴") || !strings.Contains(text, "answer") {
			t.Fatalf("%s:\n%s", view, text)
		}
	}
}
