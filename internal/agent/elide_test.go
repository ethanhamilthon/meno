package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"meno/internal/llm"
)

// elideSession is a user message followed by n single-call responses, each with an
// output of size bytes.
func elideSession(t *testing.T, n, size int) *Session {
	t.Helper()
	s := NewSession("/work", "m")
	s.Append(Turn{Role: llm.RoleUser, Text: "start"})
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("c%02d", i)
		out := fmt.Sprintf("<<%02d head>>", i) + strings.Repeat("x", size) + fmt.Sprintf("<<%02d tail>>", i)
		s.Append(Turn{Role: llm.RoleAssistant, CallID: id, Code: "cat file" + id, Output: out, ReplayGroup: "g" + id,
			Replay: &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: id, Name: "bash", Arguments: `{"command":"cat file"}`}}}})
	}
	return s
}

func toolTexts(s *Session) []string {
	var out []string
	for _, m := range s.ToMsgs() {
		if m.Role == llm.RoleTool {
			out = append(out, m.Text)
		}
	}
	return out
}

func wireOf(t *testing.T, s *Session) []string {
	t.Helper()
	var out []string
	for _, m := range s.ToMsgs() {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(b))
	}
	return out
}

var testElide = ElideOptions{Percent: 50, KeepRecent: 4, MinBytes: 1000, Head: 100, Tail: 100, MinSavedBytes: 1}

func TestElideOldKeepsRecentAndSmallOutputs(t *testing.T) {
	s := elideSession(t, 10, 5000)
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "small", Code: "echo", Output: "tiny", ReplayGroup: "gs",
		Replay: &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "small", Name: "bash", Arguments: `{}`}}}})
	turns, saved := s.ElideOld(testElide)
	if turns == 0 || saved < 30000 {
		t.Fatalf("elided %d turns, %d bytes", turns, saved)
	}
	got := toolTexts(s)
	if len(got) != 11 {
		t.Fatalf("%d tool results, want 11", len(got))
	}
	for i, text := range got[:7] { // older than the 4 newest turns
		if !strings.Contains(text, fmt.Sprintf("<<%02d head>>", i)) || !strings.Contains(text, fmt.Sprintf("<<%02d tail>>", i)) {
			t.Errorf("result %d lost its head or tail: %.80q", i, text)
		}
		if len(text) > 400 || !strings.Contains(text, "omitted from the context") {
			t.Errorf("result %d is not elided (%d bytes)", i, len(text))
		}
	}
	for i, text := range got[7:10] { // newest turns stay whole
		if len(text) < 5000 {
			t.Errorf("recent result %d was elided", i+7)
		}
	}
	if got[10] != "tiny" {
		t.Errorf("a small output changed: %q", got[10])
	}
}

func TestElideOldIsIdempotentAndKeepsStoredTurnsWhole(t *testing.T) {
	s := elideSession(t, 10, 5000)
	if turns, _ := s.ElideOld(testElide); turns == 0 {
		t.Fatal("nothing elided")
	}
	before := wireOf(t, s)
	if turns, saved := s.ElideOld(testElide); turns != 0 || saved != 0 {
		t.Fatalf("second call changed %d turns, %d bytes", turns, saved)
	}
	if after := wireOf(t, s); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("the model's view changed on a second call")
	}
	for i, turn := range s.Snapshot()[1:] {
		if len(turn.Output) < 5000 {
			t.Fatalf("stored turn %d lost output: the UI and export read it", i)
		}
	}
}

// What the model was sent stays the same when more turns are appended: the prefix
// is what the provider's cache matches.
func TestElidedPrefixIsStableWhenTurnsAreAppended(t *testing.T) {
	s := elideSession(t, 10, 5000)
	s.ElideOld(testElide)
	before := wireOf(t, s)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("n%d", i)
		s.Append(Turn{Role: llm.RoleAssistant, CallID: id, Output: strings.Repeat("y", 3000), ReplayGroup: "g" + id,
			Replay: &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: id, Name: "bash", Arguments: `{}`}}}})
	}
	after := wireOf(t, s)
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("message %d of the prefix changed", i)
		}
	}
}

func TestElideOldKeepsCallsPaired(t *testing.T) {
	s := elideSession(t, 12, 4000)
	s.ElideOld(testElide)
	calls, results := map[string]bool{}, map[string]bool{}
	for _, m := range s.ToMsgs() {
		for _, tc := range m.ToolCalls {
			calls[tc.ID] = true
		}
		if m.Role == llm.RoleTool {
			results[m.CallID] = true
		}
	}
	if len(calls) != 12 || len(results) != 12 {
		t.Fatalf("calls %d results %d", len(calls), len(results))
	}
}

func TestElideOldNeverTouchesErrorsAndRespectsSavingsFloor(t *testing.T) {
	s := elideSession(t, 8, 5000)
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "bad", Output: strings.Repeat("z", 9000), Err: "exit 2", ReplayGroup: "gb",
		Replay: &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "bad", Name: "bash", Arguments: `{}`}}}})
	for i := 0; i < 5; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: "more"})
	}
	if turns, _ := s.ElideOld(testElide); turns == 0 {
		t.Fatal("nothing elided")
	}
	for _, m := range s.ToMsgs() {
		if m.Role == llm.RoleTool && m.CallID == "bad" && len(m.Text) < 9000 {
			t.Fatal("the output of a failed call was elided")
		}
	}
	floor := testElide
	floor.MinSavedBytes = 1 << 30
	s2 := elideSession(t, 10, 5000)
	if turns, saved := s2.ElideOld(floor); turns != 0 || saved != 0 {
		t.Fatalf("below the savings floor the boundary must not move: %d %d", turns, saved)
	}
	if len(toolTexts(s2)[0]) < 5000 {
		t.Fatal("the view changed although nothing was elided")
	}
}

func TestElideKeepsTheFullOutputMarker(t *testing.T) {
	marker := "\n[900000 bytes omitted; full output: /tmp/out/log.txt]\n"
	out := strings.Repeat("a", 3000) + marker + strings.Repeat("b", 3000)
	s := elideSession(t, 6, 10)
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "huge", Output: out, ReplayGroup: "gh",
		Replay: &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "huge", Name: "bash", Arguments: `{}`}}}})
	for i := 0; i < 5; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: "more"})
	}
	s.ElideOld(testElide)
	for _, m := range s.ToMsgs() {
		if m.CallID == "huge" && m.Role == llm.RoleTool {
			if !strings.Contains(m.Text, "full output: /tmp/out/log.txt") || len(m.Text) > 600 {
				t.Fatalf("marker lost or output not elided (%d bytes): %q", len(m.Text), m.Text)
			}
			return
		}
	}
	t.Fatal("result missing")
}

func TestElideTextIsValidUTF8(t *testing.T) {
	out := strings.Repeat("é", 3000)
	if got := elideText(out, 101, 101); !strings.HasPrefix(got, "é") || strings.ToValidUTF8(got, "") != got {
		t.Fatalf("elided text is not valid UTF-8: %q", got[:40])
	}
}
