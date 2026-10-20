package agent

import (
	"reflect"
	"strings"
	"testing"

	"meno/internal/llm"
)

func TestToolResultsStayUnchangedAfterLaterTurns(t *testing.T) {
	output := strings.Repeat("full output\n", 4000)
	replay := llm.Msg{Role: llm.RoleAssistant, CallID: "a", Code: "cat file", ToolName: "bash"}
	cases := map[string][]Turn{
		"short":  {{Role: llm.RoleAssistant, CallID: "a", Code: "echo ok", Output: "ok"}},
		"legacy": {{Role: llm.RoleAssistant, CallID: "a", Code: "cat file", Output: output, Err: "failure"}},
		"replay": {{Replay: &replay, CallID: "a", Output: output, Err: "failure"}},
		"group": {
			{Replay: &replay, ReplayGroup: "g", CallID: "a", Output: output, Err: "failure"},
			{ReplayGroup: "g", CallID: "b", Output: output},
		},
	}
	for name, turns := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewSession(t.TempDir(), "test")
			for _, turn := range turns {
				s.Append(turn)
			}
			before := s.ToMsgs()
			var results []llm.Msg
			for _, m := range before {
				if m.Role == llm.RoleTool {
					results = append(results, m)
				}
			}
			if len(results) != len(turns) {
				t.Fatalf("got %d results, want %d", len(results), len(turns))
			}
			for i, m := range results {
				want := turns[i].Output
				if turns[i].Err != "" {
					want += "\nERROR: " + turns[i].Err
				}
				if m.Text != want || m.CallID != turns[i].CallID {
					t.Fatalf("result %d changed output, error, or call ID", i)
				}
			}
			for _, turn := range []Turn{
				{Role: llm.RoleAssistant, Text: "Done"},
				{Role: llm.RoleUser, Text: "Continue"},
			} {
				prefix := s.ToMsgs()
				s.Append(turn)
				grown := s.ToMsgs()
				if !reflect.DeepEqual(prefix, grown[:len(prefix)]) {
					t.Fatal("append changed an earlier message")
				}
			}
			after := s.ToMsgs()
			if !reflect.DeepEqual(before, after[:len(before)]) {
				t.Fatal("later turns changed the already-sent prefix")
			}
			if got := s.msgsOf(s.Snapshot()[:len(turns)]); !reflect.DeepEqual(before, got) {
				t.Fatal("compaction prefix differs from original messages")
			}
			if got := s.Snapshot()[0].Output; got != turns[0].Output {
				t.Fatal("stored output changed")
			}
		})
	}
}
