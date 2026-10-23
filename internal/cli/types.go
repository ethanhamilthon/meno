package cli

import (
	"encoding/json"
	"time"

	"meno/internal/api"
)

// These plain structs keep the CLI contract independent of daemon wire packages.
// Server describes one configured MCP server.
type Server struct {
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// Tool describes one MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// MCPResult contains the response from an MCP tool.
type MCPResult = api.MCPResult

// MCPPage contains one page of a stored result.
type MCPPage struct {
	Text       string `json:"text"`
	Offset     int    `json:"offset"`
	End        int    `json:"end"`
	Size       int    `json:"size"`
	NextOffset int    `json:"nextOffset,omitempty"`
}

// TaskStartRequest contains the parameters for a background task.
type TaskStartRequest struct {
	Command string            `json:"command"`
	CWD     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Name    string            `json:"name,omitempty"`
	Notify  bool              `json:"notify"`
}

// Task describes a background task.
type Task struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Command  string `json:"command,omitempty"`
	CWD      string `json:"cwd,omitempty"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Output   string `json:"output,omitempty"`
}

// TaskLog is one log record.
type TaskLog struct {
	Sequence int64  `json:"seq"`
	Stream   string `json:"stream"`
	Text     string `json:"text"`
}

// TaskLogs is a page of task output.
type TaskLogs struct {
	Records    []TaskLog `json:"records"`
	NextCursor int64     `json:"nextCursor,omitempty"`
	Lost       int       `json:"lost,omitempty"`
	Truncated  bool      `json:"truncated,omitempty"`
}

// TaskWait reports the result of waiting for a task.
type TaskWait struct {
	Task     Task `json:"task"`
	TimedOut bool `json:"waitTimedOut,omitempty"`
}

// ViewResult reports an image display request.
type ViewResult struct {
	Path      string `json:"path"`
	Displayed bool   `json:"displayed"`
}

// Model is one model catalog entry.
type Model struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
}

// DaemonStatus reports daemon lifecycle state.
type DaemonStatus struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Version string `json:"version,omitempty"`
}

// Session is a session-list entry. Status is idle, running or needs you.
type Session struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Status    string `json:"status,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
	Effort    string `json:"effort,omitempty"`
	SpawnedBy string `json:"spawnedBy,omitempty"`
	ForkOf    string `json:"forkOf,omitempty"`
	System    string `json:"system,omitempty"`
	NoAsk     bool   `json:"noAsk,omitempty"`
	Turns     int    `json:"turns"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// SessionsRequest filters the session list. SpawnedBy "me" is the calling session.
type SessionsRequest struct {
	CWD, Status, SpawnedBy, TitlePrefix string
	Limit                               int
}

// SessionDetail is `meno sessions show`: a session with its latest answer.
type SessionDetail struct {
	Session
	Answer    string           `json:"answer,omitempty"`
	LastError string           `json:"lastError,omitempty"`
	Question  *PendingQuestion `json:"question,omitempty"`
	Queued    []string         `json:"queued,omitempty"`
	Tasks     int              `json:"tasks"`
	Usage     *Usage           `json:"usage,omitempty"`
}

// Usage is the token and cost total of a session.
type Usage struct {
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	CachedTokens int64   `json:"cachedTokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
}

// PendingQuestion is a question the agent of a session waits for (`meno ask`).
type PendingQuestion struct {
	ID        string         `json:"id"`
	Title     string         `json:"title,omitempty"`
	Questions []QuestionItem `json:"questions"`
}

// QuestionItem is one question of a form.
type QuestionItem struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Type    string   `json:"type"`
	Options []string `json:"options,omitempty"`
}

// AnswerRequest answers (or cancels) the open question of a session.
type AnswerRequest struct {
	SessionID, QuestionID string
	Answers               map[string]json.RawMessage
	Cancel                bool
}

// SendRequest contains a message and session selection: exactly one of
// SessionID, New and Fork.
type SendRequest struct {
	Text      string `json:"text"`
	SessionID string `json:"sessionId,omitempty"`
	New       bool   `json:"new,omitempty"`
	Fork      string `json:"fork,omitempty"` // the source session
	At        int    `json:"at,omitempty"`   // fork after this turn; 0 is the last
	NoAsk     bool   `json:"noAsk,omitempty"`
	Detach    bool   `json:"detach,omitempty"`
	Model     string `json:"model,omitempty"` // PROVIDER/MODEL
	Effort    string `json:"effort,omitempty"`
	CWD       string `json:"cwd,omitempty"`
	Title     string `json:"title,omitempty"`
	System    string `json:"system,omitempty"`
	// Timeout stops waiting (the run goes on). KeepRunning and CancelOnExit say
	// what an interrupted send does to the run: by default it is cancelled only
	// for a session that this send created.
	Timeout      time.Duration `json:"-"`
	KeepRunning  bool          `json:"-"`
	CancelOnExit bool          `json:"-"`
}

// SendResult reports message delivery and final text. Status is done, queued,
// detached, waiting (a question is open), aborted, failed, timeout or interrupted.
type SendResult struct {
	SessionID string           `json:"sessionId"`
	RequestID string           `json:"requestId,omitempty"`
	ForkOf    string           `json:"forkOf,omitempty"`
	Queued    bool             `json:"queued,omitempty"`
	Done      bool             `json:"done,omitempty"`
	Status    string           `json:"status,omitempty"`
	Text      string           `json:"text,omitempty"`
	Error     string           `json:"error,omitempty"`
	Question  *PendingQuestion `json:"question,omitempty"`
	Usage     *Usage           `json:"usage,omitempty"`
	Warnings  []string         `json:"warnings,omitempty"`
}

// TailRequest selects what `meno tail` streams. IDs and ChildrenOf may be
// combined; Events filters event types; From continues after a sequence number
// of the boot Boot (the daemon run) the numbers belong to.
type TailRequest struct {
	IDs        []string
	ChildrenOf string
	Events     []string
	From       uint64
	HasFrom    bool
	Boot       string
}

// Event is a daemon event for the tail command.
type Event struct {
	Session  string         `json:"session,omitempty"`
	Boot     string         `json:"boot,omitempty"`
	Sequence uint64         `json:"seq"`
	Type     string         `json:"type"`
	Data     map[string]any `json:"data,omitempty"`
}

// Check is one finding of `meno doctor`. Status is ok, warn or error; Fix is a
// suggestion only, doctor never repairs anything by itself.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

// DoctorRequest selects optional checks. Probe allows bounded network calls.
type DoctorRequest struct {
	Probe bool `json:"probe,omitempty"`
}

// DoctorReport is the result of all checks; Status is the worst check status.
type DoctorReport struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

// ModelList is the answer of `meno models`: the models and the providers that
// could not be asked, with the reason.
type ModelList struct {
	Models []Model           `json:"models"`
	Errors map[string]string `json:"errors,omitempty"`
}
