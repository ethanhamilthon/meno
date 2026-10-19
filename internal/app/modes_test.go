package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
)

func sayMode(t *testing.T, a *App, text string) {
	t.Helper()
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
}

func outputOf(a *App, id string) string {
	for _, tr := range a.st.Session.Snapshot() {
		if tr.CallID == id {
			return tr.Err + tr.Output
		}
	}
	return "<no such call>"
}

func userTurns(a *App) []agent.Turn {
	var out []agent.Turn
	for _, tr := range a.st.Session.Snapshot() {
		if tr.Role == llm.RoleUser && tr.Kind == "" {
			out = append(out, tr)
		}
	}
	return out
}

func TestPlanIsReadOnlyThenOffersBuild(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "fs.write('x.txt', '1')"}},
		{{Kind: llm.EvText, Text: "the plan"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "c2", Code: "fs.write('x.txt', '1'); 'written'"}},
		{{Kind: llm.EvText, Text: "built"}, {Kind: llm.EvDone}},
	}}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()

	sayMode(t, a, "#plan add x.txt")
	pump(t, a, func() bool { return a.st.Ask != nil })
	if out := outputOf(a, "c1"); !strings.Contains(out, "not available in plan mode") {
		t.Fatalf("a write in plan mode must be refused: %q", out)
	}
	if _, err := os.Stat(filepath.Join(paths.Cwd, "x.txt")); err == nil {
		t.Fatal("plan mode wrote a file")
	}
	first := userTurns(a)[0]
	if first.Text != "add x.txt" || first.Mode != "plan" {
		t.Fatalf("the tag is taken off the text and kept as the mode: %+v", first)
	}
	if !strings.Contains(a.st.Session.ToMsgs()[0].Text, "Mode: plan.") {
		t.Fatalf("the model must be told the mode:\n%s", a.st.Session.ToMsgs()[0].Text)
	}
	if q := a.st.Ask.F.Questions[0]; !strings.Contains(q.Text, "plan is ready") || len(q.Options) != 2 || q.Options[0].Label != "Start #build" {
		t.Fatalf("offer: %+v", q)
	}

	a.onKey(keyK(tcell.KeyEnter)) // the first option: start #build
	pump(t, a, func() bool { return cl.left() == 0 && !a.st.Running })
	if out := outputOf(a, "c2"); out != "written" {
		t.Fatalf("build may write: %q", out)
	}
	if b, err := os.ReadFile(filepath.Join(paths.Cwd, "x.txt")); err != nil || string(b) != "1" {
		t.Fatalf("x.txt: %q %v", b, err)
	}
	users := userTurns(a)
	if len(users) != 2 || users[1].Mode != "build" || users[1].Text != "Carry out the plan above." {
		t.Fatalf("%+v", users)
	}
}

func TestPlainRequestAfterAModeSaysTheModeEnded(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvText, Text: "built"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "plain"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayMode(t, a, "#build do it")
	pump(t, a, func() bool { return !a.st.Running && a.st.Ask == nil })
	sayMode(t, a, "and now just talk")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	msgs := a.st.Session.ToMsgs()
	var second string
	for _, m := range msgs {
		if strings.Contains(m.Text, "and now just talk") {
			second = m.Text
		}
	}
	if !strings.Contains(second, "Mode: none") || !strings.Contains(second, "(build)") {
		t.Fatalf("the second request must end the mode:\n%s", second)
	}
	if strings.Contains(msgs[0].Text, "Mode: none") {
		t.Fatal("the first request has no earlier mode to end")
	}
}

func TestBareModeWithoutATaskKeepsTheDraft(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	sayMode(t, a, "#plan")
	if a.st.Running || string(a.st.Input) != "#plan" {
		t.Fatalf("running=%v draft=%q", a.st.Running, string(a.st.Input))
	}
	if !strings.Contains(a.lastNotice(), "usage: #plan") {
		t.Fatalf("notice %q", a.lastNotice())
	}
}

func TestGoalKeepsGoingUntilTheAgentSaysDone(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvText, Text: "started"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "g1", Code: "goal.done('every test passes')"}},
		{{Kind: llm.EvText, Text: "finished"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayMode(t, a, "#goal make the tests pass")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if a.st.RunGoal == nil {
		t.Fatal("a goal run has a goal")
	}
	if state, text := a.st.RunGoal.Verdict(); state != agent.GoalDone || text != "every test passes" {
		t.Fatalf("verdict %q %q", state, text)
	}
	if rounds, _ := a.st.RunGoal.Progress(); rounds != 1 {
		t.Fatalf("one nudge expected, got %d", rounds)
	}
	if n := a.lastNotice(); !strings.Contains(n, "goal met") || !strings.Contains(n, "every test passes") {
		t.Fatalf("notice %q", n)
	}
}

func TestGoalWithoutAVerdictStopsAtTheLimit(t *testing.T) {
	cl := &scriptedClient{}
	for i := 0; i < 40; i++ {
		cl.turns = append(cl.turns, []llm.Event{{Kind: llm.EvText, Text: "still on it"}, {Kind: llm.EvDone}})
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Modes = append(a.st.Modes[:0:0], a.st.Modes...)
	for i := range a.st.Modes {
		if a.st.Modes[i].Name == "goal" {
			a.st.Modes[i].Rounds = 2
		}
	}
	sayMode(t, a, "#goal never finishes")
	pump(t, a, func() bool { return !a.st.Running })
	if !strings.Contains(a.lastNotice(), "goal stopped") || !strings.Contains(a.lastNotice(), "2 of 2") {
		t.Fatalf("notice %q", a.lastNotice())
	}
	if used := 40 - cl.left(); used != 3 {
		t.Fatalf("1 answer + 2 rounds = 3 requests, got %d", used)
	}
}

func TestModeTagCompletesInTheComposerAtTheStartOnly(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	typeRunes(a, "#pl")
	m := a.acMatches()
	if len(m) != 1 || m[0].label() != "#plan" || m[0].desc == "" {
		t.Fatalf("%+v", m)
	}
	a.onKey(keyK(tcell.KeyTab))
	if string(a.st.Input) != "#plan " {
		t.Fatalf("input %q", string(a.st.Input))
	}
	if toks := a.inputTokens(); len(toks) != 1 || toks[0].kind != kindMode || toks[0].name != "plan" {
		t.Fatalf("the tag is a token: %+v", toks)
	}
	a.setInput([]rune("fix the #pl"))
	if len(a.acMatches()) != 0 {
		t.Fatal("a mode is only the first word")
	}
	a.setInput([]rune("fix the #plan"))
	if len(a.inputTokens()) != 0 {
		t.Fatal("not a token in the middle of a message")
	}
}

func TestModeWhileRunningIsSentAsAPlainMessage(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.st.Running = true
	sayMode(t, a, "#plan look at this")
	items := a.st.Queue.Take()
	if len(items) != 1 || items[0].Text != "look at this" {
		t.Fatalf("%+v", items)
	}
	if !strings.Contains(a.lastNotice(), "applies to a new request") {
		t.Fatalf("notice %q", a.lastNotice())
	}
	a.st.Running = false
}
