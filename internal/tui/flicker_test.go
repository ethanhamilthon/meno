package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

func paced(turns ...[]llm.Event) *pacedClient {
	return &pacedClient{turns: turns, gap: 12 * time.Millisecond}
}

func chunks(kind string, parts ...string) []llm.Event {
	var out []llm.Event
	for _, p := range parts {
		out = append(out, llm.Event{Kind: kind, Text: p})
	}
	return out
}

func codeDeltas(id string, parts ...string) []llm.Event {
	var out []llm.Event
	acc := ""
	for _, p := range parts {
		acc += p
		out = append(out, llm.Event{Kind: llm.EvToolDelta, CallID: id, Code: acc})
	}
	return out
}

// The detector itself: a background that flips and returns must be reported,
// text that only grows must not.
func TestFlickerDetectorSeesAFlipAndIgnoresGrowth(t *testing.T) {
	plain := tcell.StyleDefault
	green := plain.Background(tcell.ColorGreen)
	frame := func(cells ...frameCellState) screenFrame { return screenFrame{w: len(cells), h: 1, cells: cells} }
	c := func(r rune, st tcell.Style) frameCellState { return frameCellState{r: r, style: st} }

	growth := frameSeq{frame(c(' ', plain), c(' ', plain)), frame(c('a', plain), c(' ', plain)), frame(c('a', plain), c('b', plain))}
	if r := growth.reversals(nil); len(r) != 0 {
		t.Fatalf("growing text reported as flicker: %v", r)
	}
	if n := growth.restyles(nil); len(n) != 0 {
		t.Fatalf("growing text reported as repainted: %v", n)
	}
	flip := frameSeq{frame(c(' ', plain)), frame(c(' ', green)), frame(c(' ', plain))}
	if r := flip.reversals(nil); len(r) != 1 {
		t.Fatalf("a background that flipped and returned was not found: %v", r)
	}
	if r := flip.reversals(func(x, y int) bool { return true }); len(r) != 0 {
		t.Fatalf("a skipped region was reported: %v", r)
	}
	clock := frameSeq{frame(c('1', plain)), frame(c('2', plain)), frame(c('1', plain))}
	if r := clock.reversals(nil); len(r) != 0 {
		t.Fatalf("a digit of a running clock was reported: %v", r)
	}
	recolor := frameSeq{frame(c('a', plain)), frame(c('a', plain.Foreground(tcell.ColorRed)))}
	if n := recolor.restyles(nil); len(n) != 1 {
		t.Fatalf("recolored text was not found: %v", n)
	}
}

func commandTurns(code string, parts ...string) [][]llm.Event {
	first := append(chunks(llm.EvText, "running it"), codeDeltas("c1", parts...)...)
	first = append(first, llm.Event{Kind: llm.EvTool, CallID: "c1", Name: "bash", Code: code, Arguments: `{"command":` + jsonString(code) + `}`})
	return [][]llm.Event{first, chunks(llm.EvText, "all ", "done")}
}

func jsonString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}

// A command that appears, runs and ends is repainted at most once per cell: no
// background changes, no color that goes back.
func TestFlickerCommandBlock(t *testing.T) {
	for name, code := range map[string]string{
		"ok":     "echo one && echo two",
		"failed": "echo bad; exit 3",
		"multi":  "echo a\necho b\necho c",
	} {
		t.Run(name, func(t *testing.T) {
			parts := []string{code[:len(code)/3], code[len(code)/3 : 2*len(code)/3], code[2*len(code)/3:]}
			a, scr, _ := e2eApp(t, paced(commandTurns(code, parts...)...))
			defer scr.Fini()
			fs, skip := recordTurn(t, a, scr, "run it")
			if len(fs) < 6 {
				t.Fatalf("only %d frames were recorded", len(fs))
			}
			if r := fs.reversals(skip); len(r) != 0 {
				t.Fatalf("%d cells flicker, first: %v", len(r), r[:min(5, len(r))])
			}
			if at, n := maxCount(fs.restyles(skip)); n > 1 {
				t.Fatalf("cell %s was repainted %d times", at, n)
			}
		})
	}
}

