package llm

import (
	"context"
	"encoding/json"
	"strings"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

const ToolName = "js"

// Image is a picture attached to a user message; its bytes are read from Path
// when the request is built. The text of the message refers to it as [Image #N].
type Image struct {
	N    int
	Mime string
	Path string
}

type Msg struct {
	Role   string
	Text   string
	Images []Image `json:"images,omitempty"`
	CallID string
	Code   string
	// ToolName names the tool of a legacy call (CallID and Code, no ToolCalls):
	// "" is js, BashToolName is the shell tool.
	ToolName  string `json:"tool_name,omitempty"`
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

// Usage is the token accounting of one request, in the provider's units: In is
// all input tokens of the request including those served from the cache
// (Cached) and those written to it (CacheWrite), for both protocols. CacheReported
// says whether the provider reported cache details at all; without it Cached and
// CacheWrite are unknown, not zero.
type Usage struct {
	In            int
	Out           int
	Cached        int
	CacheWrite    int
	CacheReported bool
}

type Request struct {
	Provider Provider
	Model    string
	Effort   string
	System   string
	Msgs     []Msg
	Tools    bool
	// ToolDescription is the description of the js tool: the rules of the runtime
	// and its globals. Empty means DefaultToolDescription.
	ToolDescription string
	// ToolName selects the tool: "" is js (ToolName const), "bash" is the shell tool.
	ToolName string
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
		if m.ToolName == BashToolName {
			return []ToolCall{{ID: m.CallID, Name: BashToolName, Arguments: mustJSON(map[string]string{"command": m.Code})}}
		}
		return []ToolCall{{ID: m.CallID, Name: ToolName, Arguments: mustJSON(map[string]string{"code": m.Code})}}
	}
	return nil
}

// ToolSchema is the definition of the js tool. An empty desc gives the default.
func ToolSchema(desc string, modelTimeout bool) map[string]any {
	if desc == "" {
		desc = DefaultToolDescription
	}
	props := map[string]any{
		"code": map[string]any{"type": "string"},
		"title": map[string]any{
			"type":        "string",
			"description": "What this code does, in at most 6 words (for example \"Read the config files\"). Shown to the user.",
		},
	}
	if modelTimeout {
		props["timeout"] = map[string]any{
			"type":        "number",
			"description": "Seconds this call may run before it is stopped (default 30; 0 means no limit). Raise it for slow commands such as builds or test suites.",
		}
	}
	return map[string]any{
		"type":        "function",
		"name":        ToolName,
		"description": desc,
		"parameters": map[string]any{
			"type":       "object",
			"properties": props,
			"required":   []string{"code", "title"},
		},
	}
}

// DefaultToolDescription is the description of the js tool when the request has none.
const DefaultToolDescription = "Run JavaScript and get the value of the last expression."

// MaxTitleRunes bounds the title of a js call, whatever the model sent.
const MaxTitleRunes = 60

// ToolTitle returns the short title the model gave a js call: whitespace
// collapsed and cut to MaxTitleRunes. It is empty when there is none.
func ToolTitle(args string) string {
	var a struct {
		Title string `json:"title"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return ""
	}
	t := strings.Join(strings.Fields(a.Title), " ")
	if r := []rune(t); len(r) > MaxTitleRunes {
		t = strings.TrimSpace(string(r[:MaxTitleRunes-1])) + "…"
	}
	return t
}

// ToolArgs decodes the arguments of a js tool call. timeout is nil unless the
// model passed one (seconds).
func ToolArgs(args string) (code string, timeout *float64) {
	var a struct {
		Code    string   `json:"code"`
		Command string   `json:"command"`
		Timeout *float64 `json:"timeout"`
	}
	if json.Unmarshal([]byte(args), &a) != nil {
		return args, nil
	}
	if a.Code == "" {
		a.Code = a.Command
	}
	return a.Code, a.Timeout
}
