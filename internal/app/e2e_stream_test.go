package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// TestE2EStreamingFrames drives a whole turn and inspects every frame: thinking,
// text and the tool call code must appear while they stream, and nothing may be
// duplicated once the turn is persisted.
func TestE2EStreamingFrames(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{
			{Kind: llm.EvThinking, Text: "weighing options"},
			{Kind: llm.EvText, Text: "checking the sum"},
			{Kind: llm.EvToolDelta, CallID: "c1", Code: "echo total"},
			{Kind: llm.EvToolDelta, CallID: "c1", Code: "echo total = 20 + 22"},
			{Kind: llm.EvTool, CallID: "c1", Name: "bash", Code: "echo total = 20 + 22; echo 42", Arguments: `{"command":"echo total = 20 + 22; echo 42"}`},
		},
		{{Kind: llm.EvText, Text: "the answer is 42"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	typeRunes(a, "sum please")
	a.onKey(keyK(tcell.KeyEnter))

	seen := map[string]bool{}
	deadline := time.After(5 * time.Second)
	for a.st.Running {
		select {
		case msg := <-a.events:
			a.handle(msg)
			a.draw()
			frame := screenText(scr)
			for _, want := range []string{"weighing options", "checking the sum", "echo total", "20 + 22"} {
				if strings.Contains(frame, want) {
					seen[want] = true
				}
			}
			if strings.Count(frame, "checking the sum") > 1 {
				t.Fatalf("text duplicated in a frame:\n%s", frame)
			}
		case <-deadline:
			t.Fatal("timeout")
		}
	}
	for _, want := range []string{"weighing options", "checking the sum", "echo total", "20 + 22"} {
		if !seen[want] {
			t.Errorf("%q never appeared on screen while streaming", want)
		}
	}
	a.draw()
	final := screenText(scr)
	if !strings.Contains(final, "the answer is 42") || strings.Count(final, "echo total = 20 + 22; echo 42") != 1 {
		t.Fatalf("final frame:\n%s", final)
	}
	if len(a.st.LiveCalls) != 0 || a.st.LiveText != "" || a.st.LiveThink != "" {
		t.Fatal("live state leaked after the turn")
	}
}
