package llm

import "context"

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

const ToolName = "js"

type Msg struct {
	Role    string
	Text    string
	CallID  string
	Code    string
	Thought string
}

type Event struct {
	Kind   string
	Text   string
	CallID string
	Code   string
	Usage  Usage
	Err    error
}

const (
	EvText     = "text"
	EvThinking = "thinking"
	EvTool     = "tool"
	EvDone     = "done"
	EvError    = "error"
)

type Usage struct {
	In  int
	Out int
}

type Request struct {
	Provider Provider
	Model    string
	Effort   string
	System   string
	Msgs     []Msg
	Tools    bool
}

type Client interface {
	Stream(ctx context.Context, req Request, emit func(Event)) error
}

func ToolSchema() map[string]any {
	return map[string]any{
		"type":        "function",
		"name":        ToolName,
		"description": "Execute JavaScript in a persistent VM. Top-level declarations persist across calls (use globalThis.x for code using await, since such code runs inside an async function). Helpers: fs.read/write/edit/glob/exists, sh(cmd), print(...). fs and sh(cmd) are async-capable via await.",
		"parameters": map[string]any{
			"type":       "object",
			"properties": map[string]any{"code": map[string]any{"type": "string"}},
			"required":   []string{"code"},
		},
	}
}
