package app

import (
	"os"
	"strings"
	"time"

	"meno/internal/llm"
	"meno/internal/ui"
)

// Runtime is everything that belongs to one session while the program runs: a
// view of the session the daemon owns, the run in progress as the daemon
// reports it, and what the UI keeps for it (stream, question, scroll, composer
// draft). The daemon does the work; this only shows it and sends commands.
//
// A Runtime is only touched on the UI goroutine. Work in other goroutines gets
// what it needs as arguments and reports back through events carrying the
// session id.
type Runtime struct {
	Session   *sessView
	Cwd       string // working directory of this session
	PrevCwd   string // directory before the last /cd, for "/cd -"
	RunEffort string // effort of the request in flight, when the daemon routes it
	RequestID string // the request being worked on
	lines     lineCache
	Pick      *pickState // the @ picker, nil when closed
	// Tasks are the background work of the session, as the daemon last said.
	Tasks []taskRow

	Running   bool
	sending   int         // messages sent to the daemon and not yet acknowledged
	Queued    []string    // messages written while the turn runs; the daemon sends them between steps
	Title     string      // title of the latest tool call of this run; "" until the first one
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
	RunJS     int       // tool calls finished
	RunUsage0 llm.Usage // session usage when the run began
	LogLines  int
	Scroll    int

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
	Saved   bool // the session is in the database (the daemon has it)

	watch func() // stops the event stream of this session
}

// taskRow is a background task of the session in the /tasks list.
type taskRow struct {
	ID, Name, Command, Status, Started string
	Exit                               *int
}

// ID is the session id.
func (rt *Runtime) ID() string { return rt.Session.ID() }

// Empty reports whether nothing was said in the session yet.
func (rt *Runtime) Empty() bool { return rt.Session.Len() == 0 && !rt.Session.hasSummary }

// Idle reports whether nothing is going on in the session.
func (rt *Runtime) Idle() bool { return !rt.Running && rt.Ask == nil }

// newRuntime wraps a session view in a fresh runtime.
func (a *App) newRuntime(sess *sessView, cwd string) *Runtime {
	rt := &Runtime{Session: sess, Cwd: cwd, Mode: ui.ModeInput}
	a.rts[sess.ID()] = rt
	a.rtList = append(a.rtList, rt)
	return rt
}

// dropRuntime forgets a runtime that has nothing worth keeping.
func (a *App) dropRuntime(rt *Runtime) {
	if rt.watch != nil {
		rt.watch()
	}
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
	a.st.Mode = ui.ModeInput
	prev := a.st.Runtime
	if prev == rt {
		return
	}
	a.st.Runtime = rt
	// An empty session nobody used is not worth keeping around.
	if prev != nil && prev.Empty() && prev.Idle() && !prev.Saved && len(prev.Input) == 0 {
		a.dropRuntime(prev)
	}
	a.reloadSkills() // the project skills depend on the directory
}

// selection is the model of the current session: provider name, model id and
// reasoning effort. A session that never had one uses the default from the
// settings, which is the last model picked with /model.
func (a *App) selection() (provider, model, effort string) {
	// Only a wholly unset session previews the defaults, as the daemon will
	// resolve it. A partial pair is shown as it is: the user must pick a model.
	if s := a.st.Session; s != nil && (s.Provider != "" || s.Model != "") {
		return s.Provider, s.Model, s.Effort
	}
	st := a.st.Settings
	return st.Provider, st.Model, st.Effort
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
