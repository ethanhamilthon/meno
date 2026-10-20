package agent

import (
	"context"
	"time"

	"meno/internal/llm"
)

// Ev is an event of a running turn, for the UI and the accounting.
type Ev struct {
	Kind     string
	Text     string
	Thinking string
	Code     string
	Title    string // the model's short title of the call an EvJSStart announces
	Output   string
	Err      string
	CallID   string
	// Attempt, Max and Delay describe an EvRetry: attempt number Attempt of Max
	// starts after Delay; Err is why the previous one failed.
	Attempt, Max int
	Delay        time.Duration
	// Limit is the timeout of the js call an EvJSStart announces (0 = none);
	// Elapsed and TimedOut report how an EvJSEnd call went.
	Limit    time.Duration
	Elapsed  time.Duration
	TimedOut bool
	Usage    llm.Usage
	// Req is the request an EvRequest announces. Fingerprinting it costs a pass
	// over the whole history, so the consumer does that only when it needs to.
	Req      *llm.Request
	Provider llm.Provider
	Model    string
	// Turns is the session length when the event was emitted, so the UI can tell
	// live (streaming) content from content that is already persisted.
	Turns int
}

const (
	EvText    = "text"
	EvThink   = "think"
	EvCode    = "code" // tool call code streaming in (Code is the full text so far)
	EvJSStart = "js_start"
	EvJSEnd   = "js_end"
	EvDone    = "done"
	EvUsage   = "usage"
	EvError   = "error"
	EvAborted = "aborted"
	// EvInjected: queued user messages or steering events were added to the session.
	EvInjected = "injected"
	// EvRequest: a model request is about to be sent; Req is that request.
	EvRequest = "request"
	// EvRetry: the request failed and will be sent again; whatever it streamed
	// so far is void.
	EvRetry = "retry"
)

// Deps is everything a Runner needs.
type ToolRunner interface {
	Run(context.Context, string, time.Duration) (string, string, string, bool, time.Duration)
	Timeout() time.Duration
}

type Deps struct {
	Client   llm.Client
	Provider llm.Provider
	Model    string
	Effort   string
	System   string
	Tool     ToolRunner
	Thinking bool
	// ModelTimeout lets the model pass a timeout with every js call.
	ModelTimeout bool
	// ToolDesc is the description of the js tool (prompt.ToolDescription).
	ToolDesc string
	// ToolName is the name of the tool: "" or "js" for the js tool, "bash" for the shell tool.
	ToolName string
	// DefaultTimeout is the limit of a call whose model named none, in model-timeout
	// mode (0 means 30 s).
	DefaultTimeout time.Duration
	// Goal, when set, keeps the run going until the agent gives a verdict.
	Goal *Goal
	// Retry is the policy for failed model requests (zero value: defaults).
	Retry RetryPolicy
	// Queue, when set, delivers messages the user wrote during the turn; they
	// join the conversation between steps (never between a call and its result).
	Queue *Queue
	// Events delivers steering events (task messages and completions).
	Events EventSource
	// AutoCompact is the safety net against overflowing the context window.
	AutoCompact AutoCompact
	// DebugPayload, when set, receives every request before it is sent. Opt-in
	// debugging only: it sees full message text (never credentials).
	DebugPayload func(llm.Request)
	// Cost prices the usage of one request; ok is false when the model has no
	// price. Optional: without it a run's summary carries no cost.
	Cost func(llm.Usage) (cost float64, ok bool)
	Emit func(Ev)
}
