package app

import "meno/internal/modes"

// modeCatalog is the list of modes meno has.
func (a *App) modeCatalog() []modes.Mode { return modes.Catalog() }

func findMode(name string) (modes.Mode, bool) { return modes.Find(modes.Catalog(), name) }

func hasName(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
