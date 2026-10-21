package engine

import "meno/internal/proto"

// globalSessionChange excludes streaming and accounting events before any
// attention query. Global consumers hydrate state only for meaningful changes.
func globalSessionChange(typ string, data any) bool {
	switch typ {
	case proto.EventStatus, proto.EventAskOpen, proto.EventAskClosed, proto.EventTasksChanged, proto.EventTurnAdded:
		return true
	case proto.EventSessionChanged:
		return true
	}
	return false
}

func (e *Engine) isPersisted(id string) bool {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persisted
}

func (e *Engine) publish(id, typ string, data any) {
	if e.events != nil {
		e.events.Publish(id, typ, data)
	}
	if e.global != nil && globalSessionChange(typ, data) && e.isPersisted(id) {
		completed, read := e.attention(id)
		e.global.Publish(id, proto.EventGlobalStatus, proto.GlobalStatus{SessionID: id, Running: e.running(id), NeedsYou: e.needsYou(id), DoneUnseen: completed > read})
	}
}
func (e *Engine) running(id string) bool {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
func (e *Engine) needsYou(id string) bool {
	e.mu.Lock()
	s := e.sessions[id]
	e.mu.Unlock()
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ask != nil
}
