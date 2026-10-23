package tui

import (
	"strings"
	"testing"

	"meno/internal/render"
)

func TestTimelineNoticeSavedAndLiveAnchors(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	s.SetSize(80, 40)
	a.st.Session.turns = []render.Turn{{ID: "saved", Role: render.RoleAssistant, Title: "Saved title", Code: "echo saved", Output: "Saved output"}}
	a.notify("Notice text")
	a.st.LiveBase = a.st.Session.Len()
	a.st.LiveCalls = []liveCall{{ID: "live", Title: "Live title", Code: "echo live", Running: true}}
	lines := a.transcriptWithSelection(80)
	for y, line := range lines {
		render.DrawLine(render.Canvas{S: s}, 0, y, 80, line)
	}
	for _, needle := range []string{"Saved title", "echo saved", "Saved output", "Notice text", "Live title", "echo live"} {
		found := false
		for y := range lines {
			row := visualRow(s, y)
			if at := strings.Index(row, needle); at >= 0 {
				found = true
				if render.TextWidth(row[:at]) != 4 {
					t.Errorf("%q has wrong anchor: %q", needle, row)
				}
			}
		}
		if !found {
			t.Errorf("missing %q", needle)
		}
	}
}
