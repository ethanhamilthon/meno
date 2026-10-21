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

type modelsLoaded struct {
	provider string
	response proto.ModelsResponse
	err      error
}
