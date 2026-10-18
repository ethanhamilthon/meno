package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"meno/internal/agent"
	"meno/internal/jsvm"
	"meno/internal/llm"
	"meno/internal/mcp"
	"meno/internal/ui"
)

// Runtime is everything that belongs to one session while the program runs:
// the session itself, its own js VM, the running turn and what the UI shows
// for it (stream, question, counters, composer draft). Providers, the MCP
// manager and settings are shared and live in App and State.
//
// A Runtime is only touched on the UI goroutine, except cancel (guarded by
// App.mu) and VM, which has its own locks. Work in other goroutines gets what
// it needs as arguments and reports back through events carrying the session id.
type Runtime struct {
	Session *agent.Session
	Prompt  promptState // frozen system prompt and baseline of Session
	Cwd     string      // working directory of this session
	PrevCwd string      // directory before the last :cd, for ":cd -"
	VM      *jsvm.VM
	cancel  context.CancelFunc

	// RequestID is the root request being worked on: a user message starts one,
	// steering and continuations belong to it. Cancelled holds the stopped ones.
	RequestID         string
	lines             lineCache
	Pick              *pickState          // the @ picker, nil when closed
	Fps               []llm.Fingerprint   // fingerprints of the last two model requests
	Kids              []agent.ChildRecord // subagents of this session, as last listed
	Delegated         float64             // their spend, kept apart from this session's own
	DelegatedUnpriced bool
	Todo              *agent.Plan // the active plan as last changed; the UI draws this copy
	Cancelled         map[string]bool

	Running   bool
	Queue     agent.Queue // messages written while the turn runs; the runner takes them between steps
	Phase     string      // what the running agent is doing: thinking, writing, running, compacting
	Ask       *askState   // question from the agent waiting for an answer
	Retry     *retryState // set while a failed model request waits to be sent again
	LiveText  string
	LiveThink string
	LiveBase  int        // session length the live state was produced at
	LiveCalls []liveCall // tool calls streaming in or executing
	// The run in progress, from the user's message to the final answer.
	RunStart  time.Time
	RunTurns  int       // requests the model has answered
	RunJS     int       // js calls finished
	RunUsage0 llm.Usage // session usage when the run began
	// PlanAt is the timeline position (turns before it) where the plan block
	// sits: right after the message that started the latest run. It stays put
	// while the run streams, so nothing below it jumps. 0 = unknown.
	PlanAt   int
	LogLines int
	Scroll   int

	LastUsage llm.Usage
	Usage     llm.Usage
	Spent     float64
	Context   int
	Unpriced  bool

	// The composer draft and the vim state around it.
	Images   []draftImage // images attached to the draft; the input holds their [Image #N] tokens
	Mode     ui.Mode
	Input    []rune
	Cursor   int // rune index of the text cursor inside Input
	WantCol  int // sticky column for up/down movement
	WantSet  bool
	InputTop int // first visible composer row
	VimBuf   []rune
	Undo     []snap
	Redo     []snap
	InsBase  snap // input as it was when the current insert session began
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

	Unseen  bool // a turn finished while another session was on screen
	LastErr bool // the last turn ended with an error
	Saved   bool // the session is in the database
}

// ID is the session id.
func (rt *Runtime) ID() string { return rt.Session.ID }

// Empty reports whether nothing was said in the session yet.
func (rt *Runtime) Empty() bool { return rt.Session.Len() == 0 && rt.Session.Summary == "" }

// Idle reports whether nothing is going on in the session.
func (rt *Runtime) Idle() bool { return !rt.Running && rt.Ask == nil }

// newRuntime wraps a session in a fresh runtime with its own js VM rooted at cwd.
func (a *App) newRuntime(sess *agent.Session, cwd string) *Runtime {
	sess.AttachDir = a.attachDir()
	rt := &Runtime{Session: sess, Cwd: cwd, Mode: ui.ModeInsert}
	rt.VM = jsvm.New(jsvm.Options{Root: cwd, Timeout: a.st.Settings.TimeoutDuration()})
	rt.VM.SetMCP(mcpHost{a.mcp, mcp.ResultStore{Dir: filepath.Join(a.paths.Home, "mcp-results")}, sess.ID})
	rt.VM.SetMCPAdmin(mcpAdmin{a, sess.ID})
	rt.VM.SetAsker(appAsker{a, sess.ID})
	rt.VM.SetViewer(appViewer{a, sess.ID, &rt.Queue})
	rt.VM.SetSessionHost(sessionHost{a: a, sid: sess.ID})
	if a.tasks != nil && sess.Kind != agent.KindChild {
		rt.VM.SetTasks(tasksHost{a, sess.ID})
	}
	if a.kids != nil && sess.Kind != agent.KindChild {
		rt.VM.SetAgents(parentAgents{a, a.kids, sess.ID}, false)
	}
	if td, err := agent.NewTodo(a.paths.Home, sess.ID, sess.ExecBoundary); err == nil {
		rt.VM.SetTodo(todoHost{a, sess.ID, td})
		rt.Todo, _ = td.Active()
	}
	if kv, err := agent.OpenKV(a.paths.Home, sess.ID, agent.NSState); err == nil {
		rt.VM.SetState(jsState{kv, sess})
	}
	a.rts[sess.ID] = rt
	a.rtList = append(a.rtList, rt)
	return rt
}

