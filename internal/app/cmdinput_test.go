package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/ui"
)

func enter() *tcell.EventKey { return keyK(tcell.KeyEnter) }

func TestColonAtWordStartOpensPopup(t *testing.T) {
	for in, open := range map[string]bool{
		":": true, ":ne": true, "hello :ne": true, "a\n:se": true, "http://x": false, "a:b": false, "key: v": false, ":zzz": false,
	} {
		a, scr := newTestApp(t)
		vimKeys(a, strings.ReplaceAll(in, "\n", "<cr>"))
		if got := a.cmdActive(); got != open {
			t.Errorf("%q: active=%v want %v", in, got, open)
		}
		scr.Fini()
	}
}

func TestEnterRunsCommandAndKeepsTheRestOfTheDraft(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Settings.Model = "gpt-5.5"
	vimKeys(a, "hello :cont")
	if len(commandRows(a)) == 0 || commandRows(a)[0] != ":context" {
		t.Fatalf("rows %v", commandRows(a))
	}
	a.onKey(enter())
	if a.st.Over != OverContext {
		t.Fatal("context did not open")
	}
	if string(a.st.Input) != "hello " {
		t.Fatalf("draft %q", string(a.st.Input))
	}
}

func TestCommandInTheMiddleOfTheDraft(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setInput([]rune("ab :tasks cd"))
	a.st.Cursor = len("ab :tasks")
	a.onKey(enter())
	if a.st.Over != OverTasks || string(a.st.Input) != "ab  cd" || a.st.Cursor != 3 {
		t.Fatalf("over=%v draft=%q cursor=%d", a.st.Over, string(a.st.Input), a.st.Cursor)
	}
}

func TestEscClosesPopupKeepsText(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, "x :ne")
	a.onKey(keyK(tcell.KeyEsc))
	if a.cmdActive() || a.st.Mode != ui.ModeInput || string(a.st.Input) != "x :ne" {
		t.Fatalf("active=%v mode=%v draft=%q", a.cmdActive(), a.st.Mode, string(a.st.Input))
	}
	vimKeys(a, "w")
	if a.cmdActive() {
		t.Fatal("typing on must not reopen a dismissed popup")
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != ui.ModeEdit {
		t.Fatal("Esc without popup enters EDIT")
	}
}

func TestArrowPickedRowRunsOnEmptyQuery(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, ":")
	a.onKey(enter())
	if a.st.Over != OverNone || string(a.st.Input) != ":" {
		t.Fatal("Enter without a pick must do nothing")
	}
	rows := commandRows(a)
	a.onKey(keyK(tcell.KeyDown))
	want := rows[1]
	a.onKey(enter())
	if string(a.st.Input) != "" {
		t.Fatalf("token not removed after %s: %q", want, string(a.st.Input))
	}
}

func TestFailedCommandKeepsTheToken(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, "go :cd /definitely/not/here")
	a.onKey(enter())
	if string(a.st.Input) != "go :cd /definitely/not/here" {
		t.Fatalf("draft %q", string(a.st.Input))
	}
}

func TestTabCompletesAndUndoRestoresToken(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	vimKeys(a, "q :ses")
	a.onKey(keyK(tcell.KeyTab))
	if string(a.st.Input) != "q :sessions" {
		t.Fatalf("draft %q", string(a.st.Input))
	}
	a.setInput([]rune("hi :tasks"))
	a.st.InsBase = snap{[]rune("hi :tasks"), 9}
	a.onKey(enter())
	if string(a.st.Input) != "hi " {
		t.Fatalf("draft %q", string(a.st.Input))
	}
	a.closeOverlay()
	a.undo()
	if string(a.st.Input) != "hi :tasks" {
		t.Fatalf("undo gave %q", string(a.st.Input))
	}
}

func TestCdArgumentSpansTheLine(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setInput([]rune("x :cd ~/pr"))
	tok, ok := a.cmdToken()
	if !ok || !tok.args || tok.query != "cd ~/pr" {
		t.Fatalf("%+v %v", tok, ok)
	}
	a.setInput([]rune("x :con|text"))
	a.st.Cursor = len("x :con")
	tok, _ = a.cmdToken()
	if tok.query != "con" || tok.end != len("x :con|text") {
		t.Fatalf("%+v", tok)
	}
}
