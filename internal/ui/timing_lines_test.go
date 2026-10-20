package ui

import (
	"strings"
	"testing"
)

func TestBashHeaderPersistsWhenSourceHidden(t *testing.T) {
	for _, mode := range []JSMode{JSOff, JSLines, JSFull} {
		lines := BuildLines([]Turn{jsTurn("echo hello", func(v *Turn) { v.Title = "Inspect files" })}, 80, JSOpts{Mode: mode, Lines: 1})
		text := summaryText(lines)
		if !strings.Contains(text, "Inspect files") || !strings.Contains(text, "3.2s") {
			t.Fatal(text)
		}
		headers := 0
		for _, l := range lines {
			if l.FullWidth {
				headers++
			}
		}
		if headers != 1 {
			t.Fatalf("headers=%d", headers)
		}
	}
}

func TestSummaryPreservesMetricsAtNarrowWidths(t *testing.T) {
	cost := 0.1234
	stats := &RunStats{Status: "done", ElapsedMs: 84000, ToolCalls: 7, In: 12000, Out: 2000, Cached: 8000, CacheWrite: 1000, CacheKnown: true, Cost: &cost}
	for _, w := range []int{20, 80, 160} {
		text := summaryText(SummaryLines(stats, w))
		for _, want := range []string{"12000", "2000", "8000/1000", "$0.1234", "7 bash"} {
			if !strings.Contains(text, want) {
				t.Fatalf("width %d missing %q: %s", w, want, text)
			}
		}
		for _, line := range SummaryLines(stats, w) {
			if TextWidth(line.Text()) > w {
				t.Fatalf("overflow %d", w)
			}
		}
	}
}

func summaryText(lines []Line) string {
	var out []string
	for _, l := range lines {
		out = append(out, l.Text())
	}
	return strings.Join(out, "\n")
}
