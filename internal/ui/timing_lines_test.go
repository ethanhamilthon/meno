package ui

import (
	"strings"
	"testing"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestBuildLinesTimingWithHiddenOrLimitedCode(t *testing.T) {
	long := "a()\nb()\nc()\nd()\ne()\nf()"
	lines := BuildLines([]agent.Turn{jsTurn(long, nil)}, 80, JSOpts{Mode: JSLines, Lines: 2})
	if got := asideTexts(lines); len(got) != 1 || got[0] != "✓ 3.2s" || len(lines[2].Aside) == 0 || !strings.Contains(lines[2].Text(), "a()") {
		t.Fatalf("%v %q", got, lineTexts(lines))
	}
	// With the code hidden a fine call shows no timing, a failure still does.
	hidden := JSOpts{Mode: JSOff}
	if got := BuildLines([]agent.Turn{jsTurn("run()", nil)}, 80, hidden); len(asideTexts(got)) != 0 || strings.Contains(strings.Join(lineTexts(got), "\n"), "run()") {
		t.Fatalf("%q", lineTexts(got))
	}
	failed := BuildLines([]agent.Turn{jsTurn("run()", func(t *agent.Turn) { t.Err = "boom" })}, 80, hidden)
	if got := asideTexts(failed); len(got) != 1 || got[0] != "✕ 3.2s" || !strings.Contains(strings.Join(lineTexts(failed), "\n"), "error: boom") {
		t.Fatalf("%v %q", got, lineTexts(failed))
	}
	// Timing off removes the badges altogether.
	if got := asideTexts(BuildLines([]agent.Turn{jsTurn("run()", nil)}, 80, JSOpts{Timing: TimingOff})); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	// The badge never makes a row wider than the screen.
	for w := 12; w < 70; w++ {
		for _, l := range BuildLines([]agent.Turn{jsTurn("r()", func(t *agent.Turn) { t.ElapsedMs = 20000 })}, w, JSOpts{}) {
			if len(l.Aside) > 0 && l.Width()+segsWidth(l.Aside) > w-PadX*2 {
				t.Fatalf("width %d: %q + %q do not fit", w, l.Text(), joinText(l.Aside))
			}
		}
	}
}

func run(status string, mods func(*agent.RunStats)) *agent.RunStats {
	cost := 0.084
	r := &agent.RunStats{Status: status, ElapsedMs: 84000, Turns: 7, ToolCalls: 6, ToolMs: 23000, Usage: llm.Usage{In: 48200, Out: 3100, Cached: 39524}, Cost: &cost}
	if mods != nil {
		mods(r)
	}
	return r
}

func TestSummaryOfRuns(t *testing.T) {
	ok := SummaryLines(run(agent.RunOK, nil), 100)
	if len(ok) != 1 {
		t.Fatalf("%q", lineTexts(ok))
	}
	got := ok[0].Text()
	if !strings.HasPrefix(got, "  ✓ 1m 24s · 7 turns · 6 tool calls · 23s in tools ─") || !strings.HasSuffix(got, "─ ↓48.2k ↑3.1k ↺82% $0.084") {
		t.Fatalf("%q", got)
	}
	if ok[0].Width() != 100 {
		t.Fatalf("the rule fills the row: %d", ok[0].Width())
	}
	if s := findSeg(t, ok, "✓"); s.FG != Green {
		t.Fatalf("%+v", s)
	}
	if s := findSeg(t, ok, "1m 24s"); !s.Bold || s.FG != Green {
		t.Fatalf("%+v", s)
	}
	fail := SummaryLines(run(agent.RunFail, func(r *agent.RunStats) { r.Err = "http 503: service unavailable"; r.ElapsedMs = 12000 }), 100)
	if len(fail) != 2 || !strings.HasPrefix(fail[0].Text(), "  ! http 503: service unavailable") || !strings.HasPrefix(fail[1].Text(), "  ✕ failed after 12s · 7 turns") {
		t.Fatalf("%q", lineTexts(fail))
	}
	if s := findSeg(t, fail, "✕"); s.FG != Red {
		t.Fatalf("%+v", s)
	}
	if s := findSeg(t, fail[:1], "http 503"); s.FG != Red {
		t.Fatalf("%+v", s)
	}
	stop := SummaryLines(run(agent.RunStop, func(r *agent.RunStats) { r.ElapsedMs = 8000 }), 100)
	if len(stop) != 1 || !strings.HasPrefix(stop[0].Text(), "  ■ stopped after 8.0s") {
		t.Fatalf("%q", lineTexts(stop))
	}
	if s := findSeg(t, stop, "■"); s.FG != Yellow {
		t.Fatalf("%+v", s)
	}
}

func TestSummaryLeavesOutWhatIsEmpty(t *testing.T) {
	bare := SummaryLines(&agent.RunStats{Status: agent.RunOK, ElapsedMs: 2100, Turns: 1}, 100)[0].Text()
	if strings.TrimRight(bare, " ") != "  ✓ 2.1s · 1 turn" {
		t.Fatalf("%q", bare)
	}
	one := SummaryLines(&agent.RunStats{Status: agent.RunOK, ElapsedMs: 2100, Turns: 1, ToolCalls: 1, ToolMs: 500}, 100)[0].Text()
	if !strings.Contains(one, "1 turn · 1 tool call") || strings.Contains(one, "calls") || strings.Contains(one, "in tools") {
		t.Fatalf("%q", one)
	}
	noCost := SummaryLines(run(agent.RunOK, func(r *agent.RunStats) { r.Cost = nil }), 100)[0].Text()
	if strings.Contains(noCost, "$") || !strings.Contains(noCost, "↺82%") {
		t.Fatalf("%q", noCost)
	}
	noTokens := SummaryLines(run(agent.RunOK, func(r *agent.RunStats) { r.Usage = llm.Usage{} }), 100)[0].Text()
	if strings.Contains(noTokens, "↓") || strings.Contains(noTokens, "↺") || !strings.HasSuffix(noTokens, "$0.084") {
		t.Fatalf("%q", noTokens)
	}
	nothing := SummaryLines(&agent.RunStats{Status: agent.RunOK}, 100)[0].Text()
	if strings.TrimRight(nothing, " ") != "  ✓ 0ms" {
		t.Fatalf("%q", nothing)
	}
	// More cached tokens than input tokens must not give a rate above 100%.
	over := SummaryLines(run(agent.RunOK, func(r *agent.RunStats) { r.Usage = llm.Usage{In: 100, Out: 1, Cached: 900} }), 100)[0].Text()
	if !strings.Contains(over, "↺100%") {
		t.Fatalf("%q", over)
	}
	negative := SummaryLines(run(agent.RunOK, func(r *agent.RunStats) { r.Usage = llm.Usage{In: 100, Out: 1, Cached: -5} }), 100)[0].Text()
	if !strings.Contains(negative, "↺0%") {
		t.Fatalf("%q", negative)
	}
}

func TestSummaryShrinksToFit(t *testing.T) {
	full := run(agent.RunOK, nil)
	prev := 1 << 30
	for w := 120; w >= 1; w-- {
		lines := SummaryLines(full, w)
		if len(lines) != 1 {
			t.Fatalf("width %d: %d lines", w, len(lines))
		}
		l := lines[0]
		if l.Width() > max(w, 1) {
			t.Fatalf("width %d: %d cells %q", w, l.Width(), l.Text())
		}
		if w >= 12 && !strings.Contains(l.Text(), "1m 24s") {
			t.Fatalf("width %d lost the time: %q", w, l.Text())
		}
		if prev < l.Width()-1 && w > 20 {
			// the summary only gets shorter as the room shrinks
			t.Fatalf("width %d grew to %d", w, l.Width())
		}
		prev = l.Width()
	}
	// The right side is the first to go, then the details from the end.
	for w, want := range map[int]string{100: "$0.084", 60: "23s in tools", 44: "6 tool calls", 30: "7 turns", 20: "1m 24s"} {
		if got := SummaryLines(full, w)[0].Text(); !strings.Contains(got, want) {
			t.Errorf("width %d lacks %q: %q", w, want, got)
		}
	}
	if got := SummaryLines(full, 50)[0].Text(); strings.Contains(got, "$") || strings.Contains(got, "↓") {
		t.Errorf("right side kept at 50: %q", got)
	}
}

func TestSummaryErrorWrapsAndStaysOneLine(t *testing.T) {
	long := run(agent.RunFail, func(r *agent.RunStats) { r.Err = strings.Repeat("connection reset by peer ", 12) })
	lines := SummaryLines(long, 40)
	if len(lines) < 4 {
		t.Fatalf("%q", lineTexts(lines))
	}
	for i, l := range lines {
		if l.Width() > 40 {
			t.Fatalf("line %d is %d wide", i, l.Width())
		}
	}
	if !strings.HasPrefix(lines[len(lines)-1].Text(), "  ✕ failed after") {
		t.Fatalf("%q", lineTexts(lines))
	}
	// A failure without a reason has no reason line.
	if got := SummaryLines(run(agent.RunFail, nil), 80); len(got) != 1 {
		t.Fatalf("%q", lineTexts(got))
	}
}

func TestBuildLinesAddsSummaryAfterTheTurnThatCarriesIt(t *testing.T) {
	turns := []agent.Turn{
		{Role: llm.RoleUser, Text: "do it"},
		jsTurn("run()", nil),
		{Role: llm.RoleAssistant, Text: "all done", Run: run(agent.RunOK, nil)},
		{Role: llm.RoleUser, Text: "again"},
	}
	texts := lineTexts(BuildLines(turns, 100, JSOpts{Summary: true}))
	at := -1
	for i, s := range texts {
		if strings.Contains(s, "all done") {
			at = i
		}
	}
	if at < 0 || texts[at+1] != "" || !strings.HasPrefix(texts[at+2], "  ✓ 1m 24s") {
		t.Fatalf("%q", texts)
	}
	if strings.Count(strings.Join(texts, "\n"), "✓ 1m 24s") != 1 {
		t.Fatal("one summary per run")
	}
	// The next message is separated from the summary.
	if texts[at+3] != "" {
		t.Fatalf("%q", texts[at:])
	}
	if got := strings.Join(lineTexts(BuildLines(turns, 100, JSOpts{})), "\n"); strings.Contains(got, "1m 24s") {
		t.Fatal("summary shown although switched off")
	}
	// A run that ended on an unanswered message still reports.
	failed := []agent.Turn{{Role: llm.RoleUser, Text: "q", Run: run(agent.RunFail, func(r *agent.RunStats) { r.Err = "no key" })}}
	got := strings.Join(lineTexts(BuildLines(failed, 100, JSOpts{Summary: true})), "\n")
	if !strings.Contains(got, "! no key") || !strings.Contains(got, "✕ failed after") {
		t.Fatalf("%s", got)
	}
	// A summary on an otherwise empty turn is the first thing on screen without a leading blank.
	only := BuildLines([]agent.Turn{{Role: llm.RoleAssistant, Run: run(agent.RunStop, nil)}}, 100, JSOpts{Summary: true})
	if len(only) != 1 || !strings.Contains(only[0].Text(), "■ stopped") {
		t.Fatalf("%q", lineTexts(only))
	}
}

func TestBuildLinesRendersMarkdownInAnswers(t *testing.T) {
	turns := []agent.Turn{
		{Role: llm.RoleAssistant, Text: "# Title\n\n- [x] done\n\n| a | b |\n|---|---|\n| 1 | 2 |"},
		jsTurn("go()", func(t *agent.Turn) { t.Text = "**before** the call" }),
	}
	texts := strings.Join(lineTexts(BuildLines(turns, 60, JSOpts{})), "\n")
	for _, want := range []string{"◆ Title", "━━━━━", "[x] done", "┌───┬───┐", "◇ before the call"} {
		if !strings.Contains(texts, want) {
			t.Fatalf("missing %q:\n%s", want, texts)
		}
	}
	if strings.Contains(texts, "**") || strings.Contains(texts, "|---") {
		t.Fatalf("raw markdown left:\n%s", texts)
	}
}
