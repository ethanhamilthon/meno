package app

import (
	"meno/internal/agent"
	"meno/internal/llm"
)

type agentEv struct {
	sid string
	ev  agent.Ev
}
type turnDone struct {
	sid string
	err error
}

// preparedEv carries the prompt state and system event of a message that is
// about to be sent.
type preparedEv struct {
	sid string
	p   prepared
}

// tick drives animations (the blinking activity dot) while the agent works.
type tick struct{}

type compactDone struct {
	sid           string
	before, after int
	err           error
}
type sessionsLoaded struct {
	list []agent.SessionInfo
	err  error
}
type modelsLoaded struct {
	provider string
	models   []llm.Model
	err      error
}
