package engine

import "meno/internal/api"

// announce tells every client on the global stream that the session directory
// changed, so lists and counts stay correct without a manual refresh.
func (e *Engine) announce(typ, id, parent string) {
	if e.global != nil {
		e.global.Publish(id, typ, api.SessionLifecycle{SessionID: id, ParentID: parent})
	}
}
