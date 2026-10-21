package ui

import (
	"math"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Both edges are smooth: neighbouring columns never differ by more than the
// steepest slope of a raised cosine, for several widths and moments.
func TestWaveEdgesAreContinuous(t *testing.T) {
	for _, n := range []int{20, 80, 200} {
		limit := math.Pi/waveLen(n) + 1e-9
		for tm := 0.0; tm < 12; tm += 0.13 {
			in := WaveIntensity(n, tm)
			for c := 1; c < n; c++ {
				if math.Abs(in[c]-in[c-1]) > limit {
					t.Fatalf("n=%d t=%.2f: jump %v at column %d", n, tm, in[c]-in[c-1], c)
				}
			}
		}
	}
}

// The pulse leaves at the right edge and comes back at the left without a jump.
func TestWaveWrapsWithoutAJump(t *testing.T) {
	const n = 80
	limit := math.Pi/waveLen(n) + 1e-9
	for tm := 0.0; tm < 12; tm += 0.07 {
		in := WaveIntensity(n, tm)
		if math.Abs(in[0]-in[n-1]) > limit {
			t.Fatalf("t=%.2f: wrap jump %v", tm, in[0]-in[n-1])
		}
	}
}

// One pulse peaks near 1, is dark outside its length and moves at waveSpeed.
func TestWavePulseShapeAndSpeed(t *testing.T) {
	const n = 200
	peak := func(tm float64) int {
		best, at := -1.0, 0
		for c, v := range WaveIntensity(n, tm) {
			if v > best {
				best, at = v, c
			}
		}
		return at
	}
	top, lit := 0.0, 0
	for _, v := range WaveIntensity(n, 2) {
		top = math.Max(top, v)
		if v > 0 {
			lit++
		}
	}
	if top < 0.99 {
		t.Fatalf("peak %v", top)
	}
	if float64(lit) > waveLen(n)+1 {
		t.Fatalf("%d lit columns, the pulse is %v long", lit, waveLen(n))
	}
	got := peak(2.5) - peak(2.0)
	if want := int(waveSpeed / 2); got < want-1 || got > want+1 {
		t.Fatalf("peak moved %d columns, want about %d", got, want)
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
