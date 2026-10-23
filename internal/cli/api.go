// Package cli parses meno command-line requests and delegates daemon work to an API.
//
// Call Run with process arguments excluding the program name. Local file commands
// do not require an API. Daemon commands use the small API interface in api.go.
package cli

import "time"

// API is the daemon-facing contract used by CLI commands. Implementations keep transport details outside this package.
// Every returned struct is a plain CLI-owned value, independent of the wire client.
type API interface {
	MCPServers() ([]Server, error)
	MCPTools(server string) ([]Tool, error)
	MCPCall(server, tool string, args map[string]any) (MCPResult, error)
	MCPResults(id string, offset, limit int) (MCPPage, error)
	MCPAdmin(action, server string) (Server, error)
	TasksStart(TaskStartRequest) (Task, error)
	TasksList(includeFinished bool) ([]Task, error)
	TaskStatus(id string) (Task, error)
	TaskLogs(id string, from, limit int) (TaskLogs, error)
	TaskWait(id string, timeout time.Duration) (TaskWait, error)
	TaskCancel(id string) (Task, error)
	Ask(form []byte) (map[string]any, error)
	View(path string) (ViewResult, error)
	Models(provider string, refresh bool) (ModelList, error)
	Daemon(action string, force bool) (DaemonStatus, error)
	Sessions(SessionsRequest) ([]Session, error)
	SessionShow(id string) (SessionDetail, error)
	SessionCancel(id string) error
	SessionAnswer(AnswerRequest) error
	Send(SendRequest) (SendResult, error)
	Tail(req TailRequest, emit func(Event) error) error
	Doctor(DoctorRequest) (DoctorReport, error)
}

// APIError identifies a daemon error that maps to a CLI exit code.
type APIError struct{ Code, Message string }

func (e *APIError) Error() string { return e.Message }
