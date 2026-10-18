package app

import (
	"time"

	"meno/internal/config"
)

func (a *App) persistProviders() {
	if err := a.store.SaveProviders(config.OwnProviders(a.st.Providers)); err != nil {
		a.notifyErr(err.Error())
	}
}

func (a *App) persistServers() {
	if err := a.store.SaveServers(config.OwnServers(a.st.Servers)); err != nil {
		a.notifyErr(err.Error())
	}
}

func (a *App) persistSkills() {
	if err := a.store.SaveSkills(a.st.Skills); err != nil {
		a.notifyErr(err.Error())
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
