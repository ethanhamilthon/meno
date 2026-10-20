package proto

// Event type constants define the session and global SSE vocabulary.
const (
	EventHello          = "hello"
	EventTextDelta      = "text_delta"
	EventThinkDelta     = "think_delta"
	EventCallDelta      = "call_delta"
	EventCallStart      = "call_start"
	EventCallEnd        = "call_end"
	EventTurnAdded      = "turn_added"
	EventRequest        = "request"
	EventUsage          = "usage"
	EventRetry          = "retry"
	EventError          = "error"
	EventAborted        = "aborted"
	EventDone           = "done"
	EventInjected       = "injected"
	EventStatus         = "status"
	EventAskOpen        = "ask_open"
	EventAskClosed      = "ask_closed"
	EventSessionChanged = "session_changed"
	EventTasksChanged   = "tasks_changed"
	EventMCPChanged     = "mcp_changed"
	EventResync         = "resync"
	EventBye            = "bye"
	EventGlobalStatus   = "global_status"
)
