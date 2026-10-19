package agent

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Goal tracks a run in a goal mode. The agent ends it with Done or Blocked
// (the goal.done and goal.blocked functions of the js runtime). An answer
// without either is met by Nudge, which asks the agent to go on, up to a limit
// of rounds so that a run cannot go on for ever.
type Goal struct {
	mu     sync.Mutex
	max    int
	rounds int
	state  string
	text   string
}

// Verdicts of a goal run.
const (
	GoalDone    = "done"
	GoalBlocked = "blocked"
)

// NewGoal returns a goal that may ask the agent to go on max times.
func NewGoal(max int) *Goal { return &Goal{max: max} }

// Done records that the goal is met.
func (g *Goal) Done(summary string) error { return g.verdict(GoalDone, summary) }

// Blocked records that the agent cannot go on without the user.
func (g *Goal) Blocked(reason string) error { return g.verdict(GoalBlocked, reason) }

func (g *Goal) verdict(state, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("goal." + map[string]string{GoalDone: "done", GoalBlocked: "blocked"}[state] + " needs a text")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state, g.text = state, text
	return nil
}

// Verdict is "", GoalDone or GoalBlocked, with the text the agent gave.
func (g *Goal) Verdict() (state, text string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state, g.text
}

// Progress is how many times the agent was asked to go on, and the limit.
func (g *Goal) Progress() (rounds, max int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rounds, g.max
}

// Exhausted reports that the run ended without a verdict because the rounds
// were used up.
func (g *Goal) Exhausted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state == "" && g.rounds >= g.max
}

// Nudge returns the message that asks the agent to go on. ok is false when the
// run is over: the agent gave a verdict, or the rounds are used up.
func (g *Goal) Nudge() (text string, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.state != "" || g.rounds >= g.max {
		return "", false
	}
	g.rounds++
	return "The goal is not marked as met yet (" + strconv.Itoa(g.rounds) + " of at most " + strconv.Itoa(g.max) + " rounds). Go on. When it is met and you have checked it, call goal.done(summary). If you cannot go on without the user, call goal.blocked(reason).", true
}

// goalEvent is the steering turn that carries a nudge.
func goalEvent(text string) Turn {
	return eventTurn(Event{ID: NewID(), Source: SourceSystem, Kind: "goal", Text: text, Time: time.Now().UTC()})
}
