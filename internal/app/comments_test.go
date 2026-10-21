package app

import (
	"strings"
	"testing"

	"meno/internal/commentref"
	"meno/internal/objtag"
	"meno/internal/ui"
)

// commentApp has one saved assistant answer and puts the cursor range over rows
// that carry the given part.
func commentApp(t *testing.T) *App {
	t.Helper()
	a, _ := newTestApp(t)
	a.st.Session.turns = []ui.Turn{
		{ID: "u1", Role: ui.RoleUser, Text: "please explain"},
		{ID: "a1", Role: ui.RoleAssistant, Text: "The answer is forty two and nothing else."},
	}
	return a
}

// selectRows selects document rows [from, to] of the conversation.
func selectRows(a *App, from, to int) int {
	_, top := a.doc()
	a.st.InLog, a.st.AnchorLog = true, true
	a.st.ALine, a.st.ACol = from, 0
	a.st.LogLine, a.st.LogCol = to, 200
	a.st.Mode, a.st.VKind = ui.ModeVisual, 'V'
	return top
}

func rowOf(a *App, part string) int {
	w, _ := a.scr.Size()
	for i, l := range a.transcriptWithSelection(w) {
		if l.Source != nil && l.Source.Part == part {
			return i
		}
	}
	return -1
}

func TestCommentFromAssistantTextAppendsAtomicObjectAtDraftEnd(t *testing.T) {
	a := commentApp(t)
	a.setInput([]rune("my note "))
	row := rowOf(a, commentref.PartAssistant)
	if row < 0 {
		t.Fatal("assistant rows must carry a source")
	}
	top := selectRows(a, row, row)
	a.commentSelection(top)
	if a.st.Mode != ui.ModeInput {
		t.Fatalf("mode %v, want INPUT", a.st.Mode)
	}
	in := a.st.Input
	if len(in) != len([]rune("my note "))+1 || !isObjRune(in[len(in)-1]) {
		t.Fatalf("input %q", string(in))
	}
	msg := a.messageText()
	if !strings.HasPrefix(msg, "my note <comment>") {
		t.Fatalf("message %q", msg)
	}
	segs := objtag.Split(msg, nil)
	last := segs[len(segs)-1].Obj
	if last == nil || last.Kind != objtag.Comment {
		t.Fatalf("segments %+v", segs)
	}
	r, err := commentref.Decode(last.Ref)
	if err != nil || r.Segments[0].TurnID != "a1" || !strings.Contains(r.Segments[0].Quote, "forty two") {
		t.Fatalf("%+v %v", r, err)
	}
	if strings.Contains(r.Segments[0].Quote, "◆") {
		t.Fatalf("layout glyph leaked into the quote: %q", r.Segments[0].Quote)
	}
	if labelText(a.st.Input) == "" || !strings.Contains(labelText(a.st.Input), "[Comment - ") {
		t.Fatalf("label %q", labelText(a.st.Input))
	}
	a.undo()
	if string(a.st.Input) != "my note " {
		t.Fatalf("undo left %q", string(a.st.Input))
	}
}

func TestCommentRefusesUserTextAndDecoration(t *testing.T) {
	a := commentApp(t)
	w, _ := a.scr.Size()
	user := -1
	for i, l := range a.transcriptWithSelection(w) {
		if strings.Contains(l.Text(), "please explain") {
			user = i
		}
	}
	top := selectRows(a, user, user)
	a.commentSelection(top)
	if len(a.st.Input) != 0 {
		t.Fatalf("user text must not become a comment: %q", string(a.st.Input))
	}
}

func TestTwoSelectionsOfTheSameTextStayDistinct(t *testing.T) {
	a := commentApp(t)
	row := rowOf(a, commentref.PartAssistant)
	for i := 0; i < 2; i++ {
		a.commentSelection(selectRows(a, row, row))
	}
	n := 0
	for _, r := range a.st.Input {
		if isObjRune(r) {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want two comment objects, got %d in %q", n, string(a.st.Input))
	}
}
