package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestWaveHardLeftSoftRight(t *testing.T) {
	const n = 100
	// Put the hard edge at column 20: x - length + 0 = 20.
	tm := (20 + waveLen(n)) / waveSpeed
	in := WaveIntensity(n, tm)
	if in[19] != 0 || in[20] < 0.99 {
		t.Fatalf("edge not hard: %v %v", in[19], in[20])
	}
	for c := 20; c < 20+int(waveLen(n)); c++ {
		if in[c+1] > in[c]+1e-9 {
			t.Fatalf("fade not monotonic at %d", c)
		}
	}
	if in[20+int(waveLen(n))+2] != 0 {
		t.Fatal("fade must end")
	}
}

func TestWaveSecondStartsAtSixtyFivePercent(t *testing.T) {
	const n = 100
	at := func(headCol float64) []float64 { return WaveIntensity(n, (headCol+waveLen(n))/waveSpeed) }
	// Before the first hard edge reaches column 65 only one wave is lit near the left.
	if got := at(60)[0]; got != 0 {
		t.Fatalf("second wave too early: %v", got)
	}
	if got := at(66)[0]; got == 0 {
		t.Fatal("second wave must have started")
	}
}

func TestWaveBounds(t *testing.T) {
	if WaveIntensity(0, 1) != nil || WaveIntensity(-3, 1) != nil {
		t.Fatal("empty width")
	}
	for _, n := range []int{1, 3, 80, 300} {
		for tm := 0.0; tm < 20; tm += 0.37 {
			for _, v := range WaveIntensity(n, tm) {
				if v < 0 || v > 1 {
					t.Fatalf("n=%d t=%v: %v", n, tm, v)
				}
			}
		}
	}
	if WaveColors(5, -1, tcell.ColorRed, tcell.ColorBlue) == nil {
		t.Fatal("negative time must work")
	}
}

func TestWaveColorsBetweenBaseAndGlow(t *testing.T) {
	cs := WaveColors(80, 1.3, FgDim, Green)
	lo, hi := FgDim, Green
	for _, c := range cs {
		r, g, b := c.RGB()
		lr, lg, lb := lo.RGB()
		hr, hg, hb := hi.RGB()
		in := func(v, a, b int32) bool { return v >= min(a, b) && v <= max(a, b) }
		if !in(r, lr, hr) || !in(g, lg, hg) || !in(b, lb, hb) {
			t.Fatalf("color %v outside base..glow", c)
		}
	}
}
