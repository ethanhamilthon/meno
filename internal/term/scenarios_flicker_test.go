//go:build termtest

package term

import (
	"bytes"
	"regexp"
	"testing"
)

// A full-screen clear repaints every cell and is the visible kind of flicker.
// meno draws by changing cells only, so after the first frame no clear may be
// written, except when the terminal is resized.
var fullClear = regexp.MustCompile(`\x1b\[[0-3]?J`)

func TestNoFullClearAfterFirstFrame(t *testing.T) {
	f := newFakeLLM(t)
	f.Handler = func(q *llmReq) {
		if q.Mentions("one-done") {
			q.Text("the command ran")
			return
		}
		q.Tool("running it", "echo one-done; sleep 0.2")
	}
	s := start(t, options{fake: f, proto: "chat"})
	first := s.RawLen()
	// The pattern must be the one tcell really writes, or this test proves nothing:
	// the first frame has to contain it.
	if !fullClear.Match(s.RawSince(0)[:first]) {
		t.Fatalf("the first frame holds no clear sequence; the pattern is wrong. Bytes: %q", s.RawSince(0)[:min(first, 200)])
	}
	s.Say("run it")
	s.WaitFor("the command ran")
	s.Cmd("help")
	s.WaitFor("help")
	s.Send("q")
	s.Say("again")
	s.WaitFor("the command ran")
	got := s.RawSince(first)
	if len(got) == 0 {
		t.Fatal("no output was captured")
	}
	if loc := fullClear.FindIndex(got); loc != nil {
		t.Fatalf("a clear sequence %q was written after the first frame (byte %d of %d)", got[loc[0]:loc[1]], loc[0], len(got))
	}
	if bytes.Contains(got, []byte("\x1bc")) {
		t.Fatal("a terminal reset was written after the first frame")
	}
}
