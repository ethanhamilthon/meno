package render

import (
	"fmt"
	"time"
)

// badgeDur is the short duration of a js badge: 0.4s, 2.3s, 17s, 120s. Very
// long calls fall back to FmtDur.
func badgeDur(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	tenths := int((d + 50*time.Millisecond) / (100 * time.Millisecond))
	switch {
	case d >= 1000*time.Second:
		return FmtDur(d)
	case tenths >= 100:
		return fmt.Sprintf("%ds", int((d+500*time.Millisecond)/time.Second))
	}
	return fmt.Sprintf("%d.%ds", tenths/10, tenths%10)
}

// timingBadge is the small mark in the top right corner of a js block: a spinner
// with the elapsed time while the call runs ("⠹ 2.3s"), then
// a check ("✓ 2.3s") or a cross ("✕ 2.3s") with the time it took. ok is false
// when nothing should be shown.
func timingBadge(t Turn, js JSOpts) (segs []Seg, ok bool) {
	if js.Exec {
		segs = []Seg{{Text: spinFrame(js.Spin) + " ", FG: Yellow}, {Text: badgeDur(js.Elapsed), FG: Yellow}}
		return segs, js.Timing != TimingOff
	}
	if js.Timing == TimingOff || t.ElapsedMs <= 0 && !t.TimedOut {
		return nil, false
	}
	if js.Timing == TimingSlow && !slowCall(t) {
		return nil, false
	}
	took := time.Duration(t.ElapsedMs) * time.Millisecond
	switch {
	case t.TimedOut || t.Err != "":
		segs = []Seg{{Text: "✕ ", FG: Red}, {Text: badgeDur(took), FG: Red}}
	case slowCall(t):
		segs = []Seg{{Text: "✓ ", FG: Yellow}, {Text: badgeDur(took), FG: Yellow}}
	default:
		segs = []Seg{{Text: "✓ ", FG: Green}, {Text: badgeDur(took), FG: FgDim}}
	}
	return segs, true
}
