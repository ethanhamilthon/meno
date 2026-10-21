package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestTranscriptBaseTextSharesScreenColumn(t *testing.T) {
	for _, w := range []int{40, 80, 160} {
		turns := []Turn{
			{Role: RoleUser, Text: "User text"},
			{Role: RoleAssistant, Thinking: "Reasoning text", Text: "Assistant text"},
			{Role: RoleAssistant, Title: "Inspect files", Code: "echo source", Output: "Output text"},
			{Role: RoleAssistant, Code: "false", Err: "Failure text"},
			{Role: RoleUser, Kind: KindView, Text: "Viewed image", Attachments: []Attachment{{N: 1, W: 10, H: 10, Mime: "image/png"}}},
			{Role: RoleAssistant, Run: &RunStats{Status: "stop"}},
		}
		lines := BuildLines(turns, w, JSOpts{})
		s := alignmentScreen(t, w, len(lines))
		for y, l := range lines {
			DrawLine(Canvas{S: s}, 0, y, w, l)
		}
		for _, needle := range []string{"User text", "Reasoning text", "Assistant text", "Inspect files", "echo source", "Output text", "error:", "Viewed image", "Image #1", "stopped", "↑0"} {
			found := false
			for y := range lines {
				row := alignmentRow(s, w, y)
				if at := strings.Index(row, needle); at >= 0 {
					found = true
					if x := TextWidth(row[:at]); x != 4 {
						t.Errorf("width %d: %q starts at %d, want 4", w, needle, x)
					}
				}
			}
			if !found {
				t.Errorf("width %d: missing %q", w, needle)
			}
		}
	}
}

func TestTranscriptWrappingDoesNotClipSourceOrOutput(t *testing.T) {
	for _, w := range []int{20, 40, 80} {
		code := "  echo " + strings.Repeat("x", w*2)
		output := "  " + strings.Repeat("y", w*2)
		lines := BuildLines([]Turn{{ID: "turn", Role: RoleAssistant, Code: code, Output: output}}, w, JSOpts{})
		s := alignmentScreen(t, w, len(lines))
		got := map[string]string{}
		for y, l := range lines {
			DrawLine(Canvas{S: s}, 0, y, w, l)
			if l.Source == nil {
				continue
			}
			if l.Width() > w-2*Gutter {
				t.Errorf("width %d: row would clip: %q", w, l.Text())
			}
			got[l.Source.Part] += string([]rune(l.Text())[l.Source.Skip:])
		}
		if got["code"] != code || got["output"] != output {
			t.Fatalf("width %d lost source/output: %q", w, got)
		}
	}
}

func alignmentScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	s.SetSize(w, h)
	t.Cleanup(s.Fini)
	return s
}

func alignmentRow(s tcell.SimulationScreen, w, y int) string {
	var b strings.Builder
	for x := 0; x < w; x++ {
		r, _, _, cells := s.GetContent(x, y)
		if r == 0 {
			r = ' '
		}
		b.WriteRune(r)
		if cells > 1 {
			x += cells - 1
		}
	}
	return b.String()
}
