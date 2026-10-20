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
	return t.TimedOut || t.ElapsedMs >= 10000
}

// SummaryLines closes a run with lossless usage and execution metadata.
func SummaryLines(s *RunStats, w int) []Line {
	if s == nil {
		return nil
	}
	cache := "?/?"
	if s.CacheKnown || s.Cached > 0 || s.CacheWrite > 0 {
		cache = fmt.Sprintf("%d/%d", s.Cached, s.CacheWrite)
	}
	cost := "$?"
	if s.Cost != nil {
		cost = fmt.Sprintf("$%.4f", *s.Cost)
		if s.Partial {
			cost += "+?"
		}
	}
	left := fmt.Sprintf("in %d · out %d · cache %s · %s", s.In, s.Out, cache, cost)
	right := fmt.Sprintf("%s · %d bash", FmtDur(time.Duration(s.ElapsedMs)*time.Millisecond), s.ToolCalls)
	var out []Line
	if s.Status == "fail" {
		out = append(out, Plain("failed: "+s.Err, Red, BgTerm))
	} else if s.Status == "stop" {
		out = append(out, Plain("stopped", Orange, BgTerm))
	}
	if TextWidth(left)+TextWidth(right)+3 <= w {
		out = append(out, Plain(left+" "+strings.Repeat("─", max(1, w-TextWidth(left)-TextWidth(right)-2))+" "+right, FgDim, BgTerm))
	} else {
		out = append(out, WrapLines(left, max(1, w), func(s string) Line { return Plain(s, FgDim, BgTerm) })...)
		out = append(out, WrapLines(right, max(1, w), func(s string) Line { return Plain(s, FgDim, BgTerm) })...)
	}
	if len(out) > 0 {
		out[len(out)-1].CompletionAnchor = true
	}
	return out
}
