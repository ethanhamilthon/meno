package app

import "time"

func (a *App) persistProviders() {
	if err := a.store.SaveProviders(a.st.Providers); err != nil {
		a.st.Toast = err.Error()
	}
}

func (a *App) persistServers() {
	if err := a.store.SaveServers(a.st.Servers); err != nil {
		a.st.Toast = err.Error()
	}
}

func (a *App) persistSkills() {
	if err := a.store.SaveSkills(a.st.Skills); err != nil {
		a.st.Toast = err.Error()
	}
}

func (a *App) persistPrompts() {
	if err := a.store.SavePrompts(a.st.Prompts); err != nil {
		a.st.Toast = err.Error()
	}
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m ago"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h ago"
	}
	return itoa(int(d.Hours()/24)) + "d ago"
}
