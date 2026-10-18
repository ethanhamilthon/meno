package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// execJS runs code in the current session's VM while the UI loop is pumped, so
// a form opened by the code can be answered by the test.
func execJS(t *testing.T, a *App, code string, during func()) string {
	t.Helper()
	var res string
	done := make(chan struct{})
	vm := a.st.VM
	go func() {
		r := vm.Exec(context.Background(), code)
		res = r.Output + r.Value + r.Err
		close(done)
	}()
	acted := false
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-done:
			return res
		case e := <-a.events:
			a.handle(e)
			if !acted && a.st.Ask != nil && during != nil {
				acted = true
				during()
			}
		case <-deadline:
			t.Fatal("timed out waiting for the code to finish")
		}
	}
}

const formJS = "askUser({questions: [\n  {id: \"db\", text: \"Which database?\", type: \"single\", options: [{id: \"pg\", label: \"Postgres\"}, {id: \"sq\", label: \"SQLite\"}], recommendation: \"Postgres\"},\n  {id: \"feat\", text: \"Features?\", type: \"multi\", options: [{id: \"a\", label: \"Auth\"}, {id: \"b\", label: \"Billing\"}, {id: \"c\", label: \"Cache\"}]},\n  {id: \"name\", text: \"Project name?\", type: \"text\"}\n]})"

func formKeyPress(a *App, k tcell.Key) { a.onKey(keyK(k)) }

func TestFormEnterAdvancesAndLastQuestionSubmitsImmediately(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	got := execJS(t, a, "JSON.stringify("+formJS+".answers)", func() {
		// Q1 single: Enter selects SQLite and goes to Q2.
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter)
		if a.st.Ask == nil || a.st.Ask.Cur != 1 {
			t.Fatalf("expected Q2 after Enter on single, got %+v", a.st.Ask)
		}
		// Q2 multi: space marks Auth and Cache, plus custom text; Enter goes to Q3.
		typeRunes(a, " ") // row 0: Auth
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyDown)
		typeRunes(a, " ") // row 2: Cache
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter) // custom row: start typing
		typeRunes(a, "Search")
		formKeyPress(a, tcell.KeyEnter) // done typing and advances to Q3
		if a.st.Ask == nil || a.st.Ask.Cur != 2 || !a.st.Ask.Typing {
			t.Fatalf("expected Q3 in typing mode, got %+v", a.st.Ask)
		}
		// Go back with Shift+Tab and forward again: answers stay intact.
		formKeyPress(a, tcell.KeyBacktab)
		if a.st.Ask.Cur != 1 || !a.st.Ask.Ans[1].Picked[0] || !a.st.Ask.Ans[1].Picked[2] {
			t.Fatalf("going back lost Q2 answer: %+v", a.st.Ask)
		}
		formKeyPress(a, tcell.KeyLeft)
		if a.st.Ask.Cur != 0 || !a.st.Ask.Ans[0].Picked[1] || a.st.Ask.Row != 1 {
			t.Fatalf("going back with Left lost Q1 answer: %+v", a.st.Ask)
		}
		formKeyPress(a, tcell.KeyRight)
		formKeyPress(a, tcell.KeyRight)
		// Q3 text: Enter on the last question sends all answers at once.
		typeRunes(a, "meno")
		formKeyPress(a, tcell.KeyEnter)
	})
	want := "[{\"id\":\"db\",\"selected\":[\"sq\"],\"text\":\"\"},{\"id\":\"feat\",\"selected\":[\"a\",\"c\"],\"text\":\"Search\"},{\"id\":\"name\",\"selected\":[],\"text\":\"meno\"}]"
	if got != want {
		t.Fatalf("answers\n got %s\nwant %s", got, want)
	}
	if a.st.Ask != nil {
		t.Fatal("the form must close after answering the last question")
	}
}

