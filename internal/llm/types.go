package llm

import (
	"context"
	"encoding/json"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

const ToolName = "js"

type Msg struct {
	Role      string
	Text      string
	CallID    string
	Code      string
	Thought   string
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// Responses holds complete output items, including opaque reasoning state.
	Responses []json.RawMessage `json:"responses,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Event struct {
	Kind      string
	Text      string
	CallID    string
	Code      string
	Usage     Usage
	Err       error
	Name      string
	Arguments string
	Item      json.RawMessage
}

const (
	EvText     = "text"
	EvThinking = "thinking"
	EvTool     = "tool"
	// EvToolDelta carries the code of a tool call decoded so far (Code) while
	// its arguments are still streaming. CallID may be empty until the id is known.
	EvToolDelta = "tool_delta"
	EvHistory   = "history"
	EvDone      = "done"
	EvError     = "error"
)

type Usage struct {
	In         int
	Out        int
	Cached     int
	CacheWrite int
}

type Request struct {
	Provider Provider
	Model    string
	Effort   string
	System   string
	Msgs     []Msg
	Tools    bool
	// ModelTimeout adds a timeout argument to the js tool: the model picks how
	// long each call may run.
	ModelTimeout bool
	Cache        *CacheOptions
}

type Client interface {
	Stream(ctx context.Context, req Request, emit func(Event)) error
}

// Calls also recognizes legacy messages, including calls with empty code.
func (m Msg) Calls() []ToolCall {
	if len(m.ToolCalls) != 0 {
		return m.ToolCalls
	}
	if m.Role == RoleAssistant && (m.CallID != "" || m.Code != "") {
		return []ToolCall{{ID: m.CallID, Name: ToolName, Arguments: mustJSON(map[string]string{"code": m.Code})}}
	}
	return nil
}

func ToolSchema(modelTimeout bool) map[string]any {
	props := map[string]any{"code": map[string]any{"type": "string"}}
	if modelTimeout {
		props["timeout"] = map[string]any{
			"type":        "number",
			"description": "Seconds this call may run before it is stopped (default 30; 0 means no limit). Raise it for slow commands such as builds or test suites.",
		}
	}
	return map[string]any{
		"type":        "function",
		"name":        ToolName,
		"description": ToolDescription,
		"parameters": map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"code"},
		},
	}
}

// ToolDescription is shown to the model as the description of the js tool.
const ToolDescription = "Run JavaScript in a persistent VM. The code runs synchronously and the value of the last expression is returned; the available globals are described in the system prompt."

// ToolArgs decodes the arguments of a js tool call. timeout is nil unless the
// model passed one (seconds).
func ToolArgs(args string) (code string, timeout *float64) {
	var a struct {
		Code    string   `json:"code"`
		Timeout *float64 `json:"timeout"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return args, nil
	}
	return a.Code, a.Timeout
}
