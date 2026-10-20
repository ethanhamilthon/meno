package ui

import (
	"strings"
	"testing"
	"time"
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

func turnOf(ms, limit int64, err string, timedOut bool) Turn {
	return Turn{Role: RoleAssistant, CallID: "c", Code: "x", Err: err, ElapsedMs: ms, LimitMs: limit, TimedOut: timedOut}
}

func footerText(t Turn, js JSOpts) (string, []Seg, bool) {
	segs, ok := timingBadge(t, js)
	return joinText(segs), segs, ok
}

func TestTimingBadgeOfFinishedCalls(t *testing.T) {
	all := JSOpts{}
	if got, segs, ok := footerText(turnOf(3200, 30000, "", false), all); !ok || got != "✓ 3.2s" || segs[0].FG != Green || segs[1].FG != FgDim {
		t.Fatalf("fast call: %q %v", got, ok)
	}
	if got, segs, _ := footerText(turnOf(17200, 30000, "", false), all); got != "✓ 17s" || segs[0].FG != Yellow {
		t.Fatalf("slow call: %q", got)
	}
	if got, _, _ := footerText(turnOf(12000, 0, "", false), all); got != "✓ 12s" {
		t.Fatalf("no limit: %q", got)
	}
	if got, segs, _ := footerText(turnOf(1200, 30000, "boom", false), all); got != "✕ 1.2s" || segs[0].FG != Red {
		t.Fatalf("failed call: %q", got)
	}
	if got, segs, _ := footerText(turnOf(30012, 30000, "js timeout after 30s", true), all); got != "✕ 30s" || segs[0].FG != Red {
		t.Fatalf("timeout: %q", got)
	}
}

func TestBadgeDur(t *testing.T) {
	for d, want := range map[time.Duration]string{-time.Second: "0.0s", 0: "0.0s", 412 * time.Millisecond: "0.4s", 2340 * time.Millisecond: "2.3s",
		9940 * time.Millisecond: "9.9s", 9960 * time.Millisecond: "10s", 17400 * time.Millisecond: "17s", 120 * time.Second: "120s", 1000 * time.Second: "16m 40s"} {
		if got := badgeDur(d); got != want {
			t.Errorf("badgeDur(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestTimingModes(t *testing.T) {
	fast, slow := turnOf(300, 30000, "", false), turnOf(20000, 30000, "", false)
	failedFast := turnOf(300, 30000, "boom", false)
	timeout := turnOf(30000, 30000, "x", true)
	legacy := Turn{Role: RoleAssistant, CallID: "c", Code: "x", Output: "old session"}
	notRun := Turn{Role: RoleAssistant, CallID: "c", Code: "x", Err: "execution canceled"}
	type row struct {
		turn      Turn
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
	if _, _, ok := footerText(Turn{CallID: "c", TimedOut: true}, JSOpts{}); !ok {
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

func TestTimingBadgeOfRunningCall(t *testing.T) {
	js := JSOpts{Exec: true, Elapsed: 2300 * time.Millisecond, Spin: 2}
	got, segs, ok := footerText(Turn{}, js)
	if !ok || got != "⠹ 2.3s" || segs[0].FG != Yellow {
		t.Fatalf("%q", got)
	}
	if got, _, _ := footerText(Turn{}, JSOpts{Exec: true, Elapsed: 2 * time.Second}); got != "⠋ 2.0s" {
		t.Fatalf("no limit: %q", got)
	}
	if _, _, ok := footerText(Turn{}, JSOpts{Exec: true, Timing: TimingOff}); ok {
		t.Fatal("running badge shown with timing off")
	}
	// Streaming code is not executing yet: no badge.
	if _, _, ok := footerText(Turn{}, JSOpts{Running: true}); ok {
		t.Fatal("badge while the code is still streaming")
	}
	frames := map[string]bool{}
	for i := -3; i < 25; i++ {
		frames[spinFrame(i)] = true
	}
	if len(frames) != len(spinner) {
		t.Fatalf("spinner frames %d", len(frames))
	}
}

func jsTurn(code string, mods func(*Turn)) Turn {
	t := Turn{Role: RoleAssistant, Text: "step", CallID: "c1", Code: code, Output: "out", ElapsedMs: 3200, LimitMs: 30000}
	if mods != nil {
		mods(&t)
	}
	return t
}

// asideTexts lists the badge text of every row that has one.
func asideTexts(lines []Line) []string {
	var out []string
	for _, l := range lines {
		if len(l.Aside) > 0 {
			out = append(out, joinText(l.Aside))
		}
	}
	return out
}

func TestBuildLinesPutsBadgeInTopRightOfCode(t *testing.T) {
	lines := BuildLines([]Turn{jsTurn("run()\nnext()", nil)}, 80, JSOpts{})
	texts := lineTexts(lines)
	i := -1
	for k, s := range texts {
		if strings.Contains(s, "run()") {
			i = k
		}
	}
	if i < 0 || len(lines[i].Aside) == 0 || joinText(lines[i].Aside) != "✓ 3.2s" || len(lines[i+1].Aside) != 0 || !strings.Contains(texts[i+2], "out") {
		t.Fatalf("%q %v", texts, asideTexts(lines))
	}
	if got := asideTexts(lines); len(got) != 1 {
		t.Fatalf("one badge per call: %v", got)
	}
	// Failed and timed out calls sit on the error background, code and badge alike.
	for _, mod := range []func(*Turn){
		func(t *Turn) { t.Err = "boom" },
		func(t *Turn) { t.Err, t.TimedOut = "js timeout after 30s", true },
	} {
		ls := BuildLines([]Turn{jsTurn("run()", mod)}, 80, JSOpts{})
		for k, l := range ls {
			if strings.Contains(l.Text(), "run()") || len(l.Aside) > 0 {
				if l.BG != BgJSErr {
					t.Fatalf("line %d %q on %v", k, l.Text(), l.BG)
				}
			}
		}
		if got := asideTexts(ls); len(got) != 1 || !strings.HasPrefix(got[0], "✕") {
			t.Fatalf("%v", got)
		}
	}
	// A running call keeps the running background and counts up.
	live := BuildLines([]Turn{{Role: RoleAssistant, Code: "run()"}}, 80, JSOpts{Running: true, Exec: true, Elapsed: time.Second})
	if got := asideTexts(live); len(got) != 1 || got[0] != "⠋ 1.0s" {
		t.Fatalf("live badge: %v", got)
	}
	for _, l := range live {
		if len(l.Aside) > 0 && l.BG != BgJSRun {
			t.Fatalf("live badge on %v", l.BG)
		}
	}
}

func TestBadgeGetsOwnRowWhenCodeIsLong(t *testing.T) {
	code := strings.Repeat("x", 60)
	lines := BuildLines([]Turn{jsTurn(code, nil)}, 40, JSOpts{})
	texts := lineTexts(lines)
	for k, l := range lines {
		if len(l.Aside) > 0 && (l.Text() != "" || k+1 >= len(texts) || !strings.Contains(texts[k+1], "xxxx")) {
			t.Fatalf("badge row %q is not an empty row above the code: %q", l.Text(), texts)
		}
	}
	if len(asideTexts(lines)) != 1 {
		t.Fatalf("%q", texts)
	}
}
