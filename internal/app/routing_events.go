package app

// handleRoutingEvent applies routing state and its user-facing notice.
func (a *App) handleRoutingEvent(event any) {
	switch m := event.(type) {
	case routingStateEv:
		a.inRT(m.sid, func() { a.st.EffortSticky = m.sticky })
	case preparedEv:
		a.inRT(m.sid, func() {
			a.onPrepared(m.p)
			a.st.RunEffort = m.effort
			if m.routeNotice != "" {
				a.notify(m.routeNotice)
			}
		})
	}
}
