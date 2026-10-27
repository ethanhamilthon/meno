package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/api"
)

func twoTextForm() *api.AskForm {
	return &api.AskForm{AskID: "ask2", Questions: []api.AskQuestion{
		{ID: "a", Prompt: "First", Type: "text"},
		{ID: "b", Prompt: "Second", Type: "text"},
	}}
}

func TestAskAnswersAreTypedInTheComposer(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setInput([]rune("my draft"))
	a.setAsk(a.st.Runtime, twoTextForm())
	if len(a.st.Input) != 0 {
		t.Fatalf("the composer must be free for the answer: %q", string(a.st.Input))
	}
	typeRunes(a, "hello")
	if string(a.st.Ask.Ans[0].Other) != "hello" || string(a.st.Input) != "hello" {
		t.Fatalf("answer %q composer %q", string(a.st.Ask.Ans[0].Other), string(a.st.Input))
	}
	a.onKey(keyK(tcell.KeyEnter)) // next question
	if a.st.Ask.Cur != 1 || len(a.st.Input) != 0 {
		t.Fatalf("cur=%d composer=%q", a.st.Ask.Cur, string(a.st.Input))
	}
	typeRunes(a, "world")
	a.onKey(keyK(tcell.KeyBacktab)) // back to the first question
	a.draw()
	if a.st.Ask.Cur != 0 || string(a.st.Ask.Ans[1].Other) != "world" {
		t.Fatalf("answers must be kept: cur=%d %q", a.st.Ask.Cur, string(a.st.Ask.Ans[1].Other))
	}
	a.draw() // the first question is a text question: typing starts
	if string(a.st.Input) != "hello" {
		t.Fatalf("the first answer must come back to the composer: %q", string(a.st.Input))
	}
	a.closeAsk(a.st.Ask)
	if string(a.st.Input) != "my draft" {
		t.Fatalf("the draft must come back: %q", string(a.st.Input))
	}
}

func TestAskAnswerEditsLikeTheComposer(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setAsk(a.st.Runtime, twoTextForm())
	typeRunes(a, "ac")
	a.onKey(keyK(tcell.KeyLeft))
	typeRunes(a, "b")
	a.onKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt)) // a new line
	typeRunes(a, "d")
	if got := string(a.st.Ask.Ans[0].Other); got != "ab\ndc" {
		t.Fatalf("got %q", got)
	}
	a.onKey(keyK(tcell.KeyCtrlU))
	if len(a.st.Ask.Ans[0].Other) != 0 {
		t.Fatalf("ctrl+u must clear the answer: %q", string(a.st.Ask.Ans[0].Other))
	}
	a.askPaste("pasted text")
	if string(a.st.Ask.Ans[0].Other) != "pasted text" {
		t.Fatalf("paste: %q", string(a.st.Ask.Ans[0].Other))
	}
}

func TestAskCustomTextReplacesTheChosenOption(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.setAsk(a.st.Runtime, &api.AskForm{AskID: "ask3", Questions: []api.AskQuestion{
		{ID: "q", Prompt: "Pick", Type: "single", Options: []string{"x"}, Choices: []api.AskChoice{{ID: "x", Label: "X"}}},
	}})
	a.st.Ask.choose(0)
	a.st.Ask.Row = 1 // the custom-text row
	a.onKey(keyK(tcell.KeyEnter))
	if !a.st.Ask.Typing {
		t.Fatal("enter on the custom row must start typing")
	}
	typeRunes(a, "mine")
	if a.st.Ask.Ans[0].Picked[0] || string(a.st.Ask.Ans[0].Other) != "mine" {
		t.Fatalf("picked=%v other=%q", a.st.Ask.Ans[0].Picked, string(a.st.Ask.Ans[0].Other))
	}
	a.onKey(keyK(tcell.KeyEsc)) // leave typing
	if len(a.st.Input) != 0 {
		t.Fatalf("the composer must be empty again: %q", string(a.st.Input))
	}
}