func TestFormSubmitFocusesFirstUnansweredAndSendsNothing(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	got := execJS(t, a, "try { "+formJS+"; 'sent' } catch (e) { e.name }", func() {
		// Jump to last question without answering Q1 and Q2, type blank text, press Enter.
		a.st.Ask.enter(2)
		typeRunes(a, "   ") // blank text is not an answer
		formKeyPress(a, tcell.KeyEnter)
		if a.st.Ask == nil || a.st.Ask.Cur != 2 || a.st.Ask.Notice == "" {
			t.Errorf("blank text on Q3 must not advance: %+v", a.st.Ask)
		}
		typeRunes(a, "valid")
		formKeyPress(a, tcell.KeyEnter) // Q3 answered, but Q1 is unanswered -> jumps to Q1
		if a.st.Ask == nil || a.st.Ask.Cur != 0 || a.st.Ask.Notice == "" {
			t.Errorf("must focus first unanswered question Q1: %+v", a.st.Ask)
		}
		formKeyPress(a, tcell.KeyEnter) // answers Q1 and moves to Q2
		if a.st.Ask == nil || a.st.Ask.Cur != 1 {
			t.Errorf("expected Q2: %+v", a.st.Ask)
		}
		a.st.Ask.Focus = focusCancel
		formKeyPress(a, tcell.KeyEnter)
	})
	if got != "AskCancelled" {
		t.Fatalf("cancel must throw AskCancelled and leak no partial answers, got %q", got)
	}
}

func TestFormSingleOtherReplacesOptionAndStateSurvivesResize(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	got := execJS(t, a, "JSON.stringify("+formJS+".answers[0])", func() {
		typeRunes(a, " ") // mark Postgres without advancing
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter) // custom row: start typing
		typeRunes(a, "MySQL")
		formKeyPress(a, tcell.KeyEnter) // finishes typing and advances to Q2
		if a.st.Ask.Ans[0].Picked[0] {
			t.Error("custom text must replace the chosen option in a single question")
		}
		scr.SetSize(40, 12) // narrow: the form collapses but keeps every action reachable
		a.draw()
		if !strings.Contains(screenText(scr), "Next") || !strings.Contains(screenText(scr), "Cancel") {
			t.Errorf("actions must stay reachable on a narrow terminal:\n%s", screenText(scr))
		}
		scr.SetSize(120, 40)
		a.draw()
		typeRunes(a, " ")               // Q2: mark Auth
		formKeyPress(a, tcell.KeyEnter) // advances to Q3
		typeRunes(a, "x")
		formKeyPress(a, tcell.KeyEnter) // Q3: submits all answers
	})
	if got != "{\"id\":\"db\",\"selected\":[],\"text\":\"MySQL\"}" {
		t.Fatalf("%s", got)
	}
}

func TestFormTimeoutThrowsAskTimeoutAndEventsQueue(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	code := "try { askUser({timeout: 0.2, questions: [{id: \"a\", text: \"t\", type: \"text\"}]}) } catch (e) { e.name }"
	got := execJS(t, a, code, nil)
	if got != "AskTimeout" || a.st.Ask != nil {
		t.Fatalf("%q ask=%v", got, a.st.Ask)
	}
}

func TestFormMouseClicksRowsAndButtons(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	got := execJS(t, a, "JSON.stringify("+formJS+".answers[0].selected)", func() {
		a.draw()
		click := func(kind, idx int) {
			for _, h := range a.st.Ask.hits {
				if h.kind == kind && h.idx == idx {
					a.askMouse(tcell.NewEventMouse(h.x+3, h.y, tcell.Button1, 0))
					return
				}
			}
			t.Errorf("no clickable area kind=%d idx=%d", kind, idx)
		}
		click(hitRow, 1) // SQLite (single question -> advances to Q2)
		if a.st.Ask.Cur != 1 {
			t.Error("clicking a single-choice option must select and advance")
		}
		a.st.Ask.Ans[1].Picked[1], a.st.Ask.Ans[2].Other = true, []rune("n")
		a.st.Ask.enter(2)
		a.draw()
		click(hitButton, focusNext)
	})
	if got != "[\"sq\"]" {
		t.Fatalf("%s", got)
	}
}
