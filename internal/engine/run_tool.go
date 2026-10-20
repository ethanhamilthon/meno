package engine

import (
	"regexp"
	"strconv"

	"meno/internal/prompt"
)

// toolSpec is what a run tells the model about its one tool.
type toolSpec struct {
	name  string // "" is the js tool, "bash" the shell tool
	desc  string
	allow []string // command groups of the bash prompt; nil = all
}

// toolSpecFor builds the tool definition of a session for one request.
func (e *Engine) toolSpecFor(s *runState) toolSpec {
	var allow []string
	return toolSpec{name: "bash", desc: prompt.BashToolDescriptionFor(allow), allow: allow}
}

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
