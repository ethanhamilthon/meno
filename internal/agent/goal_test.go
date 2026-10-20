package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

func goalRunner(t *testing.T, cl *timeoutClient, g *Goal) (*Runner, *Session, *jsvm.VM) {
	t.Helper()
	vm := jsvm.New(jsvm.Options{Root: t.TempDir(), Timeout: 5 * time.Second})
	vm.SetGoal(g)
	r := &Runner{D: Deps{Retry: RetryPolicy{Attempts: 1}, Client: cl, Tool: vm, Goal: g, Emit: func(Ev) {}}}
	return r, NewSession(t.TempDir(), "m"), vm
}

func nudges(s *Session) int {
	n := 0
	for _, turn := range s.Snapshot() {
		if turn.Kind == KindEvent && strings.Contains(turn.Text, "kind=\"goal\"") {
			n++
		}
	}
	return n
}

func TestGoalAsksTheAgentToGoOnUntilTheRoundsRunOut(t *testing.T) {
	cl := &timeoutClient{}
	g := NewGoal(2)
	r, s, _ := goalRunner(t, cl, g)
	if err := r.Turn(context.Background(), s, "ship it"); err != nil {
		t.Fatal(err)
	}
	if len(cl.reqs) != 3 || nudges(s) != 2 {
		t.Fatalf("requests %d nudges %d", len(cl.reqs), nudges(s))
	}
	if !g.Exhausted() {
		t.Fatal("no verdict and no rounds left: exhausted")
	}
	if rounds, max := g.Progress(); rounds != 2 || max != 2 {
		t.Fatalf("%d/%d", rounds, max)
	}
}

func TestGoalDoneEndsTheRun(t *testing.T) {
	cl := &timeoutClient{calls: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "goal.done('all checks pass')"}, {Kind: llm.EvDone}},
	}}
	g := NewGoal(5)
	r, s, _ := goalRunner(t, cl, g)
	if err := r.Turn(context.Background(), s, "ship it"); err != nil {
		t.Fatal(err)
	}
	state, text := g.Verdict()
	if state != GoalDone || text != "all checks pass" || len(cl.reqs) != 2 || nudges(s) != 0 || g.Exhausted() {
		t.Fatalf("state %q text %q requests %d nudges %d", state, text, len(cl.reqs), nudges(s))
	}
}

func TestGoalBlockedAlsoEndsTheRun(t *testing.T) {
	cl := &timeoutClient{calls: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: "goal.blocked('I need the API key')"}, {Kind: llm.EvDone}},
	}}
	g := NewGoal(5)
	r, s, _ := goalRunner(t, cl, g)
	if err := r.Turn(context.Background(), s, "x"); err != nil {
		t.Fatal(err)
	}
	if state, text := g.Verdict(); state != GoalBlocked || !strings.Contains(text, "API key") || nudges(s) != 0 {
		t.Fatalf("%q %q", state, text)
	}
}

func TestRunWithoutAGoalEndsAtTheFirstAnswer(t *testing.T) {
	cl := &timeoutClient{}
	r := &Runner{D: Deps{Retry: RetryPolicy{Attempts: 1}, Client: cl, Tool: jsvm.New(jsvm.Options{Root: t.TempDir()}), Emit: func(Ev) {}}}
	if err := r.Turn(context.Background(), NewSession(t.TempDir(), "m"), "x"); err != nil || len(cl.reqs) != 1 {
		t.Fatalf("requests %d err %v", len(cl.reqs), err)
	}
}

func TestGoalVerdictNeedsText(t *testing.T) {
	g := NewGoal(1)
	if g.Done("  ") == nil || g.Blocked("") == nil {
		t.Fatal("an empty verdict must be refused")
	}
	if s, _ := g.Verdict(); s != "" {
		t.Fatal("no verdict yet")
	}
}

func TestGoalDoesNotNudgeWhileSubagentsWork(t *testing.T) {
	cl := &timeoutClient{}
	g := NewGoal(5)
	r, s, _ := goalRunner(t, cl, g)
	waiting := true
	r.D.Waiting = func() bool { return waiting }
	if err := r.Turn(context.Background(), s, "delegate it"); err != nil {
		t.Fatal(err)
	}
	if nudges(s) != 0 || len(cl.reqs) != 1 {
		t.Fatalf("a run that waits for subagents ends without a nudge: requests %d nudges %d", len(cl.reqs), nudges(s))
	}
	waiting = false
	if err := r.Continue(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if nudges(s) == 0 {
		t.Fatal("once nothing is pending the goal nudges again")
	}
}
