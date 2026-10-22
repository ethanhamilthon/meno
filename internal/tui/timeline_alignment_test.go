package tui

import (
	"strings"
	"testing"

	"meno/internal/content"
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

func TestWrappedToolQuoteExcludesLayoutButPreservesSourceIndent(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	s.SetSize(40, 40)
	code := "  echo " + strings.Repeat("x", 50)
	output := "  output " + strings.Repeat("y", 50)
	a.st.Session.turns = []render.Turn{{ID: "saved", Role: render.RoleAssistant, Code: code, Output: output}}
	for part, original := range map[string]string{content.PartCode: code, content.PartOutput: output} {
		lines := a.transcriptWithSelection(40)
		first, last := -1, -1
		var expected []string
		for i, line := range lines {
			if line.Source == nil || line.Source.Part != part {
				continue
			}
			if first < 0 {
				first = i
			}
			last = i
			expected = append(expected, string([]rune(line.Text())[line.Source.Skip:]))
		}
		ref, err := a.selectionReference(selectRows(a, first, last))
		if err != nil {
			t.Fatal(err)
		}
		if len(ref.Segments) != 1 || ref.Segments[0].Quote != strings.Join(expected, "\n") {
			t.Fatalf("wrong quote: %+v", ref)
		}
		if !strings.HasPrefix(ref.Segments[0].Quote, "  ") || strings.Join(expected, "") != original {
			t.Fatalf("source whitespace lost: %q", ref.Segments[0].Quote)
		}
	}
}
