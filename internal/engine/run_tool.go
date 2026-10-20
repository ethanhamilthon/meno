package engine

import (
	"regexp"
	"strconv"
	"time"

	"meno/internal/agent"
	"meno/internal/modes"
	"meno/internal/prompt"
)

// toolSpec is what a run tells the model about its one tool.
type toolSpec struct {
	name         string // "" is the js tool, "bash" the shell tool
	desc         string
	modelTimeout bool
	allow        []string // command groups of the bash prompt; nil = all
}

// groupsOf returns the command groups the session may use: a child gets the
// child set, a main agent what its modes allow (nil = every group).
func groupsOf(s *runState) []string {
	if s.session.Kind == agent.KindChild {
		return modes.ChildAllow()
	}
	return s.modes.AllowedGroups()
}

// toolSpecFor builds the tool definition of a session for one request.
func (e *Engine) toolSpecFor(s *runState) toolSpec {
	allow := groupsOf(s)
	model := e.cfg().BashModelTimeout()
	return toolSpec{name: "bash", desc: prompt.BashToolDescriptionFor(allow, model), modelTimeout: model, allow: allow}
}

// bashTimeout is the default limit of a bash call (0 = none).
func (e *Engine) bashTimeout() time.Duration { return e.cfg().BashTimeoutDuration() }

var exitLine = regexp.MustCompile(`\[exit (-?\d+)\]\s*$`)

// exitFromOutput reads the status line the bash tool ends its output with.
func exitFromOutput(out string) *int {
	m := exitLine.FindStringSubmatch(out)
	if m == nil {
		v := 1
		return &v
	}
	n, _ := strconv.Atoi(m[1])
	return &n
}
