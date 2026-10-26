package render

import (
	"strings"
	"time"
)

// CompactLive is the one row that stands for the work of a running turn in the
// compact view: what the agent is doing on the left (the start of the running
// command, or "thinking..."), the time of the whole turn on the right.
func CompactLive(what string, elapsed time.Duration, w int) Line {
	what = strings.Join(strings.Fields(what), " ")
	return ruleLine(what, FmtDur(elapsed), FgFaint, w)
}
