package tui

import (
	"github.com/gdamore/tcell/v2"
	"meno/internal/render"
)

func chromeText() tcell.Color { return render.Mix(render.FgDim, render.BgTerm, 0.15) }
