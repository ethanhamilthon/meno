package proto

// SessionMetadata replaces the complete small session metadata projection.
// Empty strings and zero counters are values, not omitted updates. A history
// change still requires a snapshot, even when metadata accompanies it.
type SessionMetadata struct {
	Session  SessionItem `json:"session"`
	Provider string      `json:"provider"`
	Effort   string      `json:"effort"`
}

// TasksChangedData replaces the complete session task projection.
type TasksChangedData struct {
	Tasks []Task `json:"tasks"`
}
