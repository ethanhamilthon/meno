package app

import "meno/internal/agent"

// eventQueued tells the UI goroutine that an event for a session was stored.
type eventQueued struct{ sid string }

// enqueueEvent stores a steering event and wakes its session. It is safe to
// call from any goroutine; the event is persisted before anything wakes.
// Provenance (Source, From, Owner) is set by the caller's wiring, never parsed
// from text.
func (a *App) enqueueEvent(ev agent.Event) { a.enqueueEventAcked(ev, false) }

// enqueueEventAcked is enqueueEvent for an event whose content the model gets
// another way (a successful wait): it is stored, already acknowledged, and
// wakes nobody.
func (a *App) enqueueEventAcked(ev agent.Event, acked bool) {
	if a.evStore == nil {
		return
	}
	ok, err := a.evStore.Enqueue(ev)
	if err != nil || !ok {
		return
	}
	if acked {
		_ = a.evStore.Ack(ev.Session, ev.ID)
		return
	}
	a.postEvent(eventQueued{ev.Session})
}

// eventSource is the steering queue of a session as the runner sees it.
func (a *App) eventSource(sid string) agent.EventSource {
	if a.evStore == nil {
		return nil
	}
	return agent.StoreSource{Store: a.evStore, Session: sid}
}

// onEventQueued runs a continuation when the session is idle. A running
// session picks the event up before its next model request; one that waits for
// an answer keeps it queued until the form is resolved.
func (a *App) onEventQueued(sid string) {
	a.inRT(sid, a.wakeIfPending)
}

// wakeIfPending starts a continuation for the current session when events are
// waiting for it and nothing else is going on. Events of a cancelled request
// never wake it.
func (a *App) wakeIfPending() {
	rt := a.st.Runtime
	if a.evStore == nil || a.closing || !rt.Idle() {
		return
	}
	pending, err := a.evStore.Pending(rt.ID())
	if err != nil {
		return
	}
	for _, e := range pending {
		if rt.Cancelled[e.Owner] {
			_ = a.evStore.CancelOwner(rt.ID(), e.Owner)
			continue
		}
		if _, ok := a.currentProvider(); !ok {
			return
		}
		if e.Owner != "" {
			rt.RequestID = e.Owner
		}
		a.startTurn(agent.Message{})
		return
	}
}

// cancelRequest marks the active root request cancelled: its pending events
// are dropped and it can never auto-resume.
func (a *App) cancelRequest() {
	rt := a.st.Runtime
	if rt == nil || rt.RequestID == "" {
		return
	}
	if rt.Cancelled == nil {
		rt.Cancelled = map[string]bool{}
	}
	rt.Cancelled[rt.RequestID] = true
	if a.evStore != nil {
		_ = a.evStore.CancelOwner(rt.ID(), rt.RequestID)
	}
	if a.tasks != nil {
		a.tasks.CancelOwner(rt.ID(), rt.RequestID) // :stop is request-scoped
	}
	if a.kids != nil {
		a.kids.CancelOwner(rt.ID(), rt.RequestID)
	}
}

// onChildChanged refreshes what the UI shows about the children of a session:
// the delegated spend and the :agents list.
func (a *App) onChildChanged(parent string) {
	rt := a.rts[parent]
	if rt == nil || a.kids == nil {
		return
	}
	recs, _ := a.kids.store.List(parent, false)
	rt.Kids, rt.Delegated, rt.DelegatedUnpriced = recs, 0, false
	for _, r := range recs {
		live := r
		a.kids.mu.Lock()
		if c := a.kids.runs[r.ID]; c != nil {
			c.mu.Lock()
			live = c.rec
			c.mu.Unlock()
		}
		a.kids.mu.Unlock()
		rt.Delegated += live.Cost
		rt.DelegatedUnpriced = rt.DelegatedUnpriced || live.Unpriced
	}
}
