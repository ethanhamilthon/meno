package daemonapi

import (
	"net/url"
	"strconv"
	"time"

	"meno/internal/cli"
)

// MCPServers lists the configured MCP servers.
func (a *API) MCPServers() ([]cli.Server, error) {
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
	out := make([]cli.Server, 0, len(raw))
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
		out = append(out, cli.Server{Name: s.Name, Status: status})
	}
	return out, nil
}

// MCPTools lists the tools of one server.
func (a *API) MCPTools(server string) ([]cli.Tool, error) {
	var out []cli.Tool
	return out, a.post("mcp", "tools", map[string]any{"server": server}, &out)
}

// MCPCall calls one MCP tool.
func (a *API) MCPCall(server, tool string, args map[string]any) (cli.MCPResult, error) {
	var res cli.MCPResult
	err := a.post("mcp", "call", map[string]any{"server": server, "tool": tool, "args": args}, &res)
	return res, err
}

// MCPResults reads a stored result.
func (a *API) MCPResults(id string, offset, limit int) (cli.MCPPage, error) {
	var out cli.MCPPage
	return out, a.post("mcp", "results", map[string]any{"id": id, "offset": offset, "limit": limit}, &out)
}

// MCPAdmin changes a server; the daemon asks the user first.
func (a *API) MCPAdmin(action, server string) (cli.Server, error) {
	var out cli.Server
	err := a.post("mcp", action, map[string]any{"server": server}, &out)
	return out, err
}

// TasksStart starts a background task.
func (a *API) TasksStart(r cli.TaskStartRequest) (cli.Task, error) {
	var out cli.Task
	in := map[string]any{"command": r.Command, "cwd": r.CWD, "name": r.Name, "notify": r.Notify}
	if len(r.Env) > 0 {
		in["env"] = r.Env
	}
	return out, a.post("tasks", "start", in, &out)
}

// TasksList lists the tasks of the session.
func (a *API) TasksList(all bool) ([]cli.Task, error) {
	var out []cli.Task
	q := url.Values{"all": {strconv.FormatBool(all)}}.Encode()
	return out, a.get("tasks", "list", q, &out)
}

// TaskStatus returns one task.
func (a *API) TaskStatus(id string) (cli.Task, error) {
	var out cli.Task
	return out, a.post("tasks", "status", map[string]any{"id": id}, &out)
}

// TaskLogs returns a page of task output.
func (a *API) TaskLogs(id string, from, limit int) (cli.TaskLogs, error) {
	var out cli.TaskLogs
	return out, a.post("tasks", "logs", map[string]any{"id": id, "from": from, "limit": limit}, &out)
}

// TaskWait waits for a task; the daemon ends the wait at the timeout.
func (a *API) TaskWait(id string, timeout time.Duration) (cli.TaskWait, error) {
	var out cli.TaskWait
	return out, a.post("tasks", "wait", map[string]any{"id": id, "timeout": int(timeout / time.Second)}, &out)
}

// TaskCancel stops a task.
func (a *API) TaskCancel(id string) (cli.Task, error) {
	var out cli.Task
	return out, a.post("tasks", "cancel", map[string]any{"id": id}, &out)
}
