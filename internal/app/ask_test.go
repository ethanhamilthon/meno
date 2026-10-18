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

const formJS = `askUser({questions: [
  {id: "db", text: "Which database?", type: "single", options: [{id: "pg", label: "Postgres"}, {id: "sq", label: "SQLite"}], recommendation: "Postgres"},
  {id: "feat", text: "Features?", type: "multi", options: [{id: "a", label: "Auth"}, {id: "b", label: "Billing"}, {id: "c", label: "Cache"}]},
  {id: "name", text: "Project name?", type: "text"}
]})`

func formKeyPress(a *App, k tcell.Key) { a.onKey(keyK(k)) }

func TestFormSubmitReturnsAllAnswersWithOtherSemantics(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	got := execJS(t, a, "JSON.stringify("+formJS+".answers)", func() {
		// Q1 single: pick SQLite with the keyboard.
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter)
		// Q2 multi: Auth and Cache, plus custom text.
		typeRunes(a, "]")
		formKeyPress(a, tcell.KeyEnter) // row 0: Auth
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter) // row 2: Cache
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter) // custom row: start typing
		typeRunes(a, "Search")
		formKeyPress(a, tcell.KeyEnter) // done typing
		// Q3 text: typing starts at once.
		typeRunes(a, "]")
		if a.st.Ask.Cur != 2 || !a.st.Ask.Typing {
			t.Errorf("a text question starts in typing mode: cur=%d typing=%v", a.st.Ask.Cur, a.st.Ask.Typing)
		}
		typeRunes(a, "meno")
		formKeyPress(a, tcell.KeyEsc) // leaves typing, the form stays
		if a.st.Ask == nil {
			t.Error("Esc must not close the form")
		}
		a.st.Ask.Focus = focusSubmit
		formKeyPress(a, tcell.KeyEnter)
	})
	want := `[{"id":"db","selected":["sq"],"text":""},{"id":"feat","selected":["a","c"],"text":"Search"},{"id":"name","selected":[],"text":"meno"}]`
	if got != want {
		t.Fatalf("answers\n got %s\nwant %s", got, want)
	}
	if a.st.Ask != nil {
		t.Fatal("the form must close after submit")
	}
}

func TestFormSubmitFocusesFirstUnansweredAndSendsNothing(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	got := execJS(t, a, "try { "+formJS+"; 'sent' } catch (e) { e.name }", func() {
		a.st.Ask.Focus = focusSubmit
		formKeyPress(a, tcell.KeyEnter) // nothing answered
		if a.st.Ask == nil || a.st.Ask.Cur != 0 || a.st.Ask.Notice == "" {
			t.Errorf("must stay open on the first unanswered question: %+v", a.st.Ask)
		}
		formKeyPress(a, tcell.KeyEnter) // row 0 selected (focus returned to the body)
		a.st.Ask.enter(2)
		typeRunes(a, "   ") // blank text is not an answer
		formKeyPress(a, tcell.KeyEsc)
		a.st.Ask.Focus = focusSubmit
		formKeyPress(a, tcell.KeyEnter)
		if a.st.Ask == nil || a.st.Ask.Cur != 1 {
			t.Errorf("question 2 is unanswered, and blank text does not count: %+v", a.st.Ask)
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
		formKeyPress(a, tcell.KeyEnter) // Postgres
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyDown)
		formKeyPress(a, tcell.KeyEnter) // custom row
		typeRunes(a, "MySQL")
		formKeyPress(a, tcell.KeyEnter)
		if a.st.Ask.Ans[0].Picked[0] {
			t.Error("custom text must replace the chosen option in a single question")
		}
		scr.SetSize(40, 12) // narrow: the form collapses but keeps every action reachable
		a.draw()
		if !strings.Contains(screenText(scr), "Submit") || !strings.Contains(screenText(scr), "Cancel") {
			t.Errorf("actions must stay reachable on a narrow terminal:\n%s", screenText(scr))
		}
		scr.SetSize(120, 40)
		a.draw()
		st := a.st.Ask
		st.Ans[1].Picked[0] = true
		st.Ans[2].Other = []rune("x")
		st.Focus = focusSubmit
		formKeyPress(a, tcell.KeyEnter)
	})
	if got != `{"id":"db","selected":[],"text":"MySQL"}` {
		t.Fatalf("%s", got)
	}
}

func TestFormTimeoutThrowsAskTimeoutAndEventsQueue(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	code := `try { askUser({timeout: 0.2, questions: [{id: "a", text: "t", type: "text"}]}) } catch (e) { e.name }`
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
		click(hitRow, 1) // SQLite
		click(hitQuestion, 1)
		if a.st.Ask.Cur != 1 {
			t.Error("clicking a question in the list must open it")
		}
		a.st.Ask.Ans[1].Picked[1], a.st.Ask.Ans[2].Other = true, []rune("n")
		a.draw()
		click(hitButton, focusSubmit)
	})
	if got != `["sq"]` {
		t.Fatalf("%s", got)
	}
}
