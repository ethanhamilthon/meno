package app

import (
	"meno/internal/agent"
	"meno/internal/cliproxy"
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

// accountsLoaded carries the OAuth accounts listed by the local helper.
type accountsLoaded struct {
	list []cliproxy.Account
	err  error
}

// oauthRefreshed is the outcome of the start-up look for a newer helper.
type oauthRefreshed struct {
	out cliproxy.Outcome
	err error
}

// The messages of a running login; id tells a stale login from the current one.
type flowStep struct {
	id   int
	text string
}

type flowLogin struct {
	id    int
	login cliproxy.Login
	err   error
}

type flowDone struct {
	id  int
	err error
}
