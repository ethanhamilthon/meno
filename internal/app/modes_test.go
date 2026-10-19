package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/modes"
	"meno/internal/objtag"
)

func outputOf(a *App, id string) string {
	for _, tr := range a.st.Session.Snapshot() {
		if tr.CallID == id {
			return tr.Err + tr.Output
		}
	}
	return "<no such call>"
}

func TestModesReplaceAsASetAndStayUntilChanged(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvText, Text: "one"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "g1", Code: "goal.done('shipped')"}},
		{{Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "three"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayObj(t, a, "#subagents #brainstorm idea")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 3 })
	if got := a.st.Session.Modes; len(got) != 2 || got[0] != "subagents" || got[1] != "brainstorm" {
		t.Fatalf("modes: %v", got)
	}
	sayObj(t, a, "#goal #subagents ship")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 1 })
	if got := a.st.Session.Modes; len(got) != 2 || got[0] != "goal" || got[1] != "subagents" {
		t.Fatalf("a new set replaces the old one as a whole: %v", got)
	}
	sayObj(t, a, "a plain message")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if got := a.st.Session.Modes; len(got) != 2 || got[0] != "goal" {
		t.Fatalf("a message without modes keeps the set: %v", got)
	}
}

func TestNoneClearsTheSetAndPlainTextIsNoMode(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.st.Session.Modes = []string{"goal"}
	sayObj(t, a, "#none")
	if len(a.st.Session.Modes) != 0 || a.st.Running {
		t.Fatalf("#none clears the set without a request: %v", a.st.Session.Modes)
	}
	a.setInput([]rune("fix the #goal login typed by hand"))
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if len(a.st.Session.Modes) != 0 {
		t.Fatalf("a typed #goal is text, not a mode: %v", a.st.Session.Modes)
	}
}

func TestModeIsSentInTheModelPromptAndLimitsTheRun(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "typeof fs"}},
		{{Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayObj(t, a, "#subagents split this")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if out := outputOf(a, "c1"); !strings.Contains(out, "undefined") {
		t.Fatalf("subagents has no fs: %q", out)
	}
	if !strings.Contains(a.st.Session.ToMsgs()[0].Text, "Mode: subagents.") {
		t.Fatalf("the model is told the mode:\n%s", a.st.Session.ToMsgs()[0].Text)
	}
}

func TestModeSurvivesARestartOfTheApp(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	sayObj(t, a, "#brainstorm plan it")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if err := agent.SaveSession(paths.Home, a.st.Session); err != nil {
		t.Fatal(err)
	}
	s, err := agent.LoadSession(paths.Home, a.st.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Modes) != 1 || s.Modes[0] != "brainstorm" {
		t.Fatalf("the set is kept with the session: %v", s.Modes)
	}
}

// sayObj sends a message whose modes are picked from the menu: each #name
// becomes an object, as acAccept makes it. Other words are typed as they are.
func sayObj(t *testing.T, a *App, text string) {
	t.Helper()
	var rs []rune
	for _, w := range strings.Split(text, " ") {
		if name, ok := strings.CutPrefix(w, "#"); ok && name != "" {
			r, ok := objectRune(objtag.Object{Kind: objtag.Mode, Ref: name})
			if !ok {
				t.Fatalf("no object for #%s", name)
			}
			rs = append(rs, r)
		} else {
			rs = append(rs, []rune(w)...)
		}
		rs = append(rs, ' ')
	}
	a.setInput(rs[:len(rs)-1])
	a.onKey(keyK(tcell.KeyEnter))
}

func TestGoalKeepsGoingUntilTheAgentSaysDone(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvText, Text: "started"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "g1", Code: "goal.done('every test passes')"}},
		{{Kind: llm.EvText, Text: "finished"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayObj(t, a, "#goal make the tests pass")
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
	for i := 0; i < modes.DefaultRounds+5; i++ {
		cl.turns = append(cl.turns, []llm.Event{{Kind: llm.EvText, Text: "still on it"}, {Kind: llm.EvDone}})
	}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayObj(t, a, "#goal never finishes")
	pump(t, a, func() bool { return !a.st.Running })
	want := strconv.Itoa(modes.DefaultRounds)
	if !strings.Contains(a.lastNotice(), "goal stopped") || !strings.Contains(a.lastNotice(), want+" of "+want) {
		t.Fatalf("notice %q", a.lastNotice())
	}
	if used := modes.DefaultRounds + 5 - cl.left(); used != modes.DefaultRounds+1 {
		t.Fatalf("1 answer + %d rounds = %d requests, got %d", modes.DefaultRounds, modes.DefaultRounds+1, used)
	}
}

func TestModeTagCompletesInTheComposerAnywhere(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	typeRunes(a, "fix the #sub")
	m := a.acMatches()
	if len(m) != 1 || m[0].label() != "#subagents" || m[0].desc == "" {
		t.Fatalf("%+v", m)
	}
	a.onKey(keyK(tcell.KeyTab))
	if toks := a.inputTokens(); len(toks) != 0 {
		t.Fatalf("a mode selected in the middle is an object, not a token here: %+v", toks)
	}
	if !strings.Contains(a.messageText(), "<mode>subagents</mode>") {
		t.Fatalf("the object is spelled as a tag: %q", a.messageText())
	}
	a.setInput([]rune("fix #subagents"))
	if strings.Contains(a.messageText(), "<mode>") {
		t.Fatal("a typed #name without the menu stays text")
	}
}

// A continuation (a subagent's event wakes the session) keeps the modes' globals
// and does not start a new request: the goal keeps its rounds.
func TestContinuationKeepsTheModePolicy(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvText, Text: "started"}, {Kind: llm.EvDone}},
		{{Kind: llm.EvTool, CallID: "w1", Code: "typeof fs"}},
		{{Kind: llm.EvText, Text: "after"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayObj(t, a, "#subagents split this")
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 2 })
	a.st.VM.Reset()
	a.startTurn(agent.Message{})
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if out := outputOf(a, "w1"); !strings.Contains(out, "undefined") {
		t.Fatalf("a continuation keeps the policy: %q", out)
	}
}
