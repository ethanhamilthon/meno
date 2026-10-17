package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/jsvm"
	"meno/internal/ui"
)

type askOutcome struct {
	text string
	err  error
}

// askAsync puts a question from another goroutine, as the VM does, and feeds
// the resulting open event to the UI (this goroutine).
func askAsync(t *testing.T, a *App, ctx context.Context, q jsvm.Question) <-chan askOutcome {
	t.Helper()
	out := make(chan askOutcome, 1)
	go func() {
		text, err := (appAsker{a, a.st.Session.ID}).Ask(ctx, q)
		out <- askOutcome{text, err}
	}()
	select {
	case e := <-a.events:
		a.handle(e)
	case <-time.After(5 * time.Second):
		t.Fatal("no ask event")
	}
	if a.st.Ask == nil {
		t.Fatal("question not open")
	}
	return out
}

func result(t *testing.T, out <-chan askOutcome) askOutcome {
	t.Helper()
	select {
	case r := <-out:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("no answer")
	}
	return askOutcome{}
}

func TestAskChoicePanelAndDraft(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeText(a, "half typed message")
	out := askAsync(t, a, context.Background(), jsvm.Question{Text: "Which database?", Choices: []string{"sqlite", "postgres"}, CustomAnswer: true})
	a.draw()
	text := screenText(scr)
	for _, want := range []string{"agent asks", "Which database?", "1 sqlite", "2 postgres", "3 Your own answer", "waiting for your answer", "enter confirm"} {
		if !strings.Contains(text, want) {
			t.Fatalf("panel misses %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "half typed message") {
		t.Fatal("the draft must be put aside while the question is open")
	}
	a.onKey(key('j'))
	a.onKey(keyK(tcell.KeyEnter))
	if r := result(t, out); r.err != nil || r.text != "postgres" {
		t.Fatalf("%+v", r)
	}
	if a.st.Ask != nil || string(a.st.Input) != "half typed message" {
		t.Fatalf("draft not restored: %q ask=%v", string(a.st.Input), a.st.Ask)
	}
	a.draw()
	if strings.Contains(screenText(scr), "agent asks") {
		t.Fatal("panel still drawn")
	}
}

func TestAskDigitPicksAndCustomAnswerIsTyped(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	out := askAsync(t, a, context.Background(), jsvm.Question{Text: "pick", Choices: []string{"a", "b"}, CustomAnswer: true})
	a.onKey(key('1'))
	if r := result(t, out); r.text != "a" {
		t.Fatalf("%+v", r)
	}
	out = askAsync(t, a, context.Background(), jsvm.Question{Text: "pick", Choices: []string{"a", "b"}, CustomAnswer: true})
	a.onKey(key('3')) // "Your own answer…"
	if !a.st.Ask.Typing {
		t.Fatal("custom answer should move input to the composer")
	}
	typeText(a, "my own")
	press(a, tcell.KeyEnter, 0)
	if r := result(t, out); r.text != "my own" {
		t.Fatalf("%+v", r)
	}
	// Without a custom answer there is no such row, and 2 beyond the list is ignored.
	out = askAsync(t, a, context.Background(), jsvm.Question{Text: "pick", Choices: []string{"a"}, CustomAnswer: false})
	a.draw()
	if strings.Contains(screenText(scr), "own answer") {
		t.Fatal("custom row shown although customAnswer=false")
	}
	a.onKey(key('2'))
	if a.st.Ask == nil {
		t.Fatal("a digit past the list must not answer")
	}
	a.onKey(key('1'))
	result(t, out)
}

func TestAskFreeTextAndDecline(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	out := askAsync(t, a, context.Background(), jsvm.Question{Text: "Port?", CustomAnswer: true})
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "Port?") || !strings.Contains(text, "your answer") {
		t.Fatal(text)
	}
	typeText(a, "8080")
	// Esc only leaves insert mode here; it must not decline.
	press(a, tcell.KeyEsc, 0)
	if a.st.Ask == nil {
		t.Fatal("esc declined a typed question")
	}
	a.onKey(key('i'))
	press(a, tcell.KeyEnter, 0)
	if r := result(t, out); r.text != "8080" {
		t.Fatalf("%+v", r)
	}
	// Empty answers are not sent.
	out = askAsync(t, a, context.Background(), jsvm.Question{Text: "again?"})
	press(a, tcell.KeyEnter, 0)
	if a.st.Ask == nil {
		t.Fatal("empty answer accepted")
	}
	press(a, tcell.KeyCtrlX, 0)
	if r := result(t, out); !errors.Is(r.err, jsvm.ErrDeclined) {
		t.Fatalf("%+v", r)
	}
	// Esc declines a choice list.
	out = askAsync(t, a, context.Background(), jsvm.Question{Text: "x", Choices: []string{"a"}, CustomAnswer: true})
	press(a, tcell.KeyEsc, 0)
	if r := result(t, out); !errors.Is(r.err, jsvm.ErrDeclined) {
		t.Fatalf("%+v", r)
	}
}

func TestAskClosesWhenTheTurnIsCanceled(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeText(a, "keep me")
	ctx, cancel := context.WithCancel(context.Background())
	out := askAsync(t, a, ctx, jsvm.Question{Text: "x?"})
	cancel()
	select {
	case e := <-a.events:
		a.handle(e)
	case <-time.After(5 * time.Second):
		t.Fatal("no close event")
	}
	if r := result(t, out); !errors.Is(r.err, context.Canceled) {
		t.Fatalf("%+v", r)
	}
	if a.st.Ask != nil || string(a.st.Input) != "keep me" {
		t.Fatalf("not closed cleanly: ask=%v input=%q", a.st.Ask, string(a.st.Input))
	}
}

func TestAskPanelHiddenUnderDialogsAndLimited(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	var choices []string
	for i := 0; i < 30; i++ {
		choices = append(choices, "option number "+itoa(i))
	}
	out := askAsync(t, a, context.Background(), jsvm.Question{Text: strings.Repeat("a very long question ", 40), Choices: choices, CustomAnswer: true, Timeout: time.Minute})
	_, h := a.scr.Size()
	if got := a.askHeight(); got > h*askMaxShare/100 || got < 4 {
		t.Fatalf("panel height %d of %d", got, h)
	}
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "answer within") || !strings.Contains(text, "more") || !strings.Contains(text, "INSERT") {
		t.Fatalf("\n%s", text)
	}
	// Scrolling keeps the selection visible.
	for i := 0; i < 25; i++ {
		a.onKey(key('j'))
	}
	a.draw()
	if !strings.Contains(screenText(scr), "option number 24") {
		t.Fatalf("selection scrolled out:\n%s", screenText(scr))
	}
	// A dialog covers the panel; the question stays pending.
	a.runCommand("settings")
	a.draw()
	if a.askHeight() != 0 || strings.Contains(screenText(scr), "agent asks") {
		t.Fatal("panel must be hidden while a dialog is open")
	}
	press(a, tcell.KeyEsc, 0)
	if a.st.Ask == nil || a.st.Over != OverNone {
		t.Fatalf("dialog close lost the question: over=%v", a.st.Over)
	}
	a.draw()
	if !strings.Contains(screenText(scr), "agent asks") {
		t.Fatal("panel did not come back")
	}
	press(a, tcell.KeyCtrlX, 0)
	result(t, out)
	if a.st.Mode != ui.ModeInsert {
		t.Fatalf("mode %v", a.st.Mode)
	}
}

func TestAskBellFollowsTheSetting(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	if a.st.Settings.Notify != "off" {
		t.Fatal("the bell must be off by default")
	}
}
