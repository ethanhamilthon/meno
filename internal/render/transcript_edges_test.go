package render

import (
	"strings"
	"testing"
)

func TestTranscriptNarrowAndLimitedModes(t *testing.T) {
	turn := Turn{ID: "saved", Role: RoleAssistant, Title: strings.Repeat("界😀 title ", 15),
		Code: strings.Repeat("echo abcdef\n", 8), Output: strings.Repeat("output line\n", 8),
		Thinking: strings.Repeat("consider this carefully\n", 8)}
	for _, w := range []int{1, 4, 8, 20, 40} {
		for _, mode := range []ViewMode{ViewFull, ViewBashOnly, ViewCompact} {
			lines := BuildLines([]Turn{turn}, w, JSOpts{View: mode})
			s := alignmentScreen(t, w, len(lines))
			for y, line := range lines {
				DrawLine(Canvas{S: s}, 0, y, w, line)
			}
		}
	}
}

func TestMarkdownIndentationSurvives(t *testing.T) {
	rows := BuildLines([]Turn{{Role: RoleAssistant, Text: "- outer\n  - inner"}}, 80, JSOpts{})
	outer, inner := -1, -1
	for _, row := range rows {
		if at := strings.Index(row.Text(), "outer"); at >= 0 {
			outer = TextWidth(row.Text()[:at])
		}
		if at := strings.Index(row.Text(), "inner"); at >= 0 {
			inner = TextWidth(row.Text()[:at])
		}
	}
	if outer < 0 || inner <= outer {
		t.Fatalf("nested Markdown lost its indentation: outer %d inner %d", outer, inner)
	}
}

func TestTranscriptRightGutterAndTiming(t *testing.T) {
	for _, w := range []int{20, 40, 80} {
		turn := Turn{Role: RoleAssistant, Title: strings.Repeat("界", 50), Code: "echo ok", ElapsedMs: 3210}
		lines := BuildLines([]Turn{turn}, w, JSOpts{})
		s := alignmentScreen(t, w, len(lines))
		for y, line := range lines {
			DrawLine(Canvas{S: s}, 0, y, w, line)
			for x := w - Gutter; x < w; x++ {
				r, _, _, _ := s.GetContent(x, y)
				if r != ' ' && r != 0 {
					t.Fatalf("text in right gutter: width %d at %d: %c", w, x, r)
				}
			}
		}
		if !strings.Contains(alignmentRow(s, w, 0), "3.2s") {
			t.Fatalf("width %d: elapsed timing lost", w)
		}
	}
}
