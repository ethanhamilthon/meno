package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"meno/internal/llm"
)

// toolHeavy is a session whose weight is old tool output: n calls of size bytes
// each, then a new user message.
func toolHeavy(n, size int) *Session {
	s := NewSession(".", "m")
	s.Append(Turn{Role: llm.RoleUser, Text: "start"})
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("c%02d", i)
		s.Append(Turn{Role: llm.RoleAssistant, CallID: id, Code: "cat f", Output: strings.Repeat("x", size), ReplayGroup: "g" + id,
			Replay: &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: id, Name: "bash", Arguments: `{}`}}}})
	}
	s.Append(Turn{Role: llm.RoleUser, Text: "continue"})
	return s
}

var windowTwenty = AutoCompact{Window: 20000, Percent: 80, Keep: 8}

func TestElisionSpareTheSummary(t *testing.T) {
	cl := &sizedClient{}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.AutoCompact = windowTwenty
	r.D.AutoCompact.Elide = ElideOptions{Percent: 50, KeepRecent: 6, MinBytes: 1000, Head: 100, Tail: 100}
	s := toolHeavy(12, 6000) // 72 KB: over 80% of a 20k window, about 43 KB once old outputs are cut
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if cl.summaries != 0 || cl.normals != 1 {
		t.Fatalf("summaries %d, normal requests %d: elision should have been enough", cl.summaries, cl.normals)
	}
	if s.Summary != "" {
		t.Fatal("a summary was written")
	}
	texts := toolTexts(s)
	if !strings.Contains(texts[0], "omitted from the context") || strings.Contains(texts[len(texts)-1], "omitted from the context") {
		t.Fatalf("old output not cut or newest output cut")
	}
}

func TestWithoutElisionTheSummaryIsStillMade(t *testing.T) {
	cl := &sizedClient{}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.AutoCompact = windowTwenty // Elide.Percent is 0: off
	s := toolHeavy(12, 6000)
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if cl.summaries != 1 {
		t.Fatalf("summaries %d, want 1", cl.summaries)
	}
}

func TestElisionThenSummaryWhenElisionIsNotEnough(t *testing.T) {
	cl := &sizedClient{}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.AutoCompact = windowTwenty
	r.D.AutoCompact.Elide = ElideOptions{Percent: 50, KeepRecent: 6, MinBytes: 1000, Head: 100, Tail: 100}
	s := toolHeavy(12, 6000)
	// Plenty of prose that elision cannot touch.
	for i := 0; i < 20; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: strings.Repeat("u", 2500)})
		s.Append(Turn{Role: llm.RoleAssistant, Text: strings.Repeat("a", 2500)})
	}
	s.Append(Turn{Role: llm.RoleUser, Text: "continue"})
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if cl.summaries != 1 {
		t.Fatalf("summaries %d, want 1", cl.summaries)
	}
}

func TestElisionBelowItsThresholdDoesNothing(t *testing.T) {
	cl := &sizedClient{}
	var evs []Ev
	r := newRunner(cl, &evs)
	r.D.AutoCompact = windowTwenty
	r.D.AutoCompact.Elide = ElideOptions{Percent: 50, KeepRecent: 6, MinBytes: 1000, Head: 100, Tail: 100}
	s := toolHeavy(4, 6000) // 24 KB: about 6k tokens, under 50% of 20k
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	for _, text := range toolTexts(s) {
		if strings.Contains(text, "omitted from the context") {
			t.Fatal("elided below the threshold")
		}
	}
}
