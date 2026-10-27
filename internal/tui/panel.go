package tui

import (
	"github.com/gdamore/tcell/v2"
	"meno/internal/render"
)

func (a *App) panelOpen() bool { return a.st.Over != OverNone || a.st.Ask != nil }

// A bounded panel keeps the conversation visible; long lists scroll internally.
func (a *App) panelBounds(w, h int) render.PanelBounds {
	if !a.panelOpen() {
		return render.PanelBounds{}
	}
	desired := 13
	switch a.st.Over {
	case OverModel:
		desired = 16
	case OverMCP, OverMCPEdit:
		desired = 17
	case OverHelp, OverContext, OverTasks:
		desired = max(13, h/2)
	}
	if a.st.Over == OverNone && a.st.Ask != nil {
		desired = 16
	}
	return render.ComposerPanel(w, h, a.composerHeightFor(w), desired)
}

func (a *App) panelBox(w, h, _, _ int) box {
	if a.panelHits == nil {
		a.panelHits = map[int]int{}
	}
	p := a.panelBounds(w, h)
	return box{x: p.X, y: p.Y + 1, w: p.Width, h: max(0, p.Height-1), oy: p.Y, oh: p.Height}
}

// panelMouse uses the painted row hit map, never document coordinates.
func (a *App) panelMouse(m *tcell.EventMouse) {
	x, y := m.Position()
	w, h := a.scr.Size()
	p := a.panelBounds(w, h)
	if x < p.X || x >= p.X+p.Width || y < p.Y || y >= p.Y+p.Height {
		return
	}
	switch {
	case m.Buttons()&tcell.WheelUp != 0:
		a.onOverlayKey(tcell.NewEventKey(tcell.KeyUp, 0, 0))
	case m.Buttons()&tcell.WheelDown != 0:
		a.onOverlayKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	case m.Buttons()&tcell.Button1 != 0:
		if hit, ok := a.panelHits[y]; ok {
			if a.st.Over == OverProviderEdit || a.st.Over == OverMCPEdit {
				a.st.Field = hit
			} else {
				a.st.Sel = hit
			}
		}
	}
}

func (a *App) recordItems(b box, y int, items []item, sel int) {
	capacity := max(0, b.h-y-2)
	start, used := 0, 0
	for i := 0; i <= sel && i < len(items); i++ {
		used++
		if i == sel && items[i].detail != "" {
			used++
		}
		for used > capacity && start < i {
			start++
			used--
		}
	}
	for i := start; i < len(items) && y < b.h-2; i++ {
		a.panelHits[b.y+y] = i
		y++
		if i == sel && items[i].detail != "" && y < b.h-2 {
			a.panelHits[b.y+y] = i
			y++
		}
	}
}
