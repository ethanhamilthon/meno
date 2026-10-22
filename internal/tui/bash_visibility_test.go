package tui

import (
	"strings"
	"testing"
	"time"
)

func TestBashSourceVisibleOnlyAtExecution(t *testing.T) {
	a, _ := visualFixture(t, 100, 25)
	a.st.LiveText = "assistant stays visible"
	a.st.LiveThink = "thinking stays visible"
	a.st.LiveCalls = []liveCall{{ID: "partial", Code: "partial-secret-marker"}, {ID: "ready", Code: "echo complete-command", Running: true, Started: time.Now()}}
	lines := a.transcriptWithSelection(100)
	var text strings.Builder
	for _, line := range lines {
		text.WriteString(joinSegText(line.Segs))
		text.WriteByte('\n')
	}
	for _, want := range []string{"assistant stays visible", "thinking stays visible", "echo complete-command"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(text.String(), "partial-secret-marker") {
		t.Fatal("partial Bash leaked")
	}
}
