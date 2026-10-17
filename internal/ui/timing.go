package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
)

// TimingMode says which js calls show how long they took.
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

// barCells is the width of the progress bar.
const barCells = 20

// bar draws a progress bar of n cells for frac (0..1). It is full only at 100%
// and never empty above 0, so a tiny or nearly complete share still shows.
func bar(frac float64, n int, fill tcell.Color) []Seg {
	if frac != frac || frac < 0 {
		frac = 0
	}
	frac = min(frac, 1)
	f := int(frac*float64(n) + 0.5)
	if frac > 0 && f == 0 {
		f = 1
	}
	if frac < 1 && f == n {
		f = n - 1
	}
	return []Seg{{Text: strings.Repeat("█", f), FG: fill}, {Text: strings.Repeat("░", n-f), FG: FgDim}}
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinFrame(i int) string {
	n := len(spinner)
	return spinner[(i%n+n)%n]
}

// slowCall is a call that took at least half of its limit, or ten seconds.
func slowCall(t agent.Turn) bool {
	return t.TimedOut || t.ElapsedMs >= 10000 || t.LimitMs > 0 && t.ElapsedMs*2 >= t.LimitMs
}

// timingFooter is the line under a js call: how long it ran, and how close it
// came to its limit. ok is false when nothing should be shown.
func timingFooter(t agent.Turn, js JSOpts) (segs []Seg, ok bool) {
	if js.Exec {
		d := js.Elapsed
		segs = []Seg{{Text: spinFrame(js.Spin), FG: Yellow}, {Text: " running ", FG: FgSoft}, {Text: FmtDur(d), FG: Yellow}}
		if js.Limit > 0 {
			segs = append(segs, Seg{Text: "   "})
			segs = append(segs, bar(float64(d)/float64(js.Limit), barCells, Yellow)...)
			segs = append(segs, Seg{Text: "  of " + FmtDur(js.Limit) + " limit", FG: FgDim})
		}
		return segs, js.Timing != TimingOff
	}
	if js.Timing == TimingOff || t.ElapsedMs <= 0 && !t.TimedOut {
		return nil, false
	}
	if js.Timing == TimingSlow && !slowCall(t) {
		return nil, false
	}
	took := time.Duration(t.ElapsedMs) * time.Millisecond
	limit := time.Duration(t.LimitMs) * time.Millisecond
	switch {
	case t.TimedOut:
		if limit <= 0 {
			limit = took
		}
		segs = []Seg{{Text: "◷", FG: Red}, {Text: " timed out after " + FmtDur(limit), FG: Red, Bold: true}, {Text: "   "}}
		segs = append(segs, bar(1, barCells, Red)...)
	case t.Err != "":
		segs = []Seg{{Text: "✕", FG: Red}, {Text: " " + FmtDur(took), FG: Red}}
	case slowCall(t):
		segs = []Seg{{Text: "✓", FG: Yellow}, {Text: " " + FmtDur(took), FG: Yellow}}
		if limit > 0 {
			frac := float64(took) / float64(limit)
			segs = append(segs, Seg{Text: "   "})
			segs = append(segs, bar(frac, barCells, Yellow)...)
			segs = append(segs, Seg{Text: fmt.Sprintf("  %d%% of %s limit", min(100, int(frac*100+0.5)), FmtDur(limit)), FG: FgDim})
		}
	default:
		segs = []Seg{{Text: "✓", FG: Green}, {Text: " " + FmtDur(took), FG: FgDim}}
	}
	return segs, true
}

// fitFooter drops what does not fit, last first: the limit text, then the whole
// bar. The duration itself always stays.
func fitFooter(segs []Seg, w int) []Seg {
	isBar := func(s Seg) bool { return strings.ContainsAny(s.Text, "█░") }
	for segsWidth(segs) > w && len(segs) > 2 {
		if !isBar(segs[len(segs)-1]) {
			segs = segs[:len(segs)-1]
			continue
		}
		for len(segs) > 2 && isBar(segs[len(segs)-1]) {
			segs = segs[:len(segs)-1]
		}
		if n := len(segs); n > 2 && strings.TrimSpace(segs[n-1].Text) == "" {
			segs = segs[:n-1]
		}
	}
	return segs
}

func plural(n int, one string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %ss", n, one)
}

// SummaryLines is what closes a run: its outcome and duration, how many
// requests and js calls it took, and on the right the tokens, cache hits and
// cost, all on one line. Parts that do not fit are dropped, the right side
// first. A failed run says why in a line above.
func SummaryLines(run *agent.RunStats, w int) []Line {
	took := time.Duration(run.ElapsedMs) * time.Millisecond
	glyph, color, head := "✓", Green, FmtDur(took)
	switch run.Status {
	case agent.RunFail:
		glyph, color, head = "✕", Red, "failed after "+head
	case agent.RunStop:
		glyph, color, head = "■", Yellow, "stopped after "+head
	}
	rest := []string{}
	if run.Turns > 0 {
		rest = append(rest, plural(run.Turns, "turn"))
	}
	if run.JSCalls > 0 {
		rest = append(rest, plural(run.JSCalls, "js call"))
	}
	if run.JSMs >= 1000 {
		rest = append(rest, FmtDur(time.Duration(run.JSMs)*time.Millisecond)+" in js")
	}
	var right []string
	if u := run.Usage; u.In > 0 || u.Out > 0 {
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
	if run.Status == agent.RunFail && run.Err != "" {
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
