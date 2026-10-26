package tui

import (
	"github.com/gdamore/tcell/v2"
	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/render"
	"strings"
	"testing"
)

func TestEveryPanelAnchorsAboveComposer(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}, {40, 12}, {20, 5}} {
		a, s := visualFixture(t, size[0], size[1])
		a.setInput([]rune("preserved draft"))
		a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "existing history"})
		for o := OverModel; o <= OverTasks; o++ {
			a.openOverlay(o)
			a.draw()
			p := a.panelBounds(size[0], size[1])
			b := a.panelBox(size[0], size[1], 0, 0)
			if p.Y+p.Height != size[1]-1-a.composerHeightFor(size[0]) || b.x != min(render.Gutter, (size[0]-1)/2) {
				t.Fatalf("unanchored %v %d: %+v", size, o, p)
			}
			a.closeOverlay()
			if string(a.st.Input) != "preserved draft" {
				t.Fatal("panel overwrote draft")
			}
		}
		s.Fini()
	}
}

func TestPanelSearchUsesComposerWithoutEditingDraft(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.setInput([]rune("keep draft"))
	cursor := a.st.Cursor
	a.runCommand("model gpt")
	a.draw()
	if a.st.Query.String() != "gpt" || !strings.Contains(screenText(s), "/model gpt") {
		t.Fatal("model query not shown in composer")
	}
	a.onKey(key('x'))
	a.onKey(keyK(tcell.KeyEsc))
	if string(a.st.Input) != "keep draft" || a.st.Cursor != cursor {
		t.Fatal("query leaked into draft")
	}
}
