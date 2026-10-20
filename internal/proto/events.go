package proto

import "encoding/json"

// Event is one numbered event on a session or global stream.
type Event struct {
	Seq       uint64          `json:"seq"`
	Type      string          `json:"type"`
	BootID    string          `json:"boot_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Data      json.RawMessage `json:"data"`
}

// TextDeltaData carries text_delta or think_delta content.
type TextDeltaData struct {
	Text string `json:"text"`
}

// CallDeltaData carries a streaming command fragment.
type CallDeltaData struct {
	CallID string `json:"call_id"`
	Delta  string `json:"delta"`
}

// CallStartData announces a tool call.
type CallStartData struct {
	CallID  string `json:"call_id"`
	Command string `json:"command"`
	Title   string `json:"title,omitempty"`
	LimitMS int64  `json:"limit_ms,omitempty"`
}

// CallEndData completes a tool call.
type CallEndData struct {
	CallID    string `json:"call_id"`
	Output    string `json:"output,omitempty"`
	Exit      int    `json:"exit"`
	ElapsedMS int64  `json:"elapsed_ms,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
}

// TurnAddedData announces a persisted turn.
type TurnAddedData struct {
	Turn Turn `json:"turn"`
}

// RequestData describes a provider request without its payload.
type RequestData struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	ContextTokens int    `json:"context_tokens,omitempty"`
}

// StatusData reports session activity.
type StatusData struct {
	Running bool   `json:"running"`
	Phase   string `json:"phase"`
}

// AskClosedData closes an ask form.
type AskClosedData struct {
	AskID string `json:"ask_id"`
	By    string `json:"by,omitempty"`
}

// ByeData describes why a daemon stream is closing.
type ByeData struct {
	Reason string `json:"reason"`
}

// ToolCall is a wire representation of a tool invocation.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}
