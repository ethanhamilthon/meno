package app

import "time"

// ticker wakes the render loop twice a second while the agent is running.
func (a *App) ticker() {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-t.C:
			a.postEvent(tick{})
		}
	}
}
