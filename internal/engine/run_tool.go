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

// isBash reports whether the session works through the bash tool.
func isBash(s *runState) bool { return s.session.Tool == "bash" }

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
	if !isBash(s) {
		return toolSpec{desc: prompt.ToolDescriptionFor(s.session.Kind == agent.KindChild, s.modes.Only), modelTimeout: e.settings.BashModelTimeout()}
	}
	allow := groupsOf(s)
	model := e.settings.BashModelTimeout()
	return toolSpec{name: "bash", desc: prompt.BashToolDescriptionFor(allow, model), modelTimeout: model, allow: allow}
}

// bashTimeout is the default limit of a bash call (0 = none).
func (e *Engine) bashTimeout() time.Duration { return e.settings.BashTimeoutDuration() }

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
