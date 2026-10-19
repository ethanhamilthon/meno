package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"meno/internal/llm"
)

func jsTurn(code, out, group string) Turn {
	return Turn{Role: llm.RoleAssistant, CallID: "c" + code, Code: code, Output: out, ReplayGroup: group}
}

func toolTexts(msgs []llm.Msg) []string {
	var out []string
	for _, m := range msgs {
		if m.Role == llm.RoleTool {
			out = append(out, m.Text)
		}
	}
	return out
}

func sessionWith(turns ...Turn) *Session {
	s := NewSession(".", "m")
	for _, t := range turns {
		s.Append(t)
	}
	return s
}

// The model gets a big output whole; the middle goes once it has answered it.
func TestBigOutputIsCutOnlyAfterTheModelAnsweredIt(t *testing.T) {
	big := strings.Repeat("a", 3000) + strings.Repeat("m", 20000) + strings.Repeat("z", 3000)
	s := sessionWith(Turn{Role: llm.RoleUser, Text: "go"}, jsTurn("1", big, ""))
	if got := toolTexts(s.ToMsgs())[0]; got != big {
		t.Fatal("the output was cut before the model had answered it")
	}
	s.Append(Turn{Role: llm.RoleAssistant, Text: "thanks"})
	got := toolTexts(s.ToMsgs())[0]
	if len(got) > trimHead+trimTail+250 || !strings.HasPrefix(got, strings.Repeat("a", trimHead)) || !strings.HasSuffix(got, strings.Repeat("z", trimTail)) {
		t.Fatalf("%d bytes: %.80q", len(got), got)
	}
	if strings.Contains(got, "mmmm") || !strings.Contains(got, "bytes cut") || !strings.Contains(got, "narrower command") {
		t.Fatalf("marker: %q", got[max(0, len(got)-500):])
	}
	if s.Snapshot()[1].Output != big {
		t.Fatal("the stored output must stay whole")
	}
}

func TestOutputAtThresholdIsKeptAndOneByteMoreIsCut(t *testing.T) {
	for _, size := range []int{trimAbove, trimAbove + 1} {
		s := sessionWith(Turn{Role: llm.RoleUser, Text: "go"}, jsTurn("1", strings.Repeat("x", size), ""), Turn{Role: llm.RoleAssistant, Text: "done"})
		got := toolTexts(s.ToMsgs())[0]
		if cut := strings.Contains(got, "bytes cut"); cut != (size > trimAbove) {
			t.Errorf("size %d cut=%v", size, cut)
		}
	}
}

func TestSmallOutputsAndErrorsAreKept(t *testing.T) {
	s := sessionWith(Turn{Role: llm.RoleUser, Text: "go"}, jsTurn("1", strings.Repeat("s", 1024), ""), jsTurn("2", "x", ""), Turn{Role: llm.RoleAssistant, Text: "ok"})
	for _, o := range toolTexts(s.ToMsgs()) {
		if strings.Contains(o, "bytes cut") {
			t.Fatal("an output at the limit was cut")
		}
	}
}

// The rendering of a turn changes once, when the model answers; after that the
// request is a pure extension of the one before, so the cache keeps serving it.
func TestCutIsStableAfterItHappened(t *testing.T) {
	big := strings.Repeat("x", 30000)
	s := sessionWith(Turn{Role: llm.RoleUser, Text: "go"}, jsTurn("1", big, ""), Turn{Role: llm.RoleAssistant, Text: "a"})
	before := s.ToMsgs()
	s.Append(Turn{Role: llm.RoleUser, Text: "more"})
	s.Append(jsTurn("2", "small", ""))
	s.Append(Turn{Role: llm.RoleAssistant, Text: "b"})
	after := s.ToMsgs()
	for i := range before {
		if before[i].Text != after[i].Text || before[i].Role != after[i].Role {
			t.Fatalf("message %d changed once the cut was made", i)
		}
	}
}

// Parallel calls of one response are answered together: the next response of the
// model is what cuts them, not their own siblings.
func TestBatchOutputsWaitForTheNextResponse(t *testing.T) {
	big := strings.Repeat("b", 20000)
	s := sessionWith(Turn{Role: llm.RoleUser, Text: "go"}, jsTurn("1", big, "g"), jsTurn("2", big, "g"))
	for _, o := range toolTexts(s.ToMsgs()) {
		if o != big {
			t.Fatal("a result was cut while its batch was still the latest")
		}
	}
	s.Append(Turn{Role: llm.RoleAssistant, Text: "next"})
	for _, o := range toolTexts(s.ToMsgs()) {
		if len(o) > trimHead+trimTail+250 {
			t.Fatal("not cut after the next response")
		}
	}
}

func TestCutKeepsValidUTF8(t *testing.T) {
	out := trimOutput(strings.Repeat("я", 12000))
	if !utf8.ValidString(out) {
		t.Fatal("cut inside a character")
	}
}

// A compaction head is rendered exactly like the same part of the full request.
func TestHeadSharesTheCutOfTheFullRequest(t *testing.T) {
	big := strings.Repeat("q", 20000)
	s := sessionWith(Turn{Role: llm.RoleUser, Text: "go"}, jsTurn("1", big, ""), Turn{Role: llm.RoleAssistant, Text: "a"}, Turn{Role: llm.RoleUser, Text: "next"})
	all := s.Snapshot()
	full, head := s.ToMsgs(), s.msgsOf(all, 2) // the user turn and the js call
	if len(head) != 3 || head[2].Text != full[2].Text {
		t.Fatalf("the head renders the output differently from the full request: %d messages", len(head))
	}
}
