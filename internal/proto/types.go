// Package proto defines the versioned JSON wire contract between meno clients and the daemon.
package proto

// Protocol is the HTTP API compatibility number. Clients require exact equality.
const Protocol = 1

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
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Cwd       string `json:"cwd"`
	Model     string `json:"model,omitempty"`
	Status    string `json:"status,omitempty"`
	Running   bool   `json:"running"`
	NeedsYou  bool   `json:"needs_you"`
	UpdatedAt string `json:"updated_at,omitempty"`
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
	Cwd      string            `json:"cwd"`
	Model    string            `json:"model,omitempty"`
	Modes    []string          `json:"modes,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	ClientID string            `json:"client_id"`
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
	LimitMS   int64  `json:"limit_ms,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
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
	Session    SessionItem `json:"session"`
	Cwd        string      `json:"cwd"`
	Model      string      `json:"model,omitempty"`
	Modes      []string    `json:"modes,omitempty"`
	Running    bool        `json:"running"`
	Phase      string      `json:"phase,omitempty"`
	RequestID  string      `json:"request_id,omitempty"`
	Seq        uint64      `json:"seq"`
	TurnCount  int         `json:"turn_count"`
	Live       LiveState   `json:"live"`
	Ask        *AskForm    `json:"ask,omitempty"`
	Retry      *RetryState `json:"retry,omitempty"`
	Metrics    Metrics     `json:"metrics"`
	TaskCount  int         `json:"task_count"`
	AgentCount int         `json:"agent_count"`
	Turns      []Turn      `json:"turns,omitempty"`
	Tasks      []Task      `json:"tasks,omitempty"`
	Agents     []Agent     `json:"agents,omitempty"`
}

// PatchSessionRequest updates session metadata.
type PatchSessionRequest struct {
	Name   *string   `json:"name,omitempty"`
	Cwd    *string   `json:"cwd,omitempty"`
	Model  *string   `json:"model,omitempty"`
	Effort *string   `json:"effort,omitempty"`
	Modes  *[]string `json:"modes,omitempty"`
}

// ForkRequest creates a session from an existing session.
type ForkRequest struct {
	Turn     int    `json:"turn,omitempty"`
	ClientID string `json:"client_id"`
}

// ForkResponse describes the forked session.
type ForkResponse struct {
	Session SessionItem `json:"session"`
}

// Turn is a persisted conversation item.
type Turn struct {
	ID        string     `json:"id,omitempty"`
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	Parts     []TurnPart `json:"parts,omitempty"`
	CreatedAt string     `json:"created_at,omitempty"`
}

// TurnPart is declared in history.go.
