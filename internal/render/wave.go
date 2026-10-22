package render

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

// Wave geometry. A wave is a raised-cosine pulse: it rises and falls smoothly
// on both sides, and it travels to the right around a ring as wide as the
// field, so it leaves at the right edge and enters at the left without a jump.
const waveSpeed = 30.0 // columns per second

func waveLen(n int) float64 { return math.Max(8, float64(n)*0.35) }

// WaveIntensity returns the glow (0..1) of each of n columns at time t seconds.
func WaveIntensity(n int, t float64) []float64 {
	if n <= 0 {
		return nil
	}
	length := waveLen(n)
	period := math.Max(float64(n), length)
	head := math.Mod(math.Max(0, t)*waveSpeed, period)
	out := make([]float64, n)
	for c := range out {
		// Distance behind the head, measured around the ring.
		d := math.Mod(head-float64(c)-0.5, period)
		if d < 0 {
			d += period
		}
		if d >= length {
			continue
		}
		out[c] = 0.5 * (1 - math.Cos(2*math.Pi*d/length))
	}
	return out
}

// WaveColors blends base toward glow by WaveIntensity, one color per column.
func WaveColors(n int, t float64, base, glow tcell.Color) []tcell.Color {
	in := WaveIntensity(n, t)
	out := make([]tcell.Color, len(in))
	for i, v := range in {
		out[i] = mix(base, glow, v)
	}
	return out
}