// Streaming text and thinking only append. Cells that are already on screen
// keep their glyph and style.
func TestFlickerStreamingTextAndThinking(t *testing.T) {
	cl := paced([]llm.Event{
		{Kind: llm.EvThinking, Text: "weighing "}, {Kind: llm.EvThinking, Text: "the options "}, {Kind: llm.EvThinking, Text: "carefully"},
		{Kind: llm.EvText, Text: "The "}, {Kind: llm.EvText, Text: "answer "}, {Kind: llm.EvText, Text: "is "}, {Kind: llm.EvText, Text: "forty-two."},
	})
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	fs, skip := recordTurn(t, a, scr, "question")
	if len(fs) < 6 {
		t.Fatalf("only %d frames were recorded", len(fs))
	}
	if r := fs.reversals(skip); len(r) != 0 {
		t.Fatalf("%d cells flicker, first: %v", len(r), r[:min(5, len(r))])
	}
	if at, n := maxCount(fs.restyles(skip)); n > 0 {
		t.Fatalf("text already on screen was repainted: cell %s %d times", at, n)
	}
}

// A response with two commands: the fast one ends while the slow one still runs.
// Its block stays on screen with its output; it does not vanish until both are saved.
func TestFlickerTwoCommandsEndingApart(t *testing.T) {
	events := chunks(llm.EvText, "two things")
	events = append(events, codeDeltas("c1", "echo fa", "echo fast")...)
	events = append(events, llm.Event{Kind: llm.EvTool, CallID: "c1", Name: "bash", Code: "echo fast", Arguments: `{"command":"echo fast"}`})
	events = append(events, codeDeltas("c2", "sleep 0.", "sleep 0.4; echo slow")...)
	events = append(events, llm.Event{Kind: llm.EvTool, CallID: "c2", Name: "bash", Code: "sleep 0.4; echo slow", Arguments: `{"command":"sleep 0.4; echo slow"}`})
	a, scr, _ := e2eApp(t, paced(events, chunks(llm.EvText, "finished")))
	defer scr.Fini()
	fs, skip := recordTurn(t, a, scr, "go")
	// The output of the first command pushes the second one down, so cells may go
	// blank and back; what must hold is that a block never leaves the screen.
	for _, needle := range []string{"echo fast", "sleep 0.4; echo slow", "two things"} {
		if gone := vanishes(fs, needle); gone >= 0 {
			t.Fatalf("%q was on screen and left it at frame %d", needle, gone)
		}
	}
	if at, n := maxCount(fs.restyles(skip)); n > 1 {
		t.Fatalf("cell %s was repainted %d times", at, n)
	}
	// While the slow command ran, the fast one was already shown with its output.
	seen := false
	for _, f := range fs {
		seen = seen || (frameHasRow(f, "    fast") && frameHasRow(f, "    sleep 0.4; echo slow") && !frameHasRow(f, "    slow"))
	}
	if !seen {
		t.Fatal("the finished command was not on screen while the slow one ran")
	}
}

// vanishes returns the first frame in which needle is missing after it has been
// shown, or -1.
func vanishes(fs frameSeq, needle string) int {
	shown := false
	for i, f := range fs {
		has := strings.Contains(frameText(f), needle)
		if shown && !has {
			return i
		}
		shown = shown || has
	}
	return -1
}

// frameHasRow reports whether a row of the frame, without trailing blanks, is row.
func frameHasRow(f screenFrame, row string) bool {
	for _, r := range strings.Split(frameText(f), "\n") {
		if strings.TrimRight(r, " ") == row {
			return true
		}
	}
	return false
}

func frameText(f screenFrame) string {
	var b strings.Builder
	for i, c := range f.cells {
		b.WriteRune(c.r)
		if (i+1)%f.w == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
