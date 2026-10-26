package tui

import (
	"time"

	"meno/internal/config"
	"meno/internal/render"
)

type themeLoaded struct {
	colors config.ThemeColors
	err    error
}

func (a *App) loadThemeFile() {
	render.ApplyPalette(render.DefaultPalette())
	colors, err := config.LoadTheme(a.paths.ThemeFile())
	if err != nil {
		a.warnings = append(a.warnings, err.Error())
		return
	}
	render.ApplyPalette(render.PaletteFromColors(colors.Colors()))
}

// themeWatcher reads only this tiny file, independently of daemon activity.
func (a *App) themeWatcher() {
	a.spawn(func() {
		timer := time.NewTicker(time.Second)
		defer timer.Stop()
		for {
			select {
			case <-a.done:
				return
			case <-timer.C:
				c, err := config.ReadTheme(a.paths.ThemeFile())
				a.postEvent(themeLoaded{c, err})
			}
		}
	})
}

func (a *App) onThemeLoaded(m themeLoaded) {
	if m.err != nil {
		warning := m.err.Error()
		if warning != a.themeWarning {
			a.themeWarning = warning
			a.idleTick = false
			a.notifyErr(warning)
		}
		return
	}
	a.themeWarning = ""
	p := render.PaletteFromColors(m.colors.Colors())
	if p != render.CurrentPalette() {
		render.ApplyPalette(p)
		a.idleTick = false
	}
}
