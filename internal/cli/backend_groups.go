package cli

import (
	"net/url"
	"strconv"
	"time"
)

// MCPServers lists the configured MCP servers.
func (a *DaemonAPI) MCPServers() ([]Server, error) {
	var raw []struct {
		Name     string `json:"name"`
		Enabled  bool   `json:"enabled"`
		Running  bool   `json:"running"`
		Starting bool   `json:"starting"`
		Error    string `json:"error"`
	}
	if err := a.get("mcp", "servers", "", &raw); err != nil {
		return nil, err
	}
	out := make([]Server, 0, len(raw))
	for _, s := range raw {
		status := "stopped"
		switch {
		case s.Starting:
			status = "starting"
		case s.Error != "":
			status = "error: " + s.Error
		case s.Running:
			status = "running"
		case !s.Enabled:
			status = "disabled"
		}
		out = append(out, Server{Name: s.Name, Status: status})
	}
	return out, nil
}

// MCPTools lists the tools of one server.
func (a *DaemonAPI) MCPTools(server string) ([]Tool, error) {
	var out []Tool
	return out, a.post("mcp", "tools", map[string]any{"server": server}, &out)
}

// MCPCall calls one MCP tool.
func (a *DaemonAPI) MCPCall(server, tool string, args map[string]any) (MCPResult, error) {
	var res MCPResult
	err := a.post("mcp", "call", map[string]any{"server": server, "tool": tool, "args": args}, &res)
	return res, err
}

// MCPResults reads a stored result.
func (a *DaemonAPI) MCPResults(id string, offset, limit int) (MCPPage, error) {
	var out MCPPage
	return out, a.post("mcp", "results", map[string]any{"id": id, "offset": offset, "limit": limit}, &out)
}

// MCPAdmin changes a server; the daemon asks the user first.
func (a *DaemonAPI) MCPAdmin(action, server string) (Server, error) {
	var out Server
	err := a.post("mcp", action, map[string]any{"server": server}, &out)
	return out, err
}

// TasksStart starts a background task.
func (a *DaemonAPI) TasksStart(r TaskStartRequest) (Task, error) {
	var out Task
	in := map[string]any{"command": r.Command, "cwd": r.CWD, "name": r.Name, "notify": r.Notify}
	if len(r.Env) > 0 {
		in["env"] = r.Env
	}
	return out, a.post("tasks", "start", in, &out)
}

// TasksList lists the tasks of the session.
func (a *DaemonAPI) TasksList(all bool) ([]Task, error) {
	var out []Task
	q := url.Values{"all": {strconv.FormatBool(all)}}.Encode()
	return out, a.get("tasks", "list", q, &out)
}

// TaskStatus returns one task.
func (a *DaemonAPI) TaskStatus(id string) (Task, error) {
	var out Task
	return out, a.post("tasks", "status", map[string]any{"id": id}, &out)
}

// TaskLogs returns a page of task output.
func (a *DaemonAPI) TaskLogs(id string, from, limit int) (TaskLogs, error) {
	var out TaskLogs
	return out, a.post("tasks", "logs", map[string]any{"id": id, "from": from, "limit": limit}, &out)
}

// TaskWait waits for a task; the daemon ends the wait at the timeout.
func (a *DaemonAPI) TaskWait(id string, timeout time.Duration) (TaskWait, error) {
	var out TaskWait
	return out, a.post("tasks", "wait", map[string]any{"id": id, "timeout": int(timeout / time.Second)}, &out)
}

// TaskCancel stops a task.
func (a *DaemonAPI) TaskCancel(id string) (Task, error) {
	var out Task
	return out, a.post("tasks", "cancel", map[string]any{"id": id}, &out)
}
