package app

import (
	"time"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/ui"
)

type Overlay int

const (
	OverNone Overlay = iota
	OverModel
	OverSettings
	OverProviders
	OverProviderEdit
	OverSkills
	OverMCP
	OverMCPEdit
	OverSessions
	OverTheme
	OverHelp
)

// liveCall is a tool call that is not in the session yet: its code is still
// streaming from the model, or it is executing.
type liveCall struct {
	ID      string
	Code    string
	Running bool
}

// retryState describes the pending retry shown in the status bar.
type retryState struct {
	Attempt, Max int
	Until        time.Time
	Reason       string
}

// State is the UI state. The fields of the session on screen live in the
// embedded Runtime, so a.st.Running, a.st.Input... always mean "the current
// session"; switching sessions swaps the pointer.
type State struct {
	*Runtime

	Over    Overlay
	CmdSel  int
	Pricing map[string]llm.ModelInfo
	Cmd     []rune
	Sel     int
	Query   []rune
	Status  string

	Settings  config.Settings
	Providers []llm.Provider
	Models    map[string][]llm.Model // provider name -> fetched model ids
	ModelErr  map[string]string      // provider name -> last fetch error
	Loading   map[string]bool
	Skills    []config.Skill
	Servers   []mcp.Server

	Field   int
	Editing bool
	Buffer  []rune

	Notices []notice

	// Vim registers and find state are shared by all sessions.
	Reg       string // unnamed register
	RegLine   bool   // register holds whole lines
	LastFind  string // f F t T of the last find, for ; and ,
	LastFindC rune

	EditIdx  int
	EditProv llm.Provider
	EditMCP  mcp.Server
}

func NewState() *State {
	return &State{
		Runtime:  &Runtime{Mode: ui.ModeInsert},
		Settings: config.DefaultSettings(),
		Models:   map[string][]llm.Model{},
		ModelErr: map[string]string{},
		Loading:  map[string]bool{},
	}
}
