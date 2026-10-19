package app

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// The js state lasts for one answer: calls of the same run share it, and the
// next message from the user starts clean.
func TestE2EJSStateLastsForOneRequest(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "var kept = 7; kept"}},
		{{Kind: llm.EvTool, CallID: "c2", Code: "kept * 2"}},
		{{Kind: llm.EvText, Text: "first answer"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "c3", Code: "typeof kept"}},
		{{Kind: llm.EvText, Text: "second answer"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	ask := func(text string) {
		typeRunes(a, text)
		a.onKey(keyK(tcell.KeyEnter))
		pump(t, a, func() bool { return !a.st.Running })
	}
	ask("one")
	ask("two")
	outputs := map[string]string{}
	for _, tr := range a.st.Session.Snapshot() {
		if tr.CallID != "" {
			outputs[tr.CallID] = tr.Output
		}
	}
	if outputs["c2"] != "14" {
		t.Fatalf("a call in the same run must see the variable: %q", outputs["c2"])
	}
	if outputs["c3"] != "undefined" {
		t.Fatalf("a new request must start clean: %q", outputs["c3"])
	}
	if r := a.st.VM.Exec(context.Background(), "typeof kept"); r.Value != "undefined" {
		t.Fatalf("%+v", r)
	}
}
