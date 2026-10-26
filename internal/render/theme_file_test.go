package render

import (
	"meno/internal/config"
	"testing"
)

func TestDefaultPaletteMatchesPanelDark(t *testing.T) {
	p := DefaultPalette()
	c := config.DefaultThemeColors()
	if p.BgTerm != hex(c.Bg) || p.Blue != hex(c.Blue) || p.FgDim != hex(c.Dim) {
		t.Fatal("panel-dark colors differ")
	}
}
