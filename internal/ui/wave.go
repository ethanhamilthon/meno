package ui

import (
	"math"

	"github.com/gdamore/tcell/v2"
)

// Wave geometry. A wave has a hard left edge and a soft right fade; it moves to
// the right. The next wave starts when the previous hard edge passes
// waveSpawn of the width, so at most two waves are visible.
const (
	waveSpawn = 0.65
	waveSpeed = 30.0 // columns per second
)

func waveLen(n int) float64 { return math.Max(8, float64(n)*0.35) }

// WaveIntensity returns the glow (0..1) of each of n columns at time t seconds.
func WaveIntensity(n int, t float64) []float64 {
	if n <= 0 {
		return nil
	}
	length := waveLen(n)
	period := waveSpawn*float64(n) + length
	x := math.Mod(math.Max(0, t)*waveSpeed, period)
	out := make([]float64, n)
	for j := 0; j < 3; j++ {
		head := x - length + float64(j)*period
		if head >= float64(n) || head+length <= 0 {
			continue
		}
		for c := range out {
			d := float64(c) - head
			if d < 0 || d > length {
				continue
			}
			u := d / length
			out[c] = math.Max(out[c], 1-u*u*(3-2*u))
		}
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
