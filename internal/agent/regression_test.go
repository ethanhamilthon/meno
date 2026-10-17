package agent

import (
	"context"
	"meno/internal/llm"
	"testing"
)

func TestCompactFailurePreservesHistory(t *testing.T) {
	for _, events := range [][]llm.Event{{}, {{Kind: llm.EvError, Text: "rate limited"}}} {
		s := NewSession(".", "m")
		s.Summary = "old summary"
		for i := 0; i < 10; i++ {
			s.Append(Turn{Role: llm.RoleUser, Text: "important"})
		}
		cl := &scripted{steps: [][]llm.Event{events}}
		if _, _, err := Compact(context.Background(), cl, llm.Provider{}, "m", s); err == nil {
			t.Fatal("failed compact accepted")
		}
		if s.Summary != "old summary" || len(s.Snapshot()) != 10 {
			t.Fatal("history destroyed")
		}
	}
}
func TestPartialStreamRetainedOnError(t *testing.T) {
	cl := &scripted{steps: [][]llm.Event{{{Kind: llm.EvText, Text: "partial answer"}, {Kind: llm.EvError, Text: "disconnected"}}}}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "q"); err == nil {
		t.Fatal("stream error ignored")
	}
	turns := s.Snapshot()
	if len(turns) != 2 || turns[1].Text != "partial answer" {
		t.Fatal("partial answer lost")
	}
}
