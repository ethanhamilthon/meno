package app

import (
	"github.com/gdamore/tcell/v2"
	"meno/internal/ui"
)

func chromeText() tcell.Color { return ui.Mix(ui.FgDim, ui.BgTerm, 0.15) }
