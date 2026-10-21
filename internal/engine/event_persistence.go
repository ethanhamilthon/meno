package engine

import "meno/internal/agent"

func isFragment(kind string) bool {
	return kind == agent.EvText || kind == agent.EvThink || kind == agent.EvCode
}

// persistEvent defines the durable boundaries of the runner event stream.
// In particular, JSEnd precedes the batch's appended turns: the following
// request/injection/terminal boundary must save those turns as well.
func persistEvent(kind string, appended bool) bool {
	if isFragment(kind) {
		return false
	}
	switch kind {
	case agent.EvRequest, agent.EvRequestRecord, agent.EvInjected,
		agent.EvJSEnd, agent.EvRetry, agent.EvError, agent.EvAborted, agent.EvDone:
		return true
	}
	return appended
}
