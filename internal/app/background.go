package app

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/safe"
	"meno/internal/ui"
)

// bgEvery is how often the background work of the sessions is counted.
const bgEvery = time.Second

// bgLoaded carries how many background tasks and subagents each session has
// running.
type bgLoaded struct{ counts map[string]int }

// bgSessionIDs are the sessions whose background work is worth counting: the
// loaded ones, and every listed one while the session picker is open.
func (a *App) bgSessionIDs() []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, rt := range a.rtList {
		if rt.Session != nil {
			add(rt.Session.ID)
		}
	}
	if a.st.Over == OverSessions {
		for _, s := range a.dbRows {
			add(s.ID)
		}
	}
	return ids
}

// refreshBackground recounts running tasks and working subagents off the UI
// goroutine; one count at a time, at most once per bgEvery.
func (a *App) refreshBackground() {
	if a.bgInflight || time.Since(a.bgAt) < bgEvery || (a.tasks == nil && a.kids == nil) {
		return
	}
	ids := a.bgSessionIDs()
	if len(ids) == 0 {
		return
	}
	a.bgInflight, a.bgAt = true, time.Now()
	a.running.Add(1)
	go func() {
		defer safe.Recover(a.reportPanic)
		defer a.running.Done()
		counts := map[string]int{}
		for _, id := range ids {
			if a.tasks != nil {
				if rows, err := a.tasks.List(id, false); err == nil {
					counts[id] += len(rows)
				}
			}
			if a.kids != nil {
				if recs, err := a.kids.store.List(id, true); err == nil {
					for _, r := range recs {
						if r.Working() {
							counts[id]++
						}
					}
				}
			}
		}
		a.postEvent(bgLoaded{counts})
	}()
}

func (a *App) onBgLoaded(m bgLoaded) {
	a.bgInflight = false
	a.bg = m.counts
}

// bgCount is the number of running background tasks and working subagents of a
// session, as last counted.
func (a *App) bgCount(sid string) int { return a.bg[sid] }

// bgAny reports whether any session has background work, which keeps the
// purple marker blinking.
func (a *App) bgAny() bool {
	for _, n := range a.bg {
		if n > 0 {
			return true
		}
	}
	return false
}

// bgColor is the blinking purple of the background-work marker.
func bgColor() tcell.Color { return dimOr(blinkOn(), ui.Purple) }

// bgBadge is the marker text for n units of background work, "" for none.
func bgBadge(n int) string {
	if n <= 0 {
		return ""
	}
	return "◆ " + itoa(n)
}
