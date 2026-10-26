package engine

import (
	"testing"

	"meno/internal/config"
)

func TestElideOptionsFollowTheSettings(t *testing.T) {
	for _, c := range []struct{ elide, compact, want int }{{50, 80, 50}, {79, 80, 79}, {80, 80, 0}, {100, 80, 0}, {10, 95, 10}} {
		got := elideOptions(config.Settings{ElidePercent: c.elide, CompactPercent: c.compact})
		if got.Percent != c.want {
			t.Errorf("elide %d compact %d: percent %d, want %d (0 is off)", c.elide, c.compact, got.Percent, c.want)
		}
	}
}
