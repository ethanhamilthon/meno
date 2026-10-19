package app

import (
	"context"
	"path/filepath"
	"strings"

	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/modes"
	"meno/internal/safe"
)

// A message that starts with #plan, #build, #goal or a mode of the user's own
// is run in that mode (see internal/modes): its text goes to the model in front
// of the message, the js runtime gets the policy of the mode, a goal mode keeps
// the agent working until it gives a verdict, and a finished run can offer the
// next mode.

// reloadModes reads the builtin modes and the modes in $MENO_HOME/modes and in
// .meno/modes of the session's directory.
func (a *App) reloadModes() {
	a.st.Modes = modes.Load(filepath.Join(a.paths.Home, "modes"), filepath.Join(a.st.Cwd, ".meno", "modes"))
}

// modeKnown reports whether name is a mode.
func (a *App) modeKnown(name string) bool {
	_, ok := modes.Find(a.st.Modes, name)
	return ok
}

// lastUserMode is the mode of the latest request the user wrote in sess.
func lastUserMode(sess *agent.Session) string {
	turns := sess.Snapshot()
	for i := len(turns) - 1; i >= 0; i-- {
		if t := turns[i]; t.Role == llm.RoleUser && t.Kind == "" {
			return t.Mode
		}
	}
	return ""
}

// modeBlock is what the model is told about the mode of a new request. A
// request without a mode that follows one with a mode says that it ended, so
// the old instructions do not linger.
func modeBlock(m modes.Mode, previous string) string {
	switch {
	case m.Name != "":
		return modes.Block(m)
	case previous != "":
		return "Mode: none. The mode of the earlier request (" + previous + ") no longer applies; work as usual."
	}
	return ""
}

// joinBlocks puts non-empty blocks one under the other.
func joinBlocks(blocks ...string) string {
	var out []string
	for _, b := range blocks {
		if strings.TrimSpace(b) != "" {
			out = append(out, strings.TrimSpace(b))
		}
	}
	return strings.Join(out, "\n\n")
}

// applyMode sets up the runtime of the current session for a new request: the
// policy of the js runtime, and the goal when the mode has one.
func (a *App) applyMode(m modes.Mode) {
	a.st.RunMode, a.st.RunGoal = m, nil
	a.st.VM.SetPolicy(jsvm.Policy{ReadOnly: m.ReadOnly, Why: m.Name + " mode"})
	if m.Goal {
		a.st.RunGoal = agent.NewGoal(max(1, m.Rounds))
		a.st.VM.SetGoal(a.st.RunGoal)
	} else {
		a.st.VM.SetGoal(nil)
	}
}

// onModeEnd reports how a goal run ended and offers the next mode.
func (a *App) onModeEnd(err error) {
	m, g := a.st.RunMode, a.st.RunGoal
	if g != nil {
		state, text := g.Verdict()
		rounds, limit := g.Progress()
		switch {
		case state == agent.GoalDone:
			a.notify("goal met · " + oneLine(text))
		case state == agent.GoalBlocked:
			a.notify("goal blocked · " + oneLine(text))
		case g.Exhausted() && err == nil:
			a.notifyErr("goal stopped: no verdict after " + itoa(rounds) + " of " + itoa(limit) + " rounds")
		}
	}
	if err != nil || m.Next == "" || a.st.Queue.Len() > 0 {
		return // failed, nothing to offer, or the user already spoke again
	}
	if next, ok := modes.Find(a.st.Modes, m.Next); ok {
		a.offerNext(m, next)
	}
}

// nextMode is the answer to the offer that follows a run.
type nextMode struct {
	sid  string
	text string
}

// offerNext asks what to do after a run: start the next mode, or answer in
// the same one. The question is the usual form of the session.
func (a *App) offerNext(m, next modes.Mode) {
	sid := a.st.Session.ID
	form := jsvm.Form{Questions: []jsvm.FormQuestion{{
		ID: "next", Type: jsvm.TypeSingle, Text: "The " + m.Name + " is ready. What next?",
		Options: []jsvm.FormOption{{ID: "go", Label: "Start #" + next.Name}, {ID: "stay", Label: "Not now"}},
	}}}
	safe.Go(func() {
		ans, err := appAsker{a, sid}.Ask(context.Background(), form)
		if err != nil || len(ans) == 0 {
			return
		}
		switch {
		case ans[0].Text != "":
			a.postEvent(nextMode{sid, "#" + m.Name + " " + ans[0].Text}) // feedback: another round in the same mode
		case len(ans[0].Selected) == 1 && ans[0].Selected[0] == "go":
			a.postEvent(nextMode{sid, "#" + next.Name})
		}
	}, a.reportPanic)
}

func (a *App) onNextMode(m nextMode) {
	if a.st.Session.ID != m.sid || a.st.Running {
		return // the user went elsewhere or started something else meanwhile
	}
	a.startTurn(agent.Message{Text: m.text})
}

// modeStatus is the mode of the running request for the status bar: "#plan",
// or "#goal 3/25" with the rounds used so far.
func (a *App) modeStatus() string {
	if !a.st.Running || a.st.RunMode.Name == "" {
		return ""
	}
	s := "#" + a.st.RunMode.Name
	if g := a.st.RunGoal; g != nil {
		rounds, limit := g.Progress()
		s += " " + itoa(rounds) + "/" + itoa(limit)
	}
	return s
}
