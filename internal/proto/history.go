package proto

import "encoding/json"

// TurnPart is a typed segment of a turn.
type TurnPart struct {
	Type string          `json:"type"`
	Text string          `json:"text,omitempty"`
	Call *CallState      `json:"call,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// TurnsPageRequest selects a history page.
type TurnsPageRequest struct {
	From  int `json:"from,omitempty"`
	Limit int `json:"limit,omitempty"`
}

// TurnsPage is a page of conversation history.
type TurnsPage struct {
	Turns []Turn `json:"turns"`
	From  int    `json:"from"`
	Next  int    `json:"next"`
	Total int    `json:"total"`
}

// Attachment is an opaque user-provided message attachment.
type Attachment struct {
	Name string `json:"name"`
	MIME string `json:"mime,omitempty"`
	Data string `json:"data,omitempty"`
	Path string `json:"path,omitempty"`
}

// MessageRequest submits or queues a user message.
type MessageRequest struct {
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
	ClientID    string       `json:"client_id"`
}

// MessageResponse identifies a started or queued request.
type MessageResponse struct {
	RequestID string `json:"request_id"`
	Queued    bool   `json:"queued,omitempty"`
}

// StopRequest stops the active run.
type StopRequest struct {
	RequestID string `json:"request_id,omitempty"`
}

// CompactRequest asks the daemon to compact session history.
type CompactRequest struct {
	ClientID string `json:"client_id"`
	Keep     int    `json:"keep,omitempty"`
}

// CompactResponse reports compaction results.
type CompactResponse struct {
	Removed int    `json:"removed"`
	Summary string `json:"summary,omitempty"`
}

// AskAnswerRequest submits answers or cancels an open ask.
type AskAnswerRequest struct {
	Answers map[string]json.RawMessage `json:"answers,omitempty"`
	Cancel  bool                       `json:"cancel,omitempty"`
}

// AskForm is a user response form emitted during a run.
type AskForm struct {
	AskID     string        `json:"ask_id"`
	By        string        `json:"by,omitempty"`
	Title     string        `json:"title,omitempty"`
	Questions []AskQuestion `json:"questions"`
	Deadline  string        `json:"deadline,omitempty"`
}

// AskChoice is one option of a question.
type AskChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// AskQuestion describes one form input.
type AskQuestion struct {
	ID       string   `json:"id"`
	Prompt   string   `json:"prompt"`
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Required bool     `json:"required,omitempty"`
	// Choices are the options with their labels; Options keeps just the ids.
	Choices        []AskChoice `json:"choices,omitempty"`
	Recommendation string      `json:"recommendation,omitempty"`
}

// RetryState describes a pending retry.
type RetryState struct {
	Attempt int    `json:"attempt"`
	InMS    int64  `json:"in_ms"`
	Reason  string `json:"reason,omitempty"`
}
