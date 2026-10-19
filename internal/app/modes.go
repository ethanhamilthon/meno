package app

import (
	"strings"

	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/modes"
	"meno/internal/objtag"
)

// A message can name modes (#subagents, #goal, ...), chosen from the menu like
// a skill. The names of a message replace the modes of its session; a message
// without modes keeps them, and #none clears them. The modes apply to every
// request until they change: each one sets the prompt, the globals the run
// keeps, and whether a goal keeps the agent going.

// modeCatalog is the list of modes meno has.
func (a *App) modeCatalog() []modes.Mode { return modes.Catalog() }

// activeModes returns the modes of the current session, in the order they were
// named.
func (a *App) activeModes() []modes.Mode {
	var out []modes.Mode
	for _, n := range a.st.Session.Modes {
		if m, ok := modes.Find(a.modeCatalog(), n); ok {
			out = append(out, m)
		}
	}
	return out
}

// setModes replaces the modes of the current session with names. Unknown names
// are dropped.
func (a *App) setModes(names []string) {
	var keep []string
	for _, n := range names {
		if _, ok := modes.Find(a.modeCatalog(), n); ok && !hasName(keep, n) {
			keep = append(keep, n)
		}
	}
	a.st.Session.Modes = keep
}

func hasName(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// restrictVM sets the globals the session's modes keep, without a goal: a
// continuation keeps the goal of its request.
func (a *App) restrictVM() {
	c := modes.Compose(a.activeModes())
	a.st.VM.SetPolicy(jsvm.Policy{Only: c.Only})
}

// applyModes sets up the runtime for a new request: the globals the modes keep,
// and the goal when they have one. It returns the combined modes.
func (a *App) applyModes() modes.Combined {
	c := modes.Compose(a.activeModes())
	a.st.RunModes = c
	a.st.VM.SetPolicy(jsvm.Policy{Only: c.Only})
	if c.Goal {
		a.st.RunGoal = agent.NewGoal(max(1, c.Rounds))
		a.st.VM.SetGoal(a.st.RunGoal)
	} else {
		a.st.RunGoal = nil
		a.st.VM.SetGoal(nil)
	}
	return c
}

// waitingSubagents reports that the session has subagents still working, so a
// goal waits for their events instead of nudging the agent.
func (a *App) waitingSubagents(sid string) bool {
	if a.kids == nil {
		return false
	}
	recs, err := a.kids.store.List(sid, true)
	if err != nil {
		return false
	}
	for _, r := range recs {
		if r.Working() {
			return true
		}
	}
	return false
}

// messageModes reads the modes a message names: the <mode> objects of its text,
// in order, and the rest of the text. "none" clears the set: it is returned as
// no name, so the caller empties the set. tagged is false when the message
// names no mode at all, and then the set stays as it is.
func (a *App) messageModes(text string) (names []string, rest string, tagged bool) {
	known := func(k objtag.Kind, ref string) bool {
		if k != objtag.Mode {
			return false
		}
		return ref == modes.Control || a.objectKnown(k, ref)
	}
	taken, kept := objtag.Extract(text, known, func(o objtag.Object) bool { return o.Kind == objtag.Mode })
	if len(taken) == 0 {
		return nil, text, false
	}
	for _, o := range taken {
		if o.Ref != modes.Control && !hasName(names, o.Ref) {
			names = append(names, o.Ref)
		}
	}
	return names, strings.TrimSpace(kept), true
}

// modesNotice is the line that says which modes a session runs in now.
func modesNotice(names []string) string {
	if len(names) == 0 {
		return "modes off"
	}
	s := "modes:"
	for _, n := range names {
		s += " #" + n
	}
	return s
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

// onGoalEnd reports how a goal run ended. A run that waits for subagents has
// not ended without a verdict, so it is not reported as stopped.
func (a *App) onGoalEnd(err error) {
	g := a.st.RunGoal
	if g == nil {
		return
	}
	state, text := g.Verdict()
	rounds, limit := g.Progress()
	switch {
	case state == agent.GoalDone:
		a.notify("goal met · " + oneLine(text))
	case state == agent.GoalBlocked:
		a.notify("goal blocked · " + oneLine(text))
	case g.Exhausted() && err == nil && !a.waitingSubagents(a.st.Session.ID):
		a.notifyErr("goal stopped: no verdict after " + itoa(rounds) + " of " + itoa(limit) + " rounds")
	}
}
