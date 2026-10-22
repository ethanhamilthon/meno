package api

import "encoding/json"

// ConfigRequest accesses an opaque daemon-owned configuration resource.
type ConfigRequest struct {
	Resource string          `json:"resource"`
	Data     json.RawMessage `json:"data,omitempty"`
	ClientID string          `json:"client_id,omitempty"`
}

// ConfigResponse returns one opaque daemon-owned configuration resource.
type ConfigResponse struct {
	Resource string          `json:"resource"`
	Data     json.RawMessage `json:"data"`
}

// FileSearchRequest searches workspace paths for the file picker.
type FileSearchRequest struct {
	Root  string `json:"root"`
	Query string `json:"q"`
	Limit int    `json:"limit,omitempty"`
}

// FileSearchItem is one fuzzy path match.
type FileSearchItem struct {
	Path      string `json:"path"`
	Score     int    `json:"score,omitempty"`
	Directory bool   `json:"directory,omitempty"`
}

// FileSearchResponse contains matching workspace paths.
type FileSearchResponse struct {
	Items []FileSearchItem `json:"items"`
}

// DebugRequestsResponse contains recent model requests, which are never streamed.
type DebugRequestsResponse struct {
	Requests []json.RawMessage `json:"requests"`
}

// GlobalStatus describes one session status update for the global stream.
type GlobalStatus struct {
	SessionID  string `json:"session_id"`
	Running    bool   `json:"running"`
	NeedsYou   bool   `json:"needs_you"`
	DoneUnseen bool   `json:"done_unseen"`
}

// SessionLifecycle is the payload of session_created and session_deleted.
type SessionLifecycle struct {
	SessionID string `json:"session_id"`
	ParentID  string `json:"parent_id,omitempty"`
}

// UsageData reports token and cost counts from one model request.
type UsageData struct {
	InputTokens  int64   `json:"input_tokens,omitempty"`
	OutputTokens int64   `json:"output_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
	Cached       int64   `json:"cached,omitempty"`
	CacheWrite   int64   `json:"cache_write,omitempty"`
	CacheKnown   bool    `json:"cache_known,omitempty"`
	Priced       bool    `json:"priced,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	Model        string  `json:"model,omitempty"`
}

// RetryData carries retry delay and reason information.
type RetryData struct {
	Attempt int    `json:"attempt"`
	Max     int    `json:"max,omitempty"`
	InMS    int64  `json:"in_ms"`
	Reason  string `json:"reason,omitempty"`
}

// ErrorData carries an error event without HTTP status semantics.
type ErrorData struct {
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

// DoneData reports final request statistics.
type DoneData struct {
	RequestID string     `json:"request_id,omitempty"`
	Metrics   Metrics    `json:"metrics"`
	Error     *ErrorData `json:"error,omitempty"`
}

// InjectedData reports a message added to an active request.
type InjectedData struct {
	Queued []string `json:"queued"`
	Kind   string   `json:"kind,omitempty"`
	Text   string   `json:"text,omitempty"`
	By     string   `json:"by,omitempty"`
}

// ChangedData lists resource identifiers that changed.
type ChangedData struct {
	IDs      []string         `json:"ids,omitempty"`
	Fields   []string         `json:"fields,omitempty"`
	Metadata *SessionMetadata `json:"metadata,omitempty"`
}
