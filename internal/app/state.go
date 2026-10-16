package app

import (
	"meno/internal/agent"
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
	OverPrompts
	OverResume
	OverHelp
	OverPromptEdit
)

type State struct {
	Mode    ui.Mode
	Over    Overlay
	Input   []rune
	Cursor  int
	Cmd     []rune
	Scroll  int
	Sel     int
	Query   []rune
	Status  string
	Running bool

	Settings  config.Settings
	Providers []llm.Provider
	Skills    []config.Skill
	Servers   []mcp.Server
	Prompts   []config.Prompt
	Sessions  []agent.SessionInfo

	Session *agent.Session
	Visual  int
	Yank    string
	Field   int
	Editing bool
	Buffer  []rune

	Usage   llm.Usage
	Pending any
	Toast   string

	EditIdx    int
	EditProv   llm.Provider
	EditMCP    mcp.Server
	EditPrompt config.Prompt
}

func NewState() *State {
	return &State{Mode: ui.ModeInsert, Settings: config.DefaultSettings()}
}
