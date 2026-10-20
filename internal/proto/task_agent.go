package proto

import "encoding/json"

// Task is a daemon task list item.
type Task struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Status    string `json:"status"`
	Command   string `json:"command,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	Exit      *int   `json:"exit,omitempty"`
}

// Agent is a subagent list item.
type Agent struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status"`
	Task   string `json:"task,omitempty"`
	Result string `json:"result,omitempty"`
}

// TaskStartRequest starts one background process.
type TaskStartRequest struct {
	Command        string            `json:"command"`
	Cwd            string            `json:"cwd,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Name           string            `json:"name,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Notify         *bool             `json:"notify,omitempty"`
}

// TaskListRequest controls inclusion of finished tasks.
type TaskListRequest struct {
	IncludeFinished bool `json:"all,omitempty"`
}

// TaskWaitResponse combines the final task and timeout result.
type TaskWaitResponse struct {
	Task         Task `json:"task"`
	WaitTimedOut bool `json:"waitTimedOut,omitempty"`
}

type TaskStatusRequest struct {
	ID string `json:"id"`
}
type TaskLogsRequest struct {
	ID    string `json:"id"`
	From  int64  `json:"from,omitempty"`
	Limit int    `json:"limit,omitempty"`
}
type TaskWaitRequest struct {
	ID             string `json:"id"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type TaskCancelRequest struct {
	ID string `json:"id"`
}
type AgentSpawnRequest struct {
	Task     string `json:"task"`
	Cwd      string `json:"cwd,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Effort   string `json:"effort,omitempty"`
}

// AgentListResponse contains all parent-owned children.
type AgentListResponse struct {
	Agents []Agent `json:"agents"`
}

// AgentWaitResponse combines the child result and timeout result.
type AgentWaitResponse struct {
	Agent        Agent `json:"agent"`
	WaitTimedOut bool  `json:"waitTimedOut,omitempty"`
}

type AgentIDRequest struct {
	ID string `json:"id"`
}
type AgentMessageRequest struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type AgentWaitRequest struct {
	ID             string `json:"id"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type AgentReportRequest struct {
	Text string `json:"text"`
}
type AgentBlockRequest struct {
	Reason string `json:"reason"`
}

// MCPServer describes configured and active server state.
type MCPServer struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// MCPServersResponse lists configured servers.
type MCPServersResponse struct {
	Servers []MCPServer `json:"servers"`
}

// MCPToolsRequest selects a server.
type MCPToolsRequest struct {
	Server string `json:"server"`
}

// MCPTool describes one callable server tool.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPToolsResponse lists tools.
type MCPToolsResponse struct {
	Tools []MCPTool `json:"tools"`
}

// MCPCallResponse contains the stored result reference.
type MCPCallResponse struct {
	Text     string `json:"text,omitempty"`
	ResultID string `json:"resultId,omitempty"`
	IsError  bool   `json:"isError,omitempty"`
}

// WebSearchResponse contains a query and search results.
type WebSearchResponse struct {
	Query   string          `json:"query"`
	Backend string          `json:"backend,omitempty"`
	Answer  string          `json:"answer,omitempty"`
	Results []WebSearchItem `json:"results"`
}
type WebSearchItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// ViewResponse reports whether an image was rendered by the caller.
type ViewResponse struct {
	Path      string `json:"path"`
	Displayed bool   `json:"displayed"`
}

// ModelListRequest filters provider models.
type ModelListRequest struct {
	Provider string `json:"provider,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
}
type ModelListResponse struct {
	Models []ModelChoice `json:"models"`
}
type ModelChoice struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
}

type MCPCallRequest struct {
	Server string         `json:"server"`
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
}
type MCPResultRequest struct {
	ID     string `json:"id"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}
type MCPServerRequest struct {
	Server string          `json:"server,omitempty"`
	Config json.RawMessage `json:"config,omitempty"`
}
type WebSearchRequest struct {
	Query string `json:"query"`
	Count int    `json:"count,omitempty"`
}
type ViewRequest struct {
	Path string `json:"path"`
}
type GoalRequest struct {
	Summary string `json:"summary,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
type ModelsRequest struct {
	Provider string `json:"provider,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
}
type AskResourceRequest struct {
	Form json.RawMessage `json:"form"`
}
