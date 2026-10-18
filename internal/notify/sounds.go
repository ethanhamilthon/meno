// Package notify supplies portable, synthesized notification sounds.
package notify

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

type preset struct {
	name     string
	notes    []float64
	duration float64
	harmonic float64
}

// Frequencies are in Hz; all presets are short, original synthesized cues.
var presets = []preset{
	{"chime", []float64{659, 831, 988}, .18, .25},
	{"bloom", []float64{392, 494, 587, 784}, .15, .15},
	{"droplet", []float64{1319, 880}, .11, .1},
	{"spark", []float64{1047, 1568, 2093}, .08, .2},
	{"soft-bell", []float64{784}, .65, .4},
	{"marimba", []float64{523, 659, 784}, .12, .5},
	{"orbit", []float64{440, 659, 880, 659}, .13, .2},
	{"ripple", []float64{988, 784, 659, 523}, .14, .1},
	{"ping", []float64{1760}, .25, .1},
	{"double-ping", []float64{1397, 1397}, .18, .15},
	{"triple-ping", []float64{1175, 1175, 1568}, .14, .1},
	{"doorbell", []float64{831, 659}, .32, .3},
	{"harp", []float64{262, 330, 392, 523, 659}, .1, .4},
	{"music-box", []float64{1047, 1319, 1175, 1568}, .18, .35},
	{"sunrise", []float64{330, 440, 554, 659}, .2, .15},
	{"moonlight", []float64{587, 440, 349}, .26, .1},
	{"pixel", []float64{523, 1047, 784}, .09, .7},
	{"arcade", []float64{659, 784, 1047, 1319}, .09, .65},
	{"sonar", []float64{880, 880}, .35, .05},
	{"woodblock", []float64{220, 330}, .09, .8},
	{"knock", []float64{165, 165, 220}, .1, .6},
	{"triangle", []float64{2349}, .6, .6},
	{"success", []float64{523, 659, 784, 1047}, .16, .25},
	{"attention", []float64{784, 587, 784}, .18, .4},
	{"gentle", []float64{349, 440}, .4, .05},
}

func Names() []string {
	names := make([]string, len(presets))
	for i, p := range presets {
		names[i] = p.name
	}
	return names
}

// WAV returns mono 16-bit PCM at 22050 Hz. A short attack and exponential
// decay keep cues quiet and prevent abrupt clicks at note boundaries.
func WAV(name string) ([]byte, error) {
	for _, p := range presets {
		if p.name == name {
			return render(p), nil
		}
	}
	return nil, fmt.Errorf("unknown notification sound %q", name)
}

func render(p preset) []byte {
	const rate = 22050
	n := int(p.duration * rate)
	samples := n * len(p.notes)
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(36+samples*2))
	out.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&out, binary.LittleEndian, v)
	}
	out.WriteString("data")
	_ = binary.Write(&out, binary.LittleEndian, uint32(samples*2))
	for _, hz := range p.notes {
		for i := 0; i < n; i++ {
			t := float64(i) / rate
			phase := 2 * math.Pi * hz * t
			envelope := math.Min(1, t/.008) * math.Exp(-5*t/p.duration) * math.Min(1, float64(n-1-i)/220)
			wave := (math.Sin(phase) + p.harmonic*math.Sin(phase*2.01)) / (1 + p.harmonic)
			_ = binary.Write(&out, binary.LittleEndian, int16(wave*envelope*10000))
		}
	}
	return out.Bytes()
}
