package agent

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"meno/internal/llm"
)

// Session kinds.
const (
	KindMain  = "main"
	KindChild = "child"
)

// Coverage values: "" means the stored history is complete from the first turn;
// CoveragePartial marks a session compacted by an older version that discarded
// the originals, so history before the summary is unavailable.
const CoveragePartial = "partial"

type Turn struct {
	// HID is the stable history ID: monotonic per session, never reused, and
	// unchanged when compaction moves the turn out of the active context.
	HID int64 `json:"hid,omitempty"`
	// Replay is the complete assistant output; subsequent tool turns share its group.
	Replay      *llm.Msg  `json:"replay,omitempty"`
	ReplayGroup string    `json:"replay_group,omitempty"`
	Role        string    `json:"role"`
	Text        string    `json:"text"`
	Thinking    string    `json:"thinking,omitempty"`
	CallID      string    `json:"call_id,omitempty"`
	Code        string    `json:"code,omitempty"`
	Output      string    `json:"output,omitempty"`
	Err         string    `json:"err,omitempty"`
	Time        time.Time `json:"time"`
	// System is the <system-event> block sent in front of a user message. It is
	// part of the request history but never shown to the user.
	System string `json:"system,omitempty"`
	// Attachments are the images the user message refers to.
	Attachments []Attachment `json:"attachments,omitempty"`
	// Kind is empty for what the user wrote; KindView for an image the agent
	// asked to see with view().
	Kind string `json:"kind,omitempty"`
	// EventID is the steering event a KindEvent turn delivered.
	EventID string `json:"event_id,omitempty"`
	// ElapsedMs is how long a js call ran (at least 1 once measured; 0 for turns
	// that were not timed, such as sessions saved before timing existed). LimitMs
	// is the timeout it ran under (0 = none) and TimedOut says it hit it.
	ElapsedMs int64 `json:"elapsed_ms,omitempty"`
	LimitMs   int64 `json:"limit_ms,omitempty"`
	TimedOut  bool  `json:"timed_out,omitempty"`
	// Title is the short description of a js call that the model wrote.
	Title string `json:"title,omitempty"`
	// Run sums up the work that ended with this turn: set on the last turn of
	// every Continue, whether it finished, failed or was stopped.
	Run *RunStats `json:"run,omitempty"`
}

// Run statuses.
const (
	RunOK   = "ok"
	RunFail = "fail"
	RunStop = "stop"
)

// RunStats is what one run of the agent (a user message to its final answer)
// took. Turns counts the requests answered by the model.
type RunStats struct {
	Status    string    `json:"status"`
	ElapsedMs int64     `json:"elapsed_ms"`
	Turns     int       `json:"turns"`
	ToolCalls int       `json:"tool_calls"`
	ToolMs    int64     `json:"tool_ms"`
	Usage     llm.Usage `json:"usage"`
	Cost      *float64  `json:"cost,omitempty"`
	Partial   bool      `json:"partial,omitempty"`
	Err       string    `json:"err,omitempty"`
}

const KindView = "view"

type RequestUsage struct {
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Usage    llm.Usage `json:"usage"`
	Cost     *float64  `json:"cost,omitempty"`
	Time     time.Time `json:"time"`
}

type Metrics struct {
	LastUsage llm.Usage `json:"last_usage"`
	Usage     llm.Usage `json:"usage"`
	Spent     float64   `json:"spent"`
	Context   int       `json:"context"`
	Unpriced  bool      `json:"unpriced,omitempty"`
}

type Session struct {
	Requests []RequestUsage `json:"requests,omitempty"`
	Metrics  Metrics        `json:"metrics"`
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Cwd      string         `json:"cwd"`
	// Provider, Model and Effort are the model this session talks to; each
	// session has its own. Sessions saved before this was per session may have
	// only Model.
	Provider  string    `json:"provider,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Model     string    `json:"model"`
	Effort    string    `json:"effort,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	Summary   string    `json:"summary,omitempty"`
	// Kind is KindMain or KindChild (empty = main). ParentID owns a child.
	Kind     string `json:"kind,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	// ForkSource/ForkBoundary say which session and history ID this one was
	// forked from (empty/0 when it was not).
	ForkSource   string `json:"fork_source,omitempty"`
	ForkBoundary int64  `json:"fork_boundary,omitempty"`
	// Legacy sessions predate the state journal. Coverage: see CoveragePartial.
	Legacy   bool     `json:"legacy,omitempty"`
	Coverage string   `json:"coverage,omitempty"`
	NextHID  int64    `json:"next_hid,omitempty"`
	Log      *SafeLog `json:"-"`
	// AttachDir is the directory with the bytes of attachments (set by the app).
	AttachDir string `json:"-"`
	Turns     []Turn `json:"turns"`
}

func (s *Session) toolName() string {
	if s.Tool == "" {
		return "js"
	}
	return s.Tool
}

func (s *Session) ensureLog() *SafeLog {
	if s.Log == nil {
		s.Log = newSafeLog(s.Turns, s.NextHID)
	}
	return s.Log
}

// ExecBoundary is the history ID of the turn that will record the step being
// executed now. Journal entries made during the step carry it, so a fork at the
// previous turn does not see them and a fork at the step does.
func (s *Session) ExecBoundary() int64 { return s.ensureLog().NextHID() + 1 }

func (s *Session) Append(t Turn) { s.ensureLog().Append(t) }

// SetRun attaches the stats of a run to the last turn.
func (s *Session) SetRun(r RunStats) { s.ensureLog().SetRunOnLast(r) }

func (s *Session) Len() int { return s.ensureLog().Len() }

func (s *Session) Snapshot() []Turn { return s.ensureLog().Snapshot() }

func (s *Session) Replace(turns []Turn) { s.ensureLog().Replace(turns) }

// SetSystem sets the system-event block of turn i.
func (s *Session) SetSystem(i int, system string) { s.ensureLog().SetSystem(i, system) }

type SessionInfo struct {
	ID        string
	Title     string
	Cwd       string
	UpdatedAt time.Time
	Turns     int
	ToolRuns  int
	Provider  string
	Model     string
	Effort    string
	Tool      string
}

func NewID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func NewSession(cwd, model string) *Session {
	s := &Session{ID: NewID(), Cwd: cwd, Model: model, Tool: "bash", UpdatedAt: time.Now()}
	s.ensureLog()
	return s
}

// SaveSession persists the session into dir/sessions.db in one transaction.
func SaveSession(dir string, s *Session) error {
	if s.ID == "" {
		return errors.New("session id is empty")
	}
	s.UpdatedAt = time.Now()
	s.Turns = s.Snapshot()
	if s.Title == "" || s.Title == "untitled" {
		// A session without a name is named after its first message, as soon as there is one.
		s.Title = deriveTitle(s)
	}
	db, err := openDB(dir)
	if err != nil {
		return err
	}
	return writeSession(db, s)
}

func LoadSession(dir, id string) (*Session, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	db, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	s, err := readSession(db, id)
	if err != nil {
		return nil, err
	}
	s.ensureLog()
	return s, nil
}

// ListSessions returns every stored session, newest first.
func ListSessions(dir string) ([]SessionInfo, error) { return ListRecentSessions(dir, -1) }

// ListRecentSessions returns the n most recently updated sessions (all when n < 0).
func ListRecentSessions(dir string, n int) ([]SessionInfo, error) {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	db, err := openDB(dir)
	if err != nil {
		return nil, err
	}
	return listSessions(db, n)
}

// DisplayTitle is the title shown in lists: the saved one, or one made from the
// first user message.
func (s *Session) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	return deriveTitle(&Session{Turns: s.Snapshot()})
}
