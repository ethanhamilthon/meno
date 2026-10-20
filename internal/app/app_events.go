package app

import (
	"meno/internal/llm"
	"meno/internal/proto"
)

// tick drives animations (the blinking activity dot) while the agent works.
type tick struct{}

type compactDone struct {
	sid  string
	resp proto.CompactResponse
	err  error
}

// sessionRefreshed carries a fresh snapshot fetched after a change notice.
type sessionRefreshed struct {
	sid  string
	snap proto.Snapshot
	err  error
}
type sessionsLoaded struct {
	list []proto.SessionItem
	err  error
}
type modelsLoaded struct {
	provider string
	models   []llm.Model
	err      error
}
