package tui

import (
	"time"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/render"
)

// mcpInfo is what the daemon says about a running MCP server.
type mcpInfo struct {
	Running, Starting bool
	Err               string
	Tools             int
}

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
	OverContext
	OverHelp
	OverFork
	OverTasks
)

// liveCall is a tool call that is not in the session yet: its code is still
// streaming from the model, or it is executing.
type liveCall struct {
	ID      string
	Code    string
	Title   string
	Running bool
	// Started is when the execution began.
	Started time.Time
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

	// EffortPick is the effort chosen in the model picker but not applied yet
	// ("" until the user presses ←/→). Enter applies it; Esc drops it.
	EffortPick string
	// LocalScope makes edits of look and notification settings apply to this
	// window only (toggled with "s" in /settings).
	LocalScope bool
	// AttnFilter narrows /sessions to running, asking or done sessions.
	AttnFilter    int
	OverlayOrigin render.Mode
	Over          Overlay
	// UpdateTag is the newer release the background check found ("" if none).
	UpdateTag  string
	TasksUI    tasksView
	CmdSel     int
	CmdPicked  bool // an arrow chose the row, so Enter may run it on an empty line
	Pricing    map[string]llm.ModelInfo
	AcDismiss  string // token dismissed until its text or cursor changes
	CmdDismiss int    // start offset+1 of a command token the user closed with Esc
	Sel        int
	Query      textEdit
	Status     string

	Settings   config.Settings
	Providers  []llm.Provider
	Models     map[string][]llm.Model // provider name -> fetched model ids
	ModelErr   map[string]string      // provider name -> last fetch error
	Loading    map[string]bool
	Skills     []config.Skill
	Servers    []mcp.Server
	ServerInfo map[string]mcpInfo // runtime state of the servers, by name

	// SelText preserves the last visual or mouse selection.
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

	// Login (/login): the OAuth accounts of the local helper, the running
	// login, and the entry waiting for a second press of d to be removed.
	Accounts    []oauthAccount
	AccountsErr string
	LoginDel    string
	Flow        loginFlow
	// Locked keeps the login list open until a working provider exists.
	Locked bool

	Notices []notice

	// Copied is the last text copied with the mouse.
	Copied string

	EditIdx  int
	EditProv llm.Provider
	EditMCP  mcp.Server
}

func NewState() *State {
	return &State{
		Runtime:  &Runtime{Mode: render.ModeInput},
		Settings: config.DefaultSettings(),
		Models:   map[string][]llm.Model{},
		ModelErr: map[string]string{},
		Loading:  map[string]bool{},
	}
}
