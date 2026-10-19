package app

import (
	"time"

	"meno/internal/cliproxy"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/modes"
	"meno/internal/ui"
)

type Overlay int

const (
	OverNone Overlay = iota
	OverModel
	OverSettings
	OverLogin
	OverLoginFlow
	OverProviderEdit
	OverSkills
	OverMCP
	OverMCPEdit
	OverSessions
	OverHelp
	OverFork
	OverTasks
	OverAgents
	OverSnippets
)

// liveCall is a tool call that is not in the session yet: its code is still
// streaming from the model, or it is executing.
type liveCall struct {
	ID      string
	Code    string
	Title   string
	Running bool
	// Started and Limit describe the execution: when it began and the timeout
	// it runs under (0 = none).
	Started time.Time
	Limit   time.Duration
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

	Over Overlay
	// UpdateTag is the newer release the background check found ("" if none).
	UpdateTag string
	TasksUI   tasksView
	AgentsUI  agentsView
	CmdSel    int
	Pricing   map[string]llm.ModelInfo
	Cmd       []rune
	Sel       int
	Query     textEdit
	Status    string

	Settings   config.Settings
	Providers  []llm.Provider
	Models     map[string][]llm.Model // provider name -> fetched model ids
	ModelErr   map[string]string      // provider name -> last fetch error
	Loading    map[string]bool
	Skills     []config.Skill
	Servers    []mcp.Server
	Snippets   []config.Snippet
	SnippetDel string

	// SelText is the text of the last visual or mouse selection; :save_snippet
	// reads it. Registers are shared by all sessions, and so is this.
	SelText string

	// Composer geometry of the last draw, for mouse selection in the composer.
	ComposerRows []vrow
	ComposerX    int
	ComposerY    int
	ComposerTop  int

	Field   int
	Editing bool
	Edit    textEdit // the field being edited while Editing
	// FormNote says why the open form is not saved yet (empty = saved).
	FormNote string

	// Login (:login): the OAuth accounts of the local helper, the running
	// login, and the entry waiting for a second press of d to be removed.
	Modes       []modes.Mode // #plan, #build, #goal and the user's own
	Accounts    []cliproxy.Account
	AccountsErr string
	LoginDel    string
	Flow        loginFlow
	// Locked keeps the login list open until a working provider exists.
	Locked bool

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
