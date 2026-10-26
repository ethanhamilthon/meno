package tui

import (
	"strings"
	"testing"
	"time"

	"meno/internal/api"
)

func TestCompactViewShowsOneLiveRow(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Settings.View = "compact"
	a.st.Running = true
	a.st.RunStart = time.Now().Add(-65 * time.Second)
	rt := a.st.Runtime
	a.applyWire(rt, wireEvent(api.EventThinkDelta, api.TextDeltaData{Text: "hmm"}))
	if text := timelineText(a); !strings.Contains(text, "thinking...") || !strings.Contains(text, "1m 05s") {
		t.Fatalf("thinking row:\n%s", text)
	}
	a.applyWire(rt, wireEvent(api.EventCallStart, api.CallStartData{CallID: "c1", Command: "sleep 3\necho hidden", Title: "Wait"}))
	text := timelineText(a)
	if !strings.Contains(text, "sleep 3 echo hidden") || strings.Contains(text, "thinking...") || strings.Contains(text, "Wait") {
		t.Fatalf("command row:\n%s", text)
	}
	a.applyWire(rt, wireEvent(api.EventCallEnd, api.CallEndData{CallID: "c1", Output: "OUTPUT-LINE", ElapsedMS: 700}))
	if text = timelineText(a); strings.Contains(text, "OUTPUT-LINE") || strings.Count(text, "1m 05s") != 1 {
		t.Fatalf("an ended call must stay out of the compact timeline:\n%s", text)
	}
	a.st.Running = false
	if text = timelineText(a); strings.Contains(text, "───") {
		t.Fatalf("the row must go when the turn ends:\n%s", text)
	}
}
