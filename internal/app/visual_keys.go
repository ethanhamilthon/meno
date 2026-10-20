package app

import "github.com/gdamore/tcell/v2"

func normalRune(k *tcell.EventKey) rune {
	switch k.Key() {
	case tcell.KeyRune:
		if k.Rune() == ' ' {
			return 'l'
		}
		return k.Rune()
	case tcell.KeyLeft, tcell.KeyBackspace, tcell.KeyBackspace2:
		return 'h'
	case tcell.KeyRight:
		return 'l'
	case tcell.KeyUp:
		return 'k'
	case tcell.KeyDown:
		return 'j'
	case tcell.KeyHome:
		return '0'
	case tcell.KeyDelete:
		return 'x'
	case tcell.KeyEnter:
		return '\n'
	}
	return 0
}

func (a *App) halfPage(full bool) int {
	_, h := a.scr.Size()
	if full {
		return max(1, h-a.bottomHeight()-1)
	}
	return max(1, (h-a.bottomHeight())/2)
}
