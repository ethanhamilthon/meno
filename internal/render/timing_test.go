package render

import (
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

func jsTurn(code string, mods func(*Turn)) Turn {
	t := Turn{Role: RoleAssistant, Text: "step", CallID: "c1", Code: code, Output: "out", ElapsedMs: 3200, LimitMs: 30000}
	if mods != nil {
		mods(&t)
	}
	return t
}
