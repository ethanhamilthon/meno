package tui

import (
	"meno/internal/api"
)

// errNotice carries an error from a goroutine to the UI.
type errNotice struct{ text string }

// tick drives animations (the blinking activity dot) while the agent works.
type tick struct{}

type compactDone struct {
	sid  string
	resp api.CompactResponse
	err  error
}

type modelsLoaded struct {
	provider string
	response api.ModelsResponse
	err      error
}
