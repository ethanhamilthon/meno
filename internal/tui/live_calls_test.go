package tui

import (
	"strings"
	"testing"

	"meno/internal/api"
)

func savedCallTurn(id, callID, code, output string) api.Turn {
	return api.Turn{ID: id, Role: "assistant", ElapsedMS: 1000, Parts: []api.TurnPart{{Type: "call", Call: &api.CallState{CallID: callID, Command: code, Title: "Run echo", Output: output}}}}
}

func timelineText(a *App) string { return transcriptText(a) }

// A call that has ended stays on screen, with its output, until the saved turn
// that holds it arrives; then the saved turn replaces it without a duplicate.
func TestEndedCallStaysUntilItsSavedTurnArrives(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Running = true
	rt := a.st.Runtime
	a.applyWire(rt, wireEvent(api.EventCallStart, api.CallStartData{CallID: "c1", Command: "echo kept", Title: "Run echo"}))
	a.applyWire(rt, wireEvent(api.EventCallEnd, api.CallEndData{CallID: "c1", Output: "kept\n[exit 0]", ElapsedMS: 700}))
	if len(rt.LiveCalls) != 1 || !rt.LiveCalls[0].Ended || rt.LiveCalls[0].Output != "kept\n[exit 0]" || rt.LiveCalls[0].ElapsedMs != 700 {
		t.Fatalf("ended call not kept: %+v", rt.LiveCalls)
	}
	if text := timelineText(a); strings.Count(text, "echo kept") != 1 || !strings.Contains(text, "[exit 0]") {
		t.Fatalf("ended call is not drawn with its output:\n%s", text)
	}
	a.applyWire(rt, wireEvent(api.EventTurnAdded, api.TurnAddedData{Turn: savedCallTurn("t1", "c1", "echo kept", "kept\n[exit 0]")}))
	a.applyWire(rt, wireEvent(api.EventTextDelta, api.TextDeltaData{Text: "next"})) // any live change
	if len(rt.LiveCalls) != 0 {
		t.Fatalf("the ended call outlived its saved turn: %+v", rt.LiveCalls)
	}
	if text := timelineText(a); strings.Count(text, "echo kept") != 1 {
		t.Fatalf("the command is drawn %d times after the hand-over:\n%s", strings.Count(text, "echo kept"), text)
	}
}

// Several calls keep the order they were issued in, whichever ends first.
func TestEndedCallsKeepTheirOrder(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Running = true
	rt := a.st.Runtime
	for _, id := range []string{"c1", "c2", "c3"} {
		a.applyWire(rt, wireEvent(api.EventCallStart, api.CallStartData{CallID: id, Command: "echo " + id}))
	}
	a.applyWire(rt, wireEvent(api.EventCallEnd, api.CallEndData{CallID: "c3", Output: "3"}))
	a.applyWire(rt, wireEvent(api.EventCallEnd, api.CallEndData{CallID: "c1", Output: "1"}))
	var got []string
	for _, c := range rt.LiveCalls {
		got = append(got, c.ID)
	}
	if strings.Join(got, ",") != "c1,c2,c3" {
		t.Fatalf("order %v, want c1,c2,c3", got)
	}
}

// A call moved to a background task has no output of its own to keep.
func TestCallMovedToATaskIsNotKept(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Running = true
	rt := a.st.Runtime
	a.applyWire(rt, wireEvent(api.EventCallStart, api.CallStartData{CallID: "c1", Command: "sleep 100"}))
	a.applyWire(rt, wireEvent(api.EventCallEnd, api.CallEndData{CallID: "c1", Output: "moved", TaskID: "t9"}))
	if len(rt.LiveCalls) != 0 {
		t.Fatalf("%+v", rt.LiveCalls)
	}
}
