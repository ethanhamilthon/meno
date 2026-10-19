package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

func TestJSStateLastsForOneRunAndTheDescriptionIsSent(t *testing.T) {
	vm := jsvm.New(jsvm.Options{Root: t.TempDir(), Timeout: 5 * time.Second})
	cl := &timeoutClient{calls: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "const n = 41; n"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "c2", Code: "n + 1"}, {Kind: llm.EvDone}},
	}}
	r := &Runner{D: Deps{Retry: RetryPolicy{Attempts: 1}, Client: cl, VM: vm, ToolDesc: "DOC", Emit: func(Ev) {}}}
	s := NewSession(t.TempDir(), "m")
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	var out string
	for _, turn := range s.Snapshot() {
		if turn.CallID == "c2" {
			out = strings.TrimSpace(turn.Err + turn.Output)
		}
	}
	if out != "42" {
		t.Fatalf("the second call must see n from the first, got %q", out)
	}
	for i, req := range cl.reqs {
		if req.ToolDescription != "DOC" {
			t.Fatalf("request %d: tool description %q", i, req.ToolDescription)
		}
	}
	vm.Reset()
	if r := vm.Exec(context.Background(), "typeof n"); r.Value != "undefined" {
		t.Fatalf("Reset must drop the state: %+v", r)
	}
}
