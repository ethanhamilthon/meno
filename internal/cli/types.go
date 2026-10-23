package cli

import "meno/internal/api"

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

// Session is a concise session-list entry.
type Session struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	CWD    string `json:"cwd,omitempty"`
	Status string `json:"status,omitempty"`
}

// SendRequest contains a message and session selection.
type SendRequest struct {
	Text      string `json:"text"`
	SessionID string `json:"sessionId,omitempty"`
	New       bool   `json:"new,omitempty"`
	NoAsk     bool   `json:"noAsk,omitempty"`
	Detach    bool   `json:"detach,omitempty"`
}

// SendResult reports message delivery and final text.
type SendResult struct {
	SessionID string `json:"sessionId"`
	RequestID string `json:"requestId,omitempty"`
	Queued    bool   `json:"queued,omitempty"`
	Done      bool   `json:"done,omitempty"`
	Text      string `json:"text,omitempty"`
}

// Event is a daemon event for the tail command.
type Event struct {
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

// WebPage is a fetched page as text. URL is what was asked for, FinalURL where
// the redirects ended.
type WebPage struct {
	URL       string `json:"url"`
	FinalURL  string `json:"final_url"`
	Title     string `json:"title,omitempty"`
	Type      string `json:"content_type,omitempty"`
	Status    int    `json:"status"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
	Notice    string `json:"notice,omitempty"`
}

// WebSource is a page a search answer cites.
type WebSource struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

// WebSearchResult is a search answer with its sources.
type WebSearchResult struct {
	Query    string      `json:"query"`
	Answer   string      `json:"answer"`
	Sources  []WebSource `json:"sources,omitempty"`
	Provider string      `json:"provider,omitempty"`
	Model    string      `json:"model,omitempty"`
	Notice   string      `json:"notice,omitempty"`
}
