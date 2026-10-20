package app

import (
	"meno/internal/proto"
)

// errNotice carries an error from a goroutine to the UI.
type errNotice struct{ text string }

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
	response proto.ModelsResponse
	err      error
}
