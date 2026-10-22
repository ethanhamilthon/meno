package client

import (
	"encoding/json"
	"meno/internal/api"
)

func (m *Mirror) applyChange(e api.Event) ProjectionFields {
	s := &m.State
	if e.Type == api.EventTasksChanged {
		var d api.TasksChangedData
		if json.Unmarshal(e.Data, &d) != nil || d.Tasks == nil {
			s.NeedsResync = true
			return 0
		}
		s.Tasks = cloneTasks(d.Tasks)
		return TasksChanged
	}
	var d api.ChangedData
	if json.Unmarshal(e.Data, &d) != nil {
		s.NeedsResync = true
		return 0
	}
	for _, f := range d.Fields {
		if f == "history" {
			s.NeedsResync = true
			return 0
		}
	}
	if d.Metadata == nil {
		s.NeedsResync = true
		return 0
	}
	v := d.Metadata
	s.Session = v.Session
	s.Cwd = v.Session.Cwd
	s.Model = v.Session.Model
	s.Provider = v.Provider
	s.Effort = v.Effort
	return MetadataChanged
}
