package ui

import (
	"fmt"
	"strings"
	"time"
)

// TimingMode says which tool calls show how long they took.
type TimingMode int

const (
	TimingAll TimingMode = iota
	TimingSlow
	TimingOff
)

// FmtDur writes a duration the way a person reads it: 412ms, 3.2s, 17s, 1m 24s,
// 1h 02m. Negative durations count as zero.
func FmtDur(d time.Duration) string {
	if d <= 0 {
		return "0ms"
	}
	ms := (d + 500*time.Microsecond).Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if tenths := (ms + 50) / 100; tenths < 100 {
		return fmt.Sprintf("%d.%ds", tenths/10, tenths%10)
	}
	s := (ms + 500) / 1000
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm %02ds", s/60, s%60)
	}
	return fmt.Sprintf("%dh %02dm", s/3600, s%3600/60)
}

// FmtTokens abbreviates a token count: 950, 1.4k, 12.4k, 250k, 1.2m.
func FmtTokens(n int) string {
	trim := func(s string) string { return strings.TrimSuffix(s, ".0") }
	switch {
	case n < 1000:
		return fmt.Sprint(max(0, n))
	case n >= 999500:
		return trim(fmt.Sprintf("%.1f", float64(n)/1e6)) + "m"
	case n >= 99950:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
	return trim(fmt.Sprintf("%.1f", float64(n)/1000)) + "k"
}

// FmtCost writes a price with enough digits to be telling.
func FmtCost(c float64) string {
	switch {
	case c < 0.01:
		return fmt.Sprintf("$%.4f", c)
	case c < 1:
		return fmt.Sprintf("$%.3f", c)
	}
	return fmt.Sprintf("$%.2f", c)
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinFrame(i int) string {
	n := len(spinner)
	return spinner[(i%n+n)%n]
}

// slowCall is a call that took at least half of its limit, or ten seconds.
func slowCall(t Turn) bool {
	return t.TimedOut || t.ElapsedMs >= 10000 || t.LimitMs > 0 && t.ElapsedMs*2 >= t.LimitMs
}

func plural(n int, one string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %ss", n, one)
}

// SummaryLines is what closes a run: its outcome and duration, how many
// requests and tool calls it took, and on the right the tokens, cache hits and
// cost, all on one line. Parts that do not fit are dropped, the right side
// first. A failed run says why in a line above.
func SummaryLines(run *RunStats, w int) []Line {
	took := time.Duration(run.ElapsedMs) * time.Millisecond
	glyph, color, head := "✓", Green, FmtDur(took)
	switch run.Status {
	case RunFail:
		glyph, color, head = "✕", Red, "failed after "+head
	case RunStop:
		glyph, color, head = "■", Yellow, "stopped after "+head
	}
	rest := []string{}
	if run.Turns > 0 {
		rest = append(rest, plural(run.Turns, "turn"))
	}
	if run.ToolCalls > 0 {
		rest = append(rest, plural(run.ToolCalls, "tool call"))
	}
	if run.ToolMs >= 1000 {
		rest = append(rest, FmtDur(time.Duration(run.ToolMs)*time.Millisecond)+" in tools")
	}
	var right []string
	if u := run; u.In > 0 || u.Out > 0 {
		right = append(right, "↓"+FmtTokens(u.In), "↑"+FmtTokens(u.Out))
		if u.In > 0 {
			right = append(right, fmt.Sprintf("↺%.0f%%", 100*float64(min(max(0, u.Cached), u.In))/float64(u.In)))
		}
	}
	if run.Cost != nil {
		right = append(right, FmtCost(*run.Cost))
	}
	rightText := strings.Join(right, " ")
	var out []Line
	if run.Status == RunFail && run.Err != "" {
		for i, l := range Wrap(run.Err, max(1, w-4)) {
			mark := "  "
			if i == 0 {
				mark = "! "
			}
			out = append(out, Line{Segs: []Seg{{Text: "  "}, {Text: mark, FG: Red, Bold: true}, {Text: l, FG: Red}}, BG: BgTerm})
		}
	}
	lead := 2 + 2 // indent and glyph
	for {
		left := head
		if len(rest) > 0 {
			left += " · " + strings.Join(rest, " · ")
		}
		need := lead + TextWidth(left)
		if rightText != "" {
			need += 1 + 3 + 1 + TextWidth(rightText) // rule of at least three cells
		}
		if need <= w || (rightText == "" && len(rest) == 0) {
			break
		}
		switch {
		case rightText != "":
			rightText = ""
		default:
			rest = rest[:len(rest)-1]
		}
	}
	segs := []Seg{{Text: "  "}, {Text: glyph + " ", FG: color}, {Text: head, FG: color, Bold: true}}
	for _, p := range rest {
		segs = append(segs, Seg{Text: " · ", FG: FgDim}, Seg{Text: p, FG: FgDim})
	}
	if rightText != "" {
		gap := w - segsWidth(segs) - 2 - TextWidth(rightText)
		segs = append(segs, Seg{Text: " " + strings.Repeat("─", max(1, gap)) + " ", FG: FgDim}, Seg{Text: rightText, FG: FgDim})
	}
	segs = clipSegs(segs, max(1, w))
	return append(out, Line{Segs: segs, BG: BgTerm})
}
