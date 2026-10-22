package tui

import (
	"meno/internal/render"
	"testing"
)

func TestTranscriptToolsHaveNoExtraGap(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	turns := []render.Turn{
		{Role: render.RoleUser, Text: "do it"},
		{Role: render.RoleAssistant, Code: "echo one", Output: "one"},
		{Role: render.RoleAssistant, Code: "echo two", Output: "two"},
		{Role: render.RoleAssistant, Text: "done"},
	}
	a.st.Session.turns = turns
	a.st.Notices = nil
	want := len(render.BuildLines(turns, 80, a.jsOpts()))
	if got := len(a.transcriptWithSelection(80)); got != want {
		t.Fatalf("%d rows, want %d", got, want)
	}
}
