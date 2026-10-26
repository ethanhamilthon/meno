package render

import (
	"testing"
	"time"
)

// A command block never has a background of its own. A background that changes
// when the call starts, runs, ends or fails repaints the whole block, which the
// eye reads as a flash.
func TestCommandBlockHasNoBackground(t *testing.T) {
	src := "echo one\nls -la | head -3\ncat <<'EOF'\nbody\nEOF"
	for _, tc := range []struct {
		name string
		turn Turn
		js   JSOpts
	}{
		{"streaming", Turn{Code: src}, JSOpts{Running: true, Streaming: true}},
		{"running", Turn{Code: src}, JSOpts{Running: true, Exec: true, Elapsed: 3 * time.Second}},
		{"done", Turn{Code: src, ElapsedMs: 1200, Output: "x"}, JSOpts{}},
		{"failed", Turn{Code: src, Err: "exit 1", ElapsedMs: 900}, JSOpts{}},
		{"timed out", Turn{Code: src, TimedOut: true}, JSOpts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := jsBlock(tc.turn, 80, tc.js)
			if len(lines) < 3 {
				t.Fatalf("block has %d lines", len(lines))
			}
			for i, l := range lines {
				if l.BG != BgTerm {
					t.Errorf("line %d has background %v, want the terminal background", i, l.BG)
				}
				for _, s := range l.Segs {
					if s.BG != 0 && s.BG != BgTerm {
						t.Errorf("line %d segment %q has its own background", i, s.Text)
					}
				}
			}
		})
	}
}

// A failure is shown by color of text: the elapsed time in the header turns red.
func TestFailedCommandHeaderIsRed(t *testing.T) {
	ok := bashHeader(Turn{Code: "false", ElapsedMs: 900}, 80, JSOpts{})
	bad := bashHeader(Turn{Code: "false", Err: "exit 1", ElapsedMs: 900}, 80, JSOpts{})
	last := func(l Line) Seg { return l.Segs[len(l.Segs)-1] }
	if last(ok).FG == Red {
		t.Fatal("a successful command header is red")
	}
	if last(bad).FG != Red || last(bad).Text != last(ok).Text {
		t.Fatalf("failed header ends with %+v, want the same time in red (%+v)", last(bad), last(ok))
	}
	if ok.Text() != bad.Text() {
		t.Fatalf("color is the only difference: %q vs %q", ok.Text(), bad.Text())
	}
}
