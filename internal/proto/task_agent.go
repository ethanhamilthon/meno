package proto

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
