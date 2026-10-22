package engine

import (
	"context"
	"strings"
	"testing"

	"meno/internal/api"
	"meno/internal/llm"
)

func bashCall(id, cmd string) llm.Event {
	return llm.Event{Kind: llm.EvTool, CallID: id, Name: "bash", Code: cmd, Arguments: `{"command":` + quote(cmd) + `,"title":"t"}`}
}

func quote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\"" }

func TestBashToolRunsAFreshShell(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{
		{bashCall("c1", "echo hello-bash; exit 3"), {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	if _, err := e.Message(context.Background(), id, api.MessageRequest{Text: "go", ClientID: "m"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, e, id, func(s *runState) bool { return !s.running && s.session.Len() >= 3 })
	s, _ := e.load(context.Background(), id)
	var out string
	for _, tr := range s.session.Snapshot() {
		if tr.CallID == "c1" {
			out = tr.Output
		}
	}
	if !strings.Contains(out, "hello-bash") || !strings.Contains(out, "[exit 3]") {
		t.Fatalf("output %q", out)
	}
	if s.session.Tool != "bash" {
		t.Fatalf("tool %q", s.session.Tool)
	}
}

func TestBashCallSeesMenoEnvironment(t *testing.T) {
	f := &fakeProvider{steps: [][]llm.Event{
		{bashCall("c2", "echo $MENO_SESSION:$MENO_CALL:$MENO_TOKEN"), {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}}
	e, _ := fixture(t, f)
	id := newSession(t, e)
	_, _ = e.Message(context.Background(), id, api.MessageRequest{Text: "go", ClientID: "m"})
	waitFor(t, e, id, func(s *runState) bool { return !s.running && s.session.Len() >= 3 })
	s, _ := e.load(context.Background(), id)
	var out string
	for _, tr := range s.session.Snapshot() {
		if tr.CallID == "c2" {
			out = tr.Output
		}
	}
	if !strings.Contains(out, id+":c2:"+s.token.value) {
		t.Fatalf("env missing in %q", out)
	}
}
