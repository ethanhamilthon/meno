package app

import (
	"time"

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
	OverResume
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

type State struct {
	Mode      ui.Mode
	Over      Overlay
	Input     []rune
	Cursor    int // rune index of the text cursor inside Input
	WantCol   int // sticky column for up/down movement
	WantSet   bool
	InputTop  int         // first visible composer row
	Ask       *askState   // question from the agent waiting for an answer
	Retry     *retryState // set while a failed model request waits to be sent again
	Phase     string      // what the running agent is doing: thinking, writing, running, compacting
	CmdSel    int
	LiveText  string
	LiveThink string
	LiveBase  int        // session length the live state was produced at
	LiveCalls []liveCall // tool calls streaming in or executing
	Unpriced  bool
	Spent     float64
	Context   int
	Pricing   map[string]llm.ModelInfo
	Cmd       []rune
	LogLines  int
	Scroll    int
	Sel       int
	Query     []rune
	Status    string
	Running   bool

	Settings  config.Settings
	Providers []llm.Provider
	Models    map[string][]llm.Model // provider name -> fetched model ids
	ModelErr  map[string]string      // provider name -> last fetch error
	Loading   map[string]bool
	Skills    []config.Skill
	Servers   []mcp.Server
	Sessions  []agent.SessionInfo

	Session *agent.Session
	Prompt  promptState // frozen system prompt and baseline of Session
	Field   int
	Editing bool
	Buffer  []rune

	LastUsage llm.Usage
	Usage     llm.Usage
	Notices   []notice

	// Vim editing state of the input (normal / visual mode).
	VimBuf    []rune // keys of the command being typed (counts, operator...)
	Reg       string // unnamed register
	RegLine   bool   // register holds whole lines
	Undo      []snap
	Redo      []snap
	InsBase   snap   // input as it was when the current insert session began
	LastFind  string // f F t T of the last find, for ; and ,
	LastFindC rune

	// Cursor in the timeline (normal / visual mode). When InLog is false the
	// cursor is Cursor inside Input.
	InLog   bool
	LogLine int
	LogCol  int
	// Visual selection: kind 'v', 'V' or 'b' (block); the anchor mirrors the cursor.
	VKind     rune
	AnchorLog bool
	ALine     int
	ACol      int

	EditIdx  int
	EditProv llm.Provider
	EditMCP  mcp.Server
}

func NewState() *State {
	return &State{
		Mode:     ui.ModeInsert,
		Settings: config.DefaultSettings(),
		Models:   map[string][]llm.Model{},
		ModelErr: map[string]string{},
		Loading:  map[string]bool{},
	}
}
