package app

import (
	"meno/internal/ui"
	"testing"
)

func TestTranscriptToolsHaveNoExtraGap(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	turns := []ui.Turn{
		{Role: ui.RoleUser, Text: "do it"},
		{Role: ui.RoleAssistant, Code: "echo one", Output: "one"},
		{Role: ui.RoleAssistant, Code: "echo two", Output: "two"},
		{Role: ui.RoleAssistant, Text: "done"},
	}
	a.st.Session.turns = turns
	a.st.Notices = nil
	want := len(ui.BuildLines(turns, 80, a.jsOpts()))
	if got := len(a.transcriptWithSelection(80)); got != want {
		t.Fatalf("%d rows, want %d", got, want)
	}
}
