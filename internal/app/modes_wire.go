package app

import "meno/internal/modes"

// modeCatalog is the list of modes meno has.
func (a *App) modeCatalog() []modes.Mode { return modes.Catalog() }

func findMode(name string) (modes.Mode, bool) { return modes.Find(modes.Catalog(), name) }

// activeModes returns the modes of the current session, in the order they were named.
func (a *App) activeModes() []modes.Mode {
	var out []modes.Mode
	for _, n := range a.st.Session.Modes {
		if m, ok := findMode(n); ok {
			out = append(out, m)
		}
	}
	return out
}

func hasName(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
