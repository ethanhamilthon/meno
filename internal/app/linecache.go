package app

import (
	"fmt"

	"meno/internal/agent"
	"meno/internal/ui"
)

// lineKey identifies the layout of one persisted turn. A turn is immutable once
// saved except for the run summary and the system block, which the signature
// covers, so the layout can be reused until the width or a display setting
// changes.
type lineKey struct {
	hid   int64
	width int
	opts  string
	sig   string
}

// lineCache keeps the laid-out lines of persisted turns, so a frame (or a
// token of streaming) does not lay out the whole history again.
type lineCache struct {
	m map[lineKey][]ui.Line
}

const lineCacheMax = 6000

func turnSig(t agent.Turn) string {
	return fmt.Sprintf("%d/%d/%d/%d/%d/%v/%d/%d", len(t.Text), len(t.Code), len(t.Output), len(t.Err), t.ElapsedMs, t.Run != nil, len(t.Attachments), len(t.Title))
}

// optsKey is the part of the display options that changes persisted layouts.
func optsKey(js ui.JSOpts) string {
	js.Elapsed, js.Spin, js.Exec, js.Running, js.Streaming = 0, 0, false, false, false
	return fmt.Sprintf("%+v", js)
}

// turnLines returns the layout of turn t, from the cache when possible. Turns
// without a history ID (legacy or synthetic) are laid out every time.
func (c *lineCache) turnLines(t agent.Turn, w int, js ui.JSOpts) []ui.Line {
	if t.HID == 0 {
		return ui.BuildLines([]agent.Turn{t}, w, js)
	}
	k := lineKey{t.HID, w, optsKey(js), turnSig(t)}
	if lines, ok := c.m[k]; ok {
		return lines
	}
	if c.m == nil || len(c.m) > lineCacheMax {
		c.m = map[lineKey][]ui.Line{}
	}
	lines := ui.BuildLines([]agent.Turn{t}, w, js)
	c.m[k] = lines
	return lines
}