// dropRuntime forgets a runtime that has nothing worth keeping.
func (a *App) dropRuntime(rt *Runtime) {
	delete(a.rts, rt.ID())
	for i, r := range a.rtList {
		if r == rt {
			a.rtList = append(a.rtList[:i], a.rtList[i+1:]...)
			break
		}
	}
}

// inRT runs fn with the session sid as the current one, so that the handlers
// written for "the current session" serve events of a background session too.
// The screen keeps showing what it showed; only a.st.Runtime is swapped for
// the duration of fn.
func (a *App) inRT(sid string, fn func()) {
	rt := a.rts[sid]
	if rt == nil {
		return
	}
	prev := a.st.Runtime
	a.st.Runtime = rt
	defer func() { a.st.Runtime = prev }()
	fn()
}

// switchTo puts a session on screen.
func (a *App) switchTo(rt *Runtime) {
	prev := a.st.Runtime
	if prev == rt {
		return
	}
	a.st.Runtime = rt
	rt.Unseen = false
	// An empty session nobody used is not worth keeping around.
	if prev != nil && prev.Empty() && prev.Idle() && !prev.Saved && len(prev.Input) == 0 {
		a.dropRuntime(prev)
	}
	a.reloadSkills() // the project skills depend on the directory
}

// newSession starts an empty session in the directory of the current one.
func (a *App) newSession() {
	a.saveSession()
	cwd := a.st.Cwd
	rt := a.newRuntime(a.newAgentSession(cwd), cwd)
	a.switchTo(rt)
}

// selection is the model of the current session: provider name, model id and
// reasoning effort. A session that never had one (an old stored session, or one
// started before a model was picked) uses the default from the settings, which
// is the last model picked with :model.
func (a *App) selection() (provider, model, effort string) {
	if s := a.st.Session; s != nil && s.Provider != "" && s.Model != "" {
		return s.Provider, s.Model, s.Effort
	}
	st := a.st.Settings
	return st.Provider, st.Model, st.Effort
}

// setSelection gives the current session a model. It also becomes the default
// for sessions started later. A turn that is running keeps the model it
// started with.
func (a *App) setSelection(provider, model, effort string) {
	if s := a.st.Session; s != nil {
		s.Provider, s.Model, s.Effort = provider, model, effort
	}
	a.st.Settings.Provider, a.st.Settings.Model, a.st.Settings.Effort = provider, model, effort
}

// newAgentSession is an empty session in cwd with the default model.
func (a *App) newAgentSession(cwd string) *agent.Session {
	s := agent.NewSession(cwd, a.st.Settings.Model)
	s.Provider, s.Effort = a.st.Settings.Provider, a.st.Settings.Effort
	return s
}

// busyAction describes what keeps a session from being left behind, or "".
func (rt *Runtime) busyAction() string {
	switch {
	case rt.Ask != nil:
		return "waiting for an answer"
	case rt.Running:
		return "running"
	}
	return ""
}

// sessionTitle is the title of a session for lists and messages.
func (rt *Runtime) sessionTitle() string {
	t := rt.Session.DisplayTitle()
	if rt.Empty() {
		return "new session"
	}
	return t
}

// busySessions lists the sessions with a running turn or a pending question.
func (a *App) busySessions() []string {
	var out []string
	for _, rt := range a.rtList {
		if what := rt.busyAction(); what != "" {
			out = append(out, "“"+rt.sessionTitle()+"” ("+what+")")
		}
	}
	return out
}

// dirExists reports whether dir is an existing directory.
func dirExists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

// expandHome turns a leading ~ into the home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return h + p[1:]
		}
	}
	return p
}
