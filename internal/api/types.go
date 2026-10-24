// Package api defines the versioned JSON wire contract between meno clients and the daemon.
package api

// Protocol is the HTTP API compatibility number. Clients require exact equality.
// Protocol 8 removes built-in account login routes and provider source metadata.
// Clients and daemons must upgrade together.
const Protocol = 8

// Error codes returned in an ErrorEnvelope.
const (
	CodeBadRequest       = "bad_request"
	CodeNotFound         = "not_found"
	CodeForbidden        = "forbidden"
	CodeConflict         = "conflict"
	CodeProtocolMismatch = "protocol_mismatch"
	CodeDaemonStopping   = "daemon_stopping"
	CodeNotImplemented   = "not_implemented"
	CodeInternal         = "internal"
)

// ErrorBody is the machine-readable part of an HTTP error.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope wraps an API error.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// Hello is the daemon compatibility and lifecycle response.
type Hello struct {
	Protocol  int    `json:"protocol"`
	Version   string `json:"version"`
	BootID    string `json:"boot_id"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
}

// ShutdownRequest requests an orderly daemon shutdown.
type ShutdownRequest struct {
	Reason string `json:"reason,omitempty"`
}

// Ack is a generic successful operation response.
type Ack struct {
	OK bool `json:"ok"`
}

// SessionItem is one item in a session list response.
type SessionItem struct {
	Completed   int64  `json:"completed,omitempty"`
	ReadThrough int64  `json:"read_through,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Cwd         string `json:"cwd"`
	Model       string `json:"model,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Effort      string `json:"effort,omitempty"`
	Status      string `json:"status,omitempty"`
	Running     bool   `json:"running"`
	NeedsYou    bool   `json:"needs_you"`
	NoAsk       bool   `json:"no_ask,omitempty"`
	SpawnedBy   string `json:"spawned_by,omitempty"` // the session whose agent created this one
	ForkOf      string `json:"fork_of,omitempty"`
	System      string `json:"system,omitempty"` // name of the custom system prompt
	TurnCount   int    `json:"turn_count"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// SessionsListRequest contains session list filters.
type SessionsListRequest struct {
	Cwd   string `json:"cwd,omitempty"`
	Query string `json:"q,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// SessionsListResponse contains matching sessions.
type SessionsListResponse struct {
	Sessions []SessionItem `json:"sessions"`
}

// SessionCreateRequest creates a session.
type SessionCreateRequest struct {
	Cwd   string `json:"cwd"`
	NoAsk bool   `json:"no_ask,omitempty"`
	// Model is PROVIDER/MODEL (a bare model id means a model of the default
	// provider). Effort must be one the model supports.
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Title  string `json:"title,omitempty"`
	// System names a custom system prompt (see settings: system_prompt);
	// "default" is the built-in one.
	System string `json:"system,omitempty"`
	// SpawnedBy is the session whose agent creates this one; the daemon limits
	// the depth and the number of such sessions.
	SpawnedBy string            `json:"spawned_by,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	ClientID  string            `json:"client_id"`
}

// SessionCreateResponse identifies a created session.
type SessionCreateResponse struct {
	Session SessionItem `json:"session"`
}

// LiveState contains content that has not yet become a persisted turn.
type LiveState struct {
	Text     string      `json:"text,omitempty"`
	Thinking string      `json:"thinking,omitempty"`
	Calls    []CallState `json:"calls,omitempty"`
}

// CallState describes one live tool call.
type CallState struct {
	CallID    string `json:"call_id"`
	Command   string `json:"command,omitempty"`
	Title     string `json:"title,omitempty"`
	Delta     string `json:"delta,omitempty"`
	Output    string `json:"output,omitempty"`
	Exit      *int   `json:"exit,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
}

// Metrics contains current run usage counters.
type Metrics struct {
	InputTokens  int64   `json:"input_tokens,omitempty"`
	OutputTokens int64   `json:"output_tokens,omitempty"`
	ToolCalls    int64   `json:"tool_calls,omitempty"`
	ToolMS       int64   `json:"tool_ms,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
}

// Snapshot is a consistent session view at one event sequence.
type Snapshot struct {
	Session    SessionItem    `json:"session"`
	AgentToken string         `json:"agent_token,omitempty"`
	Cwd        string         `json:"cwd"`
	Model      string         `json:"model,omitempty"`
	Running    bool           `json:"running"`
	Phase      string         `json:"phase,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Origin     string         `json:"origin,omitempty"` // the window that started the running turn
	Seq        uint64         `json:"seq"`
	Version    SessionVersion `json:"version"`
	TurnCount  int            `json:"turn_count"`
	Live       LiveState      `json:"live"`
	Ask        *AskForm       `json:"ask,omitempty"`
	Retry      *RetryState    `json:"retry,omitempty"`
	Metrics    Metrics        `json:"metrics"`
	Queued     []string       `json:"queued,omitempty"` // messages waiting for the next step
	TotalUsage *UsageData     `json:"total_usage,omitempty"`
	LastUsage  *UsageData     `json:"last_usage,omitempty"`
	Spent      float64        `json:"spent,omitempty"`
	Context    int            `json:"context,omitempty"`
	Unpriced   bool           `json:"unpriced,omitempty"`
	Provider   string         `json:"provider,omitempty"`
	Effort     string         `json:"effort,omitempty"`
	LastError  string         `json:"last_error,omitempty"` // outcome of the last finished run; empty after a success
	TaskCount  int            `json:"task_count"`
	Turns      []Turn         `json:"turns,omitempty"`
	Tasks      []Task         `json:"tasks,omitempty"`
}

// PatchSessionRequest updates session metadata.
type PatchSessionRequest struct {
	Name     *string `json:"name,omitempty"`
	Cwd      *string `json:"cwd,omitempty"`
	Provider *string `json:"provider,omitempty"`
	Model    *string `json:"model,omitempty"`
	Effort   *string `json:"effort,omitempty"`
	// Select changes the model of an idle session as one checked step: the
	// provider and model must exist and the effort must suit the model.
	Select *ModelSelection `json:"select,omitempty"`
}

// ModelSelection is PROVIDER/MODEL and/or an effort; empty parts stay as they are.
type ModelSelection struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

// ForkRequest creates a session from an existing session.
type ForkRequest struct {
	Turn     int    `json:"turn,omitempty"`
	ClientID string `json:"client_id"`
	// The fork keeps the model, effort, title, directory and system prompt of
	// its source unless these say otherwise. NoAsk can only be turned on.
	Model     string `json:"model,omitempty"`
	Effort    string `json:"effort,omitempty"`
	Title     string `json:"title,omitempty"`
	Cwd       string `json:"cwd,omitempty"`
	System    string `json:"system,omitempty"`
	NoAsk     bool   `json:"no_ask,omitempty"`
	SpawnedBy string `json:"spawned_by,omitempty"`
}

// ForkResponse describes the forked session.
type ForkResponse struct {
	Session  SessionItem `json:"session"`
	Warnings []string    `json:"warnings,omitempty"`
}

// Turn is a persisted conversation item.
type Turn struct {
	ID        string     `json:"id,omitempty"`
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	Parts     []TurnPart `json:"parts,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
	// Fields a client needs to draw the turn.
	Kind        string           `json:"kind,omitempty"` // "" , "event" or "view"
	Thinking    string           `json:"thinking,omitempty"`
	Err         string           `json:"err,omitempty"`
	ElapsedMS   int64            `json:"elapsed_ms,omitempty"`
	LimitMS     int64            `json:"limit_ms,omitempty"`
	TimedOut    bool             `json:"timed_out,omitempty"`
	Attachments []TurnAttachment `json:"attachments,omitempty"`
	Run         *RunStats        `json:"run,omitempty"`
}

// TurnAttachment is an image a message refers to; the bytes live in the
// attachment store of the daemon, named by SHA.
type TurnAttachment struct {
	N    int    `json:"n"`
	Mime string `json:"mime"`
	SHA  string `json:"sha"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

// RunStats closes a run: what it took and cost.
type RunStats struct {
	Status     string   `json:"status"`
	ElapsedMS  int64    `json:"elapsed_ms"`
	Turns      int      `json:"turns"`
	ToolCalls  int      `json:"tool_calls"`
	ToolMS     int64    `json:"tool_ms"`
	In         int      `json:"in"`
	Out        int      `json:"out"`
	CacheWrite int      `json:"cache_write"`
	CacheKnown bool     `json:"cache_known"`
	Cached     int      `json:"cached"`
	Cost       *float64 `json:"cost,omitempty"`
	Partial    bool     `json:"partial,omitempty"`
	Err        string   `json:"err,omitempty"`
}

// TurnPart is declared in history.go.
