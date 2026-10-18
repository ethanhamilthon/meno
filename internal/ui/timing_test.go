package ui

import (
	"strings"
	"testing"
	"time"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestFmtDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{-5 * time.Second, "0ms"}, {0, "0ms"}, {400 * time.Microsecond, "0ms"}, {time.Millisecond, "1ms"},
		{412 * time.Millisecond, "412ms"}, {999 * time.Millisecond, "999ms"}, {999600 * time.Microsecond, "1.0s"},
		{time.Second, "1.0s"}, {3240 * time.Millisecond, "3.2s"}, {3250 * time.Millisecond, "3.3s"}, {9940 * time.Millisecond, "9.9s"},
		{9960 * time.Millisecond, "10s"}, {10 * time.Second, "10s"}, {17400 * time.Millisecond, "17s"}, {59400 * time.Millisecond, "59s"},
		{59600 * time.Millisecond, "1m 00s"}, {84 * time.Second, "1m 24s"}, {64 * time.Second, "1m 04s"}, {59*time.Minute + 59*time.Second, "59m 59s"},
		{3599600 * time.Millisecond, "1h 00m"}, {3725 * time.Second, "1h 02m"}, {26 * time.Hour, "26h 00m"},
	}
	for _, c := range cases {
		if got := FmtDur(c.d); got != c.want {
			t.Errorf("FmtDur(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestFmtTokensAndCost(t *testing.T) {
	for n, want := range map[int]string{-3: "0", 0: "0", 950: "950", 999: "999", 1000: "1k", 1400: "1.4k", 9999: "10k", 10500: "10.5k", 12400: "12.4k",
		48200: "48.2k", 99999: "100k", 100000: "100k", 250400: "250k", 999499: "999k", 999500: "1m", 1200000: "1.2m", 15000000: "15m"} {
		if got := FmtTokens(n); got != want {
			t.Errorf("FmtTokens(%d) = %q, want %q", n, got, want)
		}
	}
	for c, want := range map[float64]string{0: "$0.0000", 0.0021: "$0.0021", 0.00999: "$0.0100", 0.01: "$0.010", 0.084: "$0.084", 0.9994: "$0.999", 1: "$1.00", 12.345: "$12.35"} {
		if got := FmtCost(c); got != want {
			t.Errorf("FmtCost(%v) = %q, want %q", c, got, want)
		}
	}
}

func barText(segs []Seg) (full, empty int) {
	for _, s := range segs {
		full += strings.Count(s.Text, "█")
		empty += strings.Count(s.Text, "░")
	}
	return
}

func TestBar(t *testing.T) {
	nan := 0.0
	nan = nan / nan
	cases := []struct {
		frac float64
		full int
	}{{0, 0}, {-1, 0}, {nan, 0}, {0.001, 1}, {0.02, 1}, {0.49, 10}, {0.5, 10}, {0.97, 19}, {0.999, 19}, {1, 20}, {1.5, 20}, {100, 20}}
	for _, c := range cases {
		full, empty := barText(bar(c.frac, 20, Yellow))
		if full != c.full || full+empty != 20 {
			t.Errorf("bar(%v): %d full %d empty, want %d full of 20", c.frac, full, empty, c.full)
		}
	}
}

func turnOf(ms, limit int64, err string, timedOut bool) agent.Turn {
	return agent.Turn{Role: llm.RoleAssistant, CallID: "c", Code: "x", Err: err, ElapsedMs: ms, LimitMs: limit, TimedOut: timedOut}
}

func footerText(t agent.Turn, js JSOpts) (string, []Seg, bool) {
	segs, ok := timingFooter(t, js)
	return joinText(segs), segs, ok
}

func TestTimingFooterOfFinishedCalls(t *testing.T) {
	all := JSOpts{}
	if got, segs, ok := footerText(turnOf(3200, 30000, "", false), all); !ok || got != "✓ 3.2s" || segs[0].FG != Green || segs[1].FG != FgDim {
		t.Fatalf("fast call: %q %v", got, ok)
	}
	got, segs, _ := footerText(turnOf(17200, 30000, "", false), all)
	if !strings.HasPrefix(got, "✓ 17s   ") || !strings.HasSuffix(got, "  57% of 30s limit") || segs[0].FG != Yellow || strings.Count(got, "█") != 11 {
		t.Fatalf("slow call: %q", got)
	}
	got, _, _ = footerText(turnOf(15000, 30000, "", false), all)
	if !strings.Contains(got, "50% of 30s limit") {
		t.Fatalf("half the limit is slow: %q", got)
	}
	got, segs, _ = footerText(turnOf(29990, 30000, "", false), all)
	if strings.Count(got, "█") != 19 || !strings.Contains(got, "100% of") || segs[0].FG != Yellow {
		t.Fatalf("nearly the limit: %q", got)
	}
	// No limit: slow only by the clock, and no bar.
	if got, _, _ = footerText(turnOf(12000, 0, "", false), all); got != "✓ 12s" {
		t.Fatalf("no limit: %q", got)
	}
	if got, segs, _ = footerText(turnOf(4000, 0, "", false), all); got != "✓ 4.0s" || segs[0].FG != Green {
		t.Fatalf("no limit, quick: %q", got)
	}
	got, segs, _ = footerText(turnOf(1200, 30000, "boom", false), all)
	if got != "✕ 1.2s" || segs[0].FG != Red {
		t.Fatalf("failed call: %q", got)
	}
	got, segs, _ = footerText(turnOf(30012, 30000, "js timeout after 30s", true), all)
	if !strings.HasPrefix(got, "◷ timed out after 30s   ") || strings.Count(got, "█") != 20 || segs[0].FG != Red || !segs[1].Bold {
		t.Fatalf("timeout: %q", got)
	}
	// Elapsed beyond the limit does not overflow the bar.
	if got, _, _ = footerText(turnOf(90000, 30000, "", false), all); strings.Count(got, "█")+strings.Count(got, "░") != 20 || !strings.Contains(got, "100% of 30s") {
		t.Fatalf("over the limit: %q", got)
	}
	// A timeout whose limit was not recorded still names a duration.
	if got, _, _ = footerText(turnOf(5000, 0, "x", true), all); !strings.Contains(got, "timed out after 5.0s") {
		t.Fatalf("%q", got)
	}
}

func TestTimingModes(t *testing.T) {
	fast, slow := turnOf(300, 30000, "", false), turnOf(20000, 30000, "", false)
	failedFast := turnOf(300, 30000, "boom", false)
	timeout := turnOf(30000, 30000, "x", true)
	legacy := agent.Turn{Role: llm.RoleAssistant, CallID: "c", Code: "x", Output: "old session"}
	notRun := agent.Turn{Role: llm.RoleAssistant, CallID: "c", Code: "x", Err: "execution canceled"}
	type row struct {
		turn      agent.Turn
		all, slow bool
	}
	for name, r := range map[string]row{"fast": {fast, true, false}, "slow": {slow, true, true}, "failed fast": {failedFast, true, false},
		"timeout": {timeout, true, true}, "legacy": {legacy, false, false}, "never ran": {notRun, false, false}} {
		if _, _, ok := footerText(r.turn, JSOpts{Timing: TimingAll}); ok != r.all {
			t.Errorf("%s: all → %v", name, ok)
		}
		if _, _, ok := footerText(r.turn, JSOpts{Timing: TimingSlow}); ok != r.slow {
			t.Errorf("%s: slow → %v", name, ok)
		}
		if _, _, ok := footerText(r.turn, JSOpts{Timing: TimingOff}); ok {
			t.Errorf("%s: shown although off", name)
		}
	}
	// A timeout with nothing measured is still news.
	if _, _, ok := footerText(agent.Turn{CallID: "c", TimedOut: true}, JSOpts{}); !ok {
		t.Fatal("timeout without a duration hidden")
	}
	// A very long call is slow even with no limit.
	if _, _, ok := footerText(turnOf(10000, 0, "", false), JSOpts{Timing: TimingSlow}); !ok {
		t.Fatal("10s without limit")
	}
	if _, _, ok := footerText(turnOf(9999, 0, "", false), JSOpts{Timing: TimingSlow}); ok {
		t.Fatal("9.9s without limit")
	}
}

func TestTimingFooterOfRunningCall(t *testing.T) {
	js := JSOpts{Exec: true, Elapsed: 12400 * time.Millisecond, Limit: 30 * time.Second, Spin: 2}
	got, segs, ok := footerText(agent.Turn{}, js)
	if !ok || !strings.HasPrefix(got, "⠹ running 12s   ") || !strings.HasSuffix(got, "  of 30s limit") || strings.Count(got, "█") != 8 || segs[0].FG != Yellow {
		t.Fatalf("%q", got)
	}
	if got, _, _ := footerText(agent.Turn{}, JSOpts{Exec: true, Elapsed: 2 * time.Second}); got != "⠋ running 2.0s" {
		t.Fatalf("no limit: %q", got)
	}
	if got, _, _ := footerText(agent.Turn{}, JSOpts{Exec: true, Elapsed: 50 * time.Second, Limit: 30 * time.Second}); strings.Count(got, "█") != 20 {
		t.Fatalf("past the limit: %q", got)
	}
	if _, _, ok := footerText(agent.Turn{}, JSOpts{Exec: true, Timing: TimingOff}); ok {
		t.Fatal("running footer shown with timing off")
	}
	// Streaming code is not executing yet: no footer.
	if _, _, ok := footerText(agent.Turn{}, JSOpts{Running: true}); ok {
		t.Fatal("footer while the code is still streaming")
	}
	frames := map[string]bool{}
	for i := -3; i < 25; i++ {
		frames[spinFrame(i)] = true
	}
	if len(frames) != len(spinner) {
		t.Fatalf("spinner frames %d", len(frames))
	}
}

func jsTurn(code string, mods func(*agent.Turn)) agent.Turn {
	t := agent.Turn{Role: llm.RoleAssistant, Text: "step", CallID: "c1", Code: code, Output: "out", ElapsedMs: 3200, LimitMs: 30000}
	if mods != nil {
		mods(&t)
	}
	return t
}

func TestBuildLinesPlacesFooterBetweenCodeAndOutput(t *testing.T) {
	lines := BuildLines([]agent.Turn{jsTurn("run()", nil)}, 80, JSOpts{})
	texts := lineTexts(lines)
	i := -1
	for k, s := range texts {
		if strings.Contains(s, "run()") {
			i = k
		}
	}
	if i < 0 || !strings.Contains(texts[i+1], "✓ 3.2s") || !strings.Contains(texts[i+2], "out") {
		t.Fatalf("%q", texts)
	}
	if lines[i+1].BG != BgJSDone || lines[i].BG != BgJSDone {
		t.Fatal("footer is part of the code block")
	}
	// Failed and timed out calls sit on the error background, code and footer alike.
	for _, mod := range []func(*agent.Turn){
		func(t *agent.Turn) { t.Err = "boom" },
		func(t *agent.Turn) { t.Err, t.TimedOut = "js timeout after 30s", true },
	} {
		ls := BuildLines([]agent.Turn{jsTurn("run()", mod)}, 80, JSOpts{})
		for k, l := range ls {
			if strings.Contains(l.Text(), "run()") || strings.Contains(l.Text(), "✕") || strings.Contains(l.Text(), "timed out") {
				if l.BG != BgJSErr {
					t.Fatalf("line %d %q on %v", k, l.Text(), l.BG)
				}
			}
		}
	}
	// A running call keeps the running background.
	live := BuildLines([]agent.Turn{{Role: llm.RoleAssistant, Code: "run()"}}, 80, JSOpts{Running: true, Exec: true, Elapsed: time.Second, Limit: time.Minute})
	for _, l := range live {
		if strings.Contains(l.Text(), "running") && l.BG != BgJSRun {
			t.Fatalf("live footer on %v", l.BG)
		}
	}
	if !strings.Contains(strings.Join(lineTexts(live), "\n"), "running 1.0s") {
		t.Fatal("no live footer")
	}
}
