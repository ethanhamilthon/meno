package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/api"
)

func pendingForm() *api.AskForm {
	return &api.AskForm{AskID: "ask1", Questions: []api.AskQuestion{{ID: "q", Prompt: "Pick", Type: "single", Options: []string{"x"}, Choices: []api.AskChoice{{ID: "x", Label: "X"}}}}}
}

// Leaving a pending question for the session list answers nothing and cancels
// nothing: the form and the draft stay with their session.
func TestLeavingAPendingQuestionKeepsItOpen(t *testing.T) {
	a, _ := newTestApp(t)
	a.setInput([]rune("half written"))
	a.setAsk(a.st.Runtime, pendingForm())
	if a.st.Ask == nil {
		t.Fatal("the form should be open")
	}
	a.onKey(tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModCtrl))
	if a.st.Over != OverSessions {
		t.Fatalf("overlay %v, want the session list", a.st.Over)
	}
	if a.st.Runtime.Ask == nil || a.st.Runtime.Ask.ID != "ask1" {
		t.Fatal("the question must still be pending")
	}
	if a.st.Runtime.Ask.draft.Input == nil || string(a.st.Runtime.Ask.draft.Input) != "half written" {
		t.Fatalf("the draft was lost: %q", string(a.st.Runtime.Ask.draft.Input))
	}
}
