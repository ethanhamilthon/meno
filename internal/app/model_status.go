package app

// emptyModelText separates a search with no matches from loading and errors.
func (a *App) emptyModelText() string {
	if len(a.allModels()) > 0 {
		return "no models match"
	}
	active := false
	for _, p := range a.st.Providers {
		if !p.Active() {
			continue
		}
		active = true
		if a.st.Loading[p.Name] {
			return "loading models…"
		}
	}
	for _, p := range a.st.Providers {
		if p.Active() && a.st.ModelErr[p.Name] != "" {
			return p.Name + ": " + briefReason(a.st.ModelErr[p.Name])
		}
	}
	if !active {
		return "no active providers · use :login"
	}
	return "no models available"
}
