package engine

import "meno/internal/proto"

// announce tells every client on the global stream that the session directory
// changed, so lists and counts stay correct without a manual refresh.
func (e *Engine) announce(typ, id, parent string) {
	if e.global != nil {
		e.global.Publish(id, typ, proto.SessionLifecycle{SessionID: id, ParentID: parent})
	}
}
